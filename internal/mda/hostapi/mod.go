// Package hostapi is the client for the LangSmith APIs `langsmith deploy`
// talks to, ported from the mda CLI's `langsmith` module and `github.rs`.
//
// LangSmith exposes a few HTTP surfaces, all authenticated with the same
// credential (and optional workspace/tenant scope):
//
//   - the host backend (`api.host.langchain.com`), which manages LangGraph
//     Deployments: listing/creating deployments, requesting source-upload URLs,
//     triggering builds, and polling revisions (deployments.go);
//   - the platform API (`api.smith.langchain.com`), used here to read a
//     workspace's configured secret names (secrets.go), to manage sandboxes,
//     snapshots and registries (sandboxes.go, snapshots.go, registries.go), to
//     name the organization and workspace a deploy writes into
//     (workspaces.go), and for Agent Auth, Agents and Trigger Server
//     (agent_auth.go, agents.go, github_app_setup.go, github_channel.go,
//     linear_app.go, trigger_server.go);
//   - Context Hub, used during deploy to sync durable agent context
//     ([ContextHubClient]); and
//   - a deployed Agent Server, for cron reconciliation (crons.go).
//
// [Client] carries the host backend and platform API base URLs; those endpoint
// groups live in the other files as methods on it. Context Hub has its own
// client because it uses a separate base URL. Where a Rust function took no
// context, its Go form takes a [context.Context] first.
//
// # Authentication
//
// Every request goes through the LangSmith CLI's [client.Client]: typed
// langsmith-go calls through its SDK, and the endpoints the SDK lacks through
// [client.Client.DoAbsolute], which sends an absolute URL through the same
// SDK. Either way the SDK attaches the credential the CLI resolved
// (`--api-key`, `LANGSMITH_API_KEY`, or the selected profile's API key or OAuth
// token, refreshed under the lock shared with other CLI processes) and the
// workspace (`X-Tenant-Id`) from the client options. hostapi never builds a
// credential header itself; the only header it adds is not a credential:
// `x-auth-scheme: langsmith` for Agent Server calls. This holds for every host: the host backend, the
// platform API, Context Hub, a deployed Agent Server, Trigger Server (on the
// platform API) and a sandbox dataplane. The credential only travels over
// `https` (or plain `http` to this machine), and never follows a redirect to
// another origin. The signed GCS upload URL the host
// backend hands out carries its own authorization and must be sent without
// any LangSmith credential, so the source upload stays outside this package.
//
// Where the Rust handed its API key to other code, the Go port works from the
// request that was sent instead: [Client.SetupTriggerChannel] redacts
// whatever credential its own request carried from Trigger Server's answers.
// No credential header ever leaves this package's requests. The PAT
// cache, device login and `Login` of `login.rs` are dropped; only the URL
// helpers deploy reaches are kept (platform_api.go, browser.go).
//
// # Typed SDK and raw requests
//
// Endpoints langsmith-go covers (Workspaces, Agents, Repos, Repos.Directories,
// Sandboxes.Boxes/Snapshots/Registries) are sent with its service methods, so
// the path and the query and body encoding are the SDK's, while the response
// body is read here so decoding stays the Rust's: strict where the Rust was
// strict, lenient where it was lenient, and every unsuccessful status is an
// [APIError]. Those calls go to the endpoints' SDK paths under `/api`
// (`/api/v2/sandboxes`, `/api/v1/platform/hub`), which SaaS serves alongside
// the bare paths the Rust used and which self-hosted installations require.
// The rest (host backend deployments, Agent Auth, Trigger Server, Context Hub
// taggings, workspace secrets and info, Agent Server crons, sandbox batch
// delete and dataplane execute) are raw requests to the Rust's URLs.
//
// # Names
//
// Enum variants are constants prefixed with their type
// (`DeploymentType::Dev` is [DeploymentTypeDev]), since several share a name.
// Enums carrying data are a struct with a Kind. `as_str` is String. An
// `Option<String>` field or argument is a string, empty for None, unless null
// and empty differ on the wire.
package hostapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/langchain-ai/langsmith-go/option"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// APIKeyEnv is the accepted env var for the LangSmith / host-backend API key.
const APIKeyEnv = "LANGSMITH_API_KEY"

// BuildArtifactsTokenEnv is the runner-only credential for reporting artifacts
// for one GitHub build revision.
const BuildArtifactsTokenEnv = "MDA_BUILD_ARTIFACTS_TOKEN"

// RemovedAPIKeyEnvs are removed API key env names that previously aliased
// [APIKeyEnv].
var RemovedAPIKeyEnvs = []string{"LANGCHAIN_API_KEY"}

// GatewayAPIKeyEnv is the legacy env name for authenticating a constructed
// client to LangSmith Gateway.
//
// Older `mda init --gateway` scaffolds read this instead of [APIKeyEnv], which
// is reserved and never reaches a deployment. Deploy still fills a blank alias
// from the resolved LangSmith key so those projects keep working; new projects
// should use `langsmith:` model strings or a provider key directly.
const GatewayAPIKeyEnv = "LANGSMITH_GATEWAY_API_KEY"

// ResolvePreferredEnv resolves a preferred env var from CLI override, process
// env, then project `.env`.
//
// When the preferred name is unset but a removed alias is present, returns an
// error of the form `{alias} is no longer supported; set {preferred}{hint}`.
// cliOverride is ignored when blank.
func ResolvePreferredEnv(preferred string, removed []string, cliOverride string, envFile map[string]string, processEnv func(string) (string, bool), replacementHint string) (string, bool, error) {
	if value := strings.TrimSpace(cliOverride); value != "" {
		return value, true, nil
	}
	if value, ok := LookupEnv(preferred, envFile, processEnv); ok {
		return value, true, nil
	}
	for _, key := range removed {
		if _, ok := LookupEnv(key, envFile, processEnv); ok {
			return "", false, fmt.Errorf("%s is no longer supported; set %s%s", key, preferred, replacementHint)
		}
	}
	return "", false, nil
}

// EnvOrigin is where [LookupEnvWithOrigin] found a value.
type EnvOrigin int

// The origins.
const (
	EnvOriginShell EnvOrigin = iota
	EnvOriginEnvFile
)

// LookupEnv looks up a non-empty trimmed env value from process env, then
// project `.env`.
func LookupEnv(key string, envFile map[string]string, processEnv func(string) (string, bool)) (string, bool) {
	value, _, ok := LookupEnvWithOrigin(key, envFile, processEnv)
	return value, ok
}

// LookupEnvWithOrigin is like [LookupEnv], also reporting which source
// supplied the value.
//
// The shell wins so a one-off `KEY=value mda …` or a CI-injected variable is
// not silently shadowed by a project file. When both are set and disagree,
// warn once per key so a stale shell export is not silently used either.
func LookupEnvWithOrigin(key string, envFile map[string]string, processEnv func(string) (string, bool)) (string, EnvOrigin, bool) {
	if raw, ok := processEnv(key); ok {
		if fromShell, ok := nonEmptyTrimmed(raw); ok {
			WarnIfShellShadowsEnvFile(key, fromShell, envFile)
			return fromShell, EnvOriginShell, true
		}
	}
	if raw, ok := envFile[key]; ok {
		if value, ok := nonEmptyTrimmed(raw); ok {
			return value, EnvOriginEnvFile, true
		}
	}
	return "", 0, false
}

// WarnIfShellShadowsEnvFile warns once per key when a shell value shadows a
// different, non-empty value in the project `.env`. For callers that already
// chose the shell value without going through [LookupEnvWithOrigin]. Returns
// whether they differ; a trailing `/` alone does not count.
func WarnIfShellShadowsEnvFile(key, fromShell string, envFile map[string]string) bool {
	fromFile, ok := nonEmptyTrimmed(envFile[key])
	differs := ok && strings.TrimRight(fromFile, "/") != strings.TrimRight(strings.TrimSpace(fromShell), "/")
	if differs {
		warnShellShadowsEnvFile(key)
	}
	return differs
}

func nonEmptyTrimmed(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	return trimmed, trimmed != ""
}

var (
	shadowWarningsMu sync.Mutex
	shadowWarnings   = map[string]bool{}
)

// warnShellShadowsEnvFile names the key only: values are often credentials.
func warnShellShadowsEnvFile(key string) {
	shadowWarningsMu.Lock()
	first := !shadowWarnings[key]
	shadowWarnings[key] = true
	shadowWarningsMu.Unlock()
	if first {
		_ = ui.LogWarning(fmt.Sprintf("%s is set in both your shell and .env with different values; using the shell value. Unset it in your shell to use the .env value.", key))
	}
}

// LookupEnvOrProcess is [LookupEnv] with the real process environment as the
// fallback.
func LookupEnvOrProcess(key string, envFile map[string]string) (string, bool) {
	return LookupEnv(key, envFile, os.LookupEnv)
}

// RequestTimeout is the per-request timeout. Source uploads use their own
// client/timeout.
const RequestTimeout = 30 * time.Second

// CreateDeploymentTimeout is the timeout of first-time deployment creation,
// which can provision backend resources and routinely exceeds
// [RequestTimeout].
const CreateDeploymentTimeout = 120 * time.Second

// requestOptions shape one request. The zero value is the client's default.
type requestOptions struct {
	// timeout replaces the client's timeout when set.
	timeout time.Duration
	// studioAuth declares the LangSmith studio auth scheme (see
	// [Client.requestAgentServer]).
	studioAuth bool
	// noRedirects answers a redirect with the redirect itself.
	noRedirects bool
	// observe sees the request as sent, credentials included.
	observe func(*http.Request)
}

// Secret is a deployment secret (an environment variable injected into the
// deployment).
type Secret struct {
	// Name is the environment variable name.
	Name string `json:"name"`
	// Value is the environment variable value.
	Value string `json:"value"`
}

// DeploymentType is the deployment type accepted by `langsmith deploy
// --deployment-type` and forwarded to the LangSmith host backend.
type DeploymentType int

// The deployment types; the zero value is [DeploymentTypeDev].
const (
	DeploymentTypeDev DeploymentType = iota
	DeploymentTypeProd
)

// String returns the wire value sent to the LangSmith host backend.
func (t DeploymentType) String() string {
	if t == DeploymentTypeProd {
		return "prod"
	}
	return "dev"
}

// AgentEnvironment is the environment of an agent deployment.
type AgentEnvironment int

// The environments.
const (
	AgentEnvironmentDevelopment AgentEnvironment = iota
	AgentEnvironmentStaging
	AgentEnvironmentProduction
)

// String returns the environment's wire name.
func (e AgentEnvironment) String() string {
	switch e {
	case AgentEnvironmentStaging:
		return "staging"
	case AgentEnvironmentProduction:
		return "production"
	default:
		return "development"
	}
}

// MarshalText serializes the environment as its lowercase name.
func (e AgentEnvironment) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

// DeploymentAgent names an agent deployment by agent and environment.
type DeploymentAgent struct {
	AgentID     string           `json:"agent_id"`
	Environment AgentEnvironment `json:"environment"`
}

// ResourceName is the prefix for the deploy-owned resources (sandboxes,
// Context Hub repos, recipes). The host names the deployment itself, so look
// that up with [Client.ListAgentDeployments] instead.
//
// Production keeps the bare agent ID: a name-only deploy lands in the
// production environment too, so switching it to agent mode keeps its
// memories and sandboxes. Other environments get a suffix to stay apart.
func (a DeploymentAgent) ResourceName() (string, error) {
	if a.Environment == AgentEnvironmentProduction {
		return a.AgentID, nil
	}
	return project.NormalizeName(a.AgentID + "-" + a.Environment.String())
}

// Client is the JSON client for the LangSmith APIs.
//
// Sends every request through the LangSmith CLI client, which attaches the
// credential and workspace, and routes calls to either the host backend or the
// platform API depending on the endpoint group. Copies are independent.
type Client struct {
	ls *client.Client
	// timeout is the default per-request timeout.
	timeout time.Duration
	// hostBackendURL is the host backend base URL
	// (`https://api.host.langchain.com`), trailing slash trimmed.
	hostBackendURL string
	// platformAPIURL is the LangSmith platform API base URL
	// (`https://api.smith.langchain.com`), trailing slash trimmed.
	platformAPIURL string
}

// PlatformBase is the LangSmith platform API root used by Fleet and sandbox
// registry APIs.
func (c *Client) PlatformBase() string {
	return c.platformAPIURL
}

// IsSelfHosted reports whether this client reaches Deployments through a
// self-hosted frontend.
func (c *Client) IsSelfHosted() bool {
	return IsSelfHostedHostBackendURL(c.hostBackendURL)
}

// PublicPlatformBase is the public platform API base for this LangSmith
// installation.
//
// Self-hosted frontends reserve `/api` for backend traffic. SaaS and the
// direct local-development stack expose platform services at their API origin
// instead.
func (c *Client) PublicPlatformBase() (string, error) {
	platform := c.PlatformBase()
	if !c.IsSelfHosted() {
		return platform, nil
	}
	base, ok := SelfHostedPublicAPIBaseURL(platform)
	if !ok {
		return "", errors.New("LangSmith platform API URL is invalid")
	}
	return base, nil
}

// New creates a client for hostBackendURL and platformAPIURL that sends every
// request through ls, which carries the credential and workspace.
//
// Local LangSmith splits these: host-backend (`:8300`, `/v2/deployments`) vs
// nginx (`:1980`, `/v1/fleet/*`). A blank platformAPIURL is derived from the
// host (`api.host.*` → `api.smith.*`), else shares its base.
func New(ls *client.Client, hostBackendURL, platformAPIURL string) (*Client, error) {
	if hostBackendURL == "" {
		return nil, errors.New("Host backend URL is required")
	}
	hostBackendURL = strings.TrimRight(hostBackendURL, "/")
	platform := strings.TrimRight(strings.TrimSpace(platformAPIURL), "/")
	if platform == "" {
		derived, ok := LangsmithAPIBaseURL(hostBackendURL)
		if ok {
			platform = derived
		} else {
			// Non-SaaS single-origin installs: Fleet often shares the host base.
			platform = hostBackendURL
		}
	}
	return &Client{
		ls:             ls,
		timeout:        RequestTimeout,
		hostBackendURL: hostBackendURL,
		platformAPIURL: platform,
	}, nil
}

// WorkspaceID is the workspace the client is scoped to, or "" when the
// credential's default workspace applies.
func (c *Client) WorkspaceID() string {
	return c.ls.WorkspaceID()
}

// queryParam is one query parameter; requests keep their order.
type queryParam struct {
	key, value string
}

// request performs a request against base and returns the JSON response, nil
// when empty.
func (c *Client) request(ctx context.Context, base, method, path string, params []queryParam, payload any) (json.RawMessage, error) {
	return c.requestInner(ctx, base, method, path, params, payload, requestOptions{})
}

// requestAgentServer is like [Client.request], but declares the LangSmith
// studio auth scheme (`x-auth-scheme: langsmith`).
//
// Requests to a deployed Agent Server pass through the app's auth stack. A
// managed-identity deployment installs a custom `authenticate` handler that
// requires an end-user bearer token, which the CLI (authenticating as the
// workspace) does not have. LangGraph only routes a request to its LangSmith
// studio auth backend, which validates the LangSmith credential, when this
// header is present; otherwise the custom handler runs and rejects the call
// with a 401. Use this for control-plane calls against the Agent Server (e.g.
// cron reconciliation).
func (c *Client) requestAgentServer(ctx context.Context, base, method, path string, params []queryParam, payload any) (json.RawMessage, error) {
	return c.requestInner(ctx, base, method, path, params, payload, requestOptions{studioAuth: true})
}

// requestOptional is like [Client.request], but treats HTTP 404 as absent.
//
// Teardown paths are idempotent: a resource that is already gone is a success,
// not an error.
func (c *Client) requestOptional(ctx context.Context, base, method, path string, params []queryParam, payload any) (json.RawMessage, bool, error) {
	status, text, err := c.send(ctx, base+path, method, params, payload, requestOptions{})
	if err != nil {
		return nil, false, requestFailed(method, path, err)
	}
	return optionalResult(status, text, path)
}

func (c *Client) requestWithTimeout(ctx context.Context, base, method, path string, params []queryParam, payload any, timeout time.Duration) (json.RawMessage, error) {
	return c.requestInner(ctx, base, method, path, params, payload, requestOptions{timeout: timeout})
}

func (c *Client) requestInner(ctx context.Context, base, method, path string, params []queryParam, payload any, options requestOptions) (json.RawMessage, error) {
	status, text, err := c.send(ctx, base+path, method, params, payload, options)
	if err != nil {
		return nil, requestFailed(method, path, err)
	}
	return successResult(status, text, path)
}

// send is [sendJSONRequest] with the client's default timeout.
func (c *Client) send(ctx context.Context, rawURL, method string, params []queryParam, payload any, options requestOptions) (int, []byte, error) {
	if options.timeout == 0 {
		options.timeout = c.timeout
	}
	return sendJSONRequest(ctx, c.ls, method, rawURL, params, payload, options)
}

// sdk is [sendSDKRequest] against the platform API with the client's default
// timeout.
func (c *Client) sdk(ctx context.Context, options requestOptions, call sdkCall) (int, []byte, error) {
	if options.timeout == 0 {
		options.timeout = c.timeout
	}
	return sendSDKRequest(ctx, c.platformAPIURL, options, call)
}

// sdkRequest is [Client.request] for a typed SDK call against the platform
// API; path names it in errors.
func (c *Client) sdkRequest(ctx context.Context, method, path string, options requestOptions, call sdkCall) (json.RawMessage, error) {
	status, text, err := c.sdk(ctx, options, call)
	if err != nil {
		return nil, requestFailed(method, path, err)
	}
	return successResult(status, text, path)
}

// sdkRequestOptional is [Client.requestOptional] for a typed SDK call against
// the platform API.
func (c *Client) sdkRequestOptional(ctx context.Context, method, path string, call sdkCall) (json.RawMessage, bool, error) {
	status, text, err := c.sdk(ctx, requestOptions{}, call)
	if err != nil {
		return nil, false, requestFailed(method, path, err)
	}
	return optionalResult(status, text, path)
}

// requestError is a request that got no response: it never reached the
// server, or the response could not be read.
type requestError struct {
	method, path string
	err          error
}

func (e *requestError) Error() string {
	return fmt.Sprintf("%s %s request failed: %v", e.method, e.path, e.err)
}

func (e *requestError) Unwrap() error { return e.err }

func requestFailed(method, path string, err error) error {
	return &requestError{method: method, path: path, err: err}
}

func isSuccess(status int) bool {
	return status >= 200 && status < 300
}

func successResult(status int, text []byte, path string) (json.RawMessage, error) {
	if !isSuccess(status) {
		return nil, apiErrorFromResponse(status, text)
	}
	return decodeJSONBody(text, path)
}

func optionalResult(status int, text []byte, path string) (json.RawMessage, bool, error) {
	if status == http.StatusNotFound {
		return nil, false, nil
	}
	value, err := successResult(status, text, path)
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

// noRedirectHTTPClient answers a redirect with the redirect itself.
var noRedirectHTTPClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// sendJSONRequest is the authenticated JSON request shared by [Client] and
// [ContextHubClient], sent through ls so the SDK attaches the credential and
// workspace. It returns the status and body of any response.
func sendJSONRequest(ctx context.Context, ls *client.Client, method, rawURL string, params []queryParam, payload any, options requestOptions) (int, []byte, error) {
	if len(params) > 0 {
		separator := "?"
		if strings.Contains(rawURL, "?") {
			separator = "&"
		}
		rawURL += separator + encodeQuery(params)
	}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, fmt.Errorf("encoding the request body: %w", err)
		}
		body = bytes.NewReader(data)
	}
	header := http.Header{}
	header.Set("Accept", "application/json")
	if options.studioAuth {
		header.Set("x-auth-scheme", "langsmith")
	}
	// Bodyless POST/PATCH requests still need a Content-Length: some gateways
	// (e.g. Google frontends) reject them with HTTP 411 otherwise. Go sends
	// `Content-Length: 0` for them.
	response, err := ls.DoAbsolute(ctx, method, rawURL, body, header, transportOptions(options)...)
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, response.Body, nil
}

// sdkCall makes one typed SDK call with opts appended to its own.
type sdkCall func(ctx context.Context, opts ...option.RequestOption) error

// sendSDKRequest makes a typed SDK call against base and returns the status
// and body of its response, as [sendJSONRequest] does: one attempt, the
// unsuccessful statuses as a response rather than an error.
func sendSDKRequest(ctx context.Context, base string, options requestOptions, call sdkCall) (int, []byte, error) {
	if err := client.CheckCredentialTransport(base); err != nil {
		return 0, nil, err
	}
	var response *http.Response
	var body []byte
	opts := append(transportOptions(options),
		option.WithBaseURL(sdkRoot(base)),
		option.WithMaxRetries(0),
		option.WithResponseInto(&response),
		option.WithResponseBodyInto(&body),
	)
	err := call(ctx, opts...)
	if response == nil {
		return 0, nil, err
	}
	if response.StatusCode >= 400 {
		// The SDK puts the body of an unsuccessful response back for reading.
		defer response.Body.Close()
		text, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return 0, nil, fmt.Errorf("failed to read the response body: %w", readErr)
		}
		return response.StatusCode, text, nil
	}
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, body, nil
}

// transportOptions are the request options both senders share.
func transportOptions(options requestOptions) []option.RequestOption {
	var opts []option.RequestOption
	if options.timeout > 0 {
		opts = append(opts, option.WithRequestTimeout(options.timeout))
	}
	if options.noRedirects {
		opts = append(opts, option.WithHTTPClient(noRedirectHTTPClient))
	}
	if options.observe != nil {
		observe := options.observe
		opts = append(opts, option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			observe(req)
			return next(req)
		}))
	}
	return opts
}

// sdkRoot is base as the SDK's base URL: the deployment root its `api/…`
// paths are joined to.
func sdkRoot(base string) string {
	root := strings.TrimRight(base, "/")
	for _, suffix := range []string{"/api/v1", "/api"} {
		if trimmed, ok := strings.CutSuffix(root, suffix); ok {
			return trimmed
		}
	}
	return root
}

// encodeQuery form-encodes params in order.
func encodeQuery(params []queryParam) string {
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(parts, url.QueryEscape(param.key)+"="+url.QueryEscape(param.value))
	}
	return strings.Join(parts, "&")
}

// decodeJSONBody validates a JSON response body, nil when it is empty or
// null.
func decodeJSONBody(text []byte, context string) (json.RawMessage, error) {
	if len(text) == 0 {
		return nil, nil
	}
	if !json.Valid(text) {
		var value any
		err := json.Unmarshal(text, &value)
		return nil, fmt.Errorf("failed to decode response from %s: %w", context, err)
	}
	if string(bytes.TrimSpace(text)) == "null" {
		return nil, nil
	}
	return json.RawMessage(text), nil
}

// decodeObject decodes a JSON object response; nil when the body was empty or
// null.
func decodeObject(raw json.RawMessage, context string) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("failed to decode response from %s: %w", context, err)
	}
	return object, nil
}

// APIError is an unsuccessful HTTP response from a LangSmith API.
//
// Carries the status next to the humanized Detail so callers can react to a
// specific server condition (`langsmith deploy` turns the host backend's
// entitlement 403s into a plan upsell) instead of pattern-matching a message
// that has already been flattened to a string.
//
// Error keeps the `{detail} (HTTP {code})` shape the CLI has always printed.
type APIError struct {
	// Status is the status the API returned.
	Status int
	// Detail is the server's explanation, humanized by humanizeErrorBody,
	// falling back to the raw body and then to a stand-in when the response
	// carried nothing to explain itself with.
	Detail string
}

func apiErrorFromResponse(status int, text []byte) *APIError {
	detail, ok := humanizeErrorBody(text)
	if !ok {
		detail = string(text)
	}
	if detail == "" {
		// Error always prints the status, so this must not repeat it.
		detail = "the server sent no detail"
	}
	return &APIError{Status: status, Detail: detail}
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s (HTTP %d)", e.Detail, e.Status)
}

// HTTPStatus is the status, for errs.CategoryOf.
func (e *APIError) HTTPStatus() int {
	return e.Status
}

// HostURLParts are the parsed pieces of a host-backend style URL (scheme +
// hostname, no port/path).
type HostURLParts struct {
	Scheme string
	// Hostname is the hostname without port or path (e.g.
	// `eu.api.host.langchain.com`).
	Hostname string
	// Trimmed is the original URL with a trailing slash trimmed.
	Trimmed string
}

// ParseHostURL splits a host-backend URL into scheme + hostname for
// sibling-service mapping.
func ParseHostURL(hostURL string) HostURLParts {
	trimmed := strings.TrimRight(hostURL, "/")
	scheme, afterScheme := "https", trimmed
	if before, after, ok := strings.Cut(trimmed, "://"); ok && before != "" {
		scheme, afterScheme = before, after
	}
	hostPort, _, _ := strings.Cut(afterScheme, "/")
	hostname, _, _ := strings.Cut(hostPort, ":")
	return HostURLParts{Scheme: scheme, Hostname: hostname, Trimmed: trimmed}
}

// LangsmithAPIBaseURL derives the LangSmith REST API base URL
// (`https://api.smith.langchain.com`) from the host backend API URL,
// preserving any regional prefix.
//
// The host backend (`api.host.langchain.com`) and the LangSmith API
// (`api.smith.langchain.com`) are sibling services that share a regional
// prefix (`eu.api.host.langchain.com` ↔ `eu.api.smith.langchain.com`), so the
// mapping is a straight `host.` → `smith.` substitution on the hostname.
// Returns false for hosts that are not `*.host.langchain.com` (localhost,
// custom/self-hosted backends), where the platform API location is unknown.
func LangsmithAPIBaseURL(hostURL string) (string, bool) {
	parts := ParseHostURL(hostURL)
	const apiHostSuffix = "host.langchain.com"
	if !strings.HasSuffix(parts.Hostname, apiHostSuffix) {
		return "", false
	}
	smith := strings.Replace(parts.Hostname, "host.langchain.com", "smith.langchain.com", 1)
	return parts.Scheme + "://" + smith, true
}

// LangsmithHostBackendURL is the inverse of [LangsmithAPIBaseURL]: platform
// API → host backend.
//
// Only maps `api.smith.langchain.com` and `{region}.api.smith.langchain.com`
// (not bare dashboard hosts like `smith.langchain.com`).
func LangsmithHostBackendURL(platformURL string) (string, bool) {
	parts := ParseHostURL(platformURL)
	isAPISmith := parts.Hostname == "api.smith.langchain.com" ||
		strings.HasSuffix(parts.Hostname, ".api.smith.langchain.com")
	if !isAPISmith {
		return "", false
	}
	host := strings.Replace(parts.Hostname, "smith.langchain.com", "host.langchain.com", 1)
	return parts.Scheme + "://" + host, true
}

// IsSelfHostedHostBackendURL reports whether hostBackendURL is the Deployment
// API mounted by a self-hosted frontend.
//
// API origins and hostnames are configurable for self-hosted installations, so
// the resolved `/api-host` mount is the authoritative signal.
func IsSelfHostedHostBackendURL(hostBackendURL string) bool {
	u, err := url.Parse(hostBackendURL)
	if err != nil || u.Scheme == "" {
		return false
	}
	return lastPathSegment(strings.TrimRight(u.Path, "/")) == "api-host"
}

// SelfHostedHostBackendURL derives the Deployment API base mounted by a
// self-hosted frontend.
func SelfHostedHostBackendURL(platformURL string) (string, bool) {
	return selfHostedURLWithSuffix(platformURL, "api-host")
}

// SelfHostedPublicAPIBaseURL derives the public backend API base mounted by a
// self-hosted frontend.
func SelfHostedPublicAPIBaseURL(platformURL string) (string, bool) {
	return selfHostedURLWithSuffix(platformURL, "api")
}

func selfHostedURLWithSuffix(platformURL, suffix string) (string, bool) {
	u, err := url.Parse(platformURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	path := strings.TrimRight(u.Path, "/")
	path = strings.TrimSuffix(path, "/api/v1")
	if lastPathSegment(path) != suffix {
		u.Path = path + "/" + suffix
		u.RawPath = ""
	}
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	return strings.TrimRight(u.String(), "/"), true
}

func lastPathSegment(path string) string {
	return path[strings.LastIndex(path, "/")+1:]
}

// humanizeErrorBody turns a host-backend error response body into a concise,
// human-readable message, avoiding raw JSON in user-facing errors.
//
// Handles FastAPI-style payloads where `detail` is a string, an array of
// `{loc, msg}` validation errors, or an `{error, message}` envelope. Returns
// false when the body has no recognizable detail (the caller falls back to
// the raw text or status).
func humanizeErrorBody(text []byte) (string, bool) {
	var value map[string]any
	if err := json.Unmarshal(text, &value); err != nil {
		return "", false
	}
	detail, ok := value["detail"]
	if !ok {
		return "", false
	}
	switch detail := detail.(type) {
	case string:
		return detail, true
	case []any:
		var messages []string
		for _, item := range detail {
			if message, ok := formatValidationItem(item); ok {
				messages = append(messages, message)
			}
		}
		if len(messages) == 0 {
			return "", false
		}
		return strings.Join(messages, "; "), true
	case map[string]any:
		if message, ok := detail["message"].(string); ok {
			return message, true
		}
		return jsonText(detail), true
	default:
		return jsonText(detail), true
	}
}

// jsonText is value as compact JSON, the way serde_json prints a Value.
func jsonText(value any) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Sprint(value)
	}
	return strings.TrimSuffix(buffer.String(), "\n")
}

// formatValidationItem formats a single FastAPI validation error into a
// concise, human-readable message that surfaces the offending property path.
//
// Well-known error `type`s are rewritten into friendlier phrasing (e.g.
// `missing` → `Missing required field 'secrets'`); anything else falls back
// to the server-provided `msg`, prefixed with the property path when
// available.
func formatValidationItem(item any) (string, bool) {
	object, _ := item.(map[string]any)
	field, hasField := fieldPath(object)
	errorType, _ := object["type"].(string)

	switch errorType {
	case "missing", "value_error.missing":
		if hasField {
			return fmt.Sprintf("Missing required field '%s'", field), true
		}
		return "Missing a required field", true
	default:
		message, ok := object["msg"].(string)
		if !ok {
			return "", false
		}
		if hasField {
			return fmt.Sprintf("Invalid value for '%s': %s", field, message), true
		}
		return message, true
	}
}

// fieldPath extracts the dotted property path from a validation error's
// `loc`, dropping the leading `body`/`query` location prefix. Returns false
// when empty.
func fieldPath(item map[string]any) (string, bool) {
	loc, ok := item["loc"].([]any)
	if !ok {
		return "", false
	}
	var path strings.Builder
	skippingPrefix := true
	for _, part := range loc {
		var segment string
		switch part := part.(type) {
		case string:
			segment = part
		case float64:
			segment = jsonText(part)
		default:
			continue
		}
		if skippingPrefix && (segment == "body" || segment == "query") {
			continue
		}
		skippingPrefix = false
		if path.Len() > 0 {
			path.WriteByte('.')
		}
		path.WriteString(segment)
	}
	return path.String(), path.Len() > 0
}

// RequireStr extracts a required string field from a JSON object, erroring
// helpfully.
func RequireStr(value any, key, context string) (string, error) {
	object, _ := value.(map[string]any)
	text, ok := object[key].(string)
	if !ok {
		return "", fmt.Errorf("%s: response missing string field '%s'", context, key)
	}
	return text, nil
}

// requireFields fails the way serde does for a struct field without a
// default: when a field named in names is absent or null in the JSON object
// data.
func requireFields(data []byte, names ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	for _, name := range names {
		if raw, ok := object[name]; !ok || string(bytes.TrimSpace(raw)) == "null" {
			return fmt.Errorf("missing field `%s`", name)
		}
	}
	return nil
}
