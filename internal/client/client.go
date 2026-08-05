package client

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
	"strconv"
	"strings"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
)

// Client wraps the LangSmith Go SDK and provides helpers for raw HTTP calls.
type Client struct {
	SDK              *langsmith.Client
	apiKey           string
	oauthAccessToken string
	apiURL           string
	workspaceID      string

	// Cached session name → ID mappings (per invocation).
	sessionCache map[string]string

	// Cached v2-API decision (see UseV2API), resolved once per invocation
	// from GET /info.
	cachedUseV2API *bool
}

// Options controls LangSmith client authentication and routing.
type Options struct {
	APIKey           string
	OAuthAccessToken string
	APIURL           string
	WorkspaceID      string
	ProfileName      string
}

// NormalizeURL strips a trailing "/api/v1" suffix (with or without a trailing
// slash) so that the SDK — which appends "api/v1" itself — does not double it.
// Self-hosted users commonly set LANGSMITH_ENDPOINT to "https://host/api/v1".
func NormalizeURL(apiURL string) string {
	u := strings.TrimRight(apiURL, "/")
	return strings.TrimSuffix(u, "/api/v1")
}

// New creates a new Client.
func New(apiKey, apiURL string) *Client {
	return NewWithOptions(Options{
		APIKey:      apiKey,
		APIURL:      apiURL,
		WorkspaceID: os.Getenv("LANGSMITH_WORKSPACE_ID"),
	})
}

// NewWithOptions creates a new Client from resolved options.
func NewWithOptions(options Options) *Client {
	normalized := NormalizeURL(options.APIURL)

	var opts []option.RequestOption
	// Auth precedence mirrors resolveClientOptions. A resolved profile is routed
	// through WithProfile so an explicit selection replaces the config's
	// current_profile (clearing any inherited tenant/base URL); WithProfile
	// supplies the profile's own auth (API key or OAuth). A profile-less explicit
	// key or bearer is applied directly.
	switch {
	case options.ProfileName != "":
		opts = append(opts, langsmith.WithProfile(options.ProfileName))
	case options.APIKey != "":
		opts = append(opts, option.WithAPIKey(options.APIKey))
	case options.OAuthAccessToken != "":
		opts = append(opts, option.WithHeader("authorization", "Bearer "+options.OAuthAccessToken))
	}
	// Only set base URL if not the default (the SDK reads LANGSMITH_ENDPOINT too).
	if normalized != "" {
		opts = append(opts, option.WithBaseURL(normalized))
	}
	if options.WorkspaceID != "" {
		opts = append(opts, option.WithTenantID(options.WorkspaceID))
	}

	return &Client{
		SDK:              langsmith.NewClient(opts...),
		apiKey:           options.APIKey,
		oauthAccessToken: options.OAuthAccessToken,
		apiURL:           normalized,
		workspaceID:      options.WorkspaceID,
		sessionCache:     make(map[string]string),
	}
}

// ResolveSessionID resolves a project name to its session UUID, with caching.
func (c *Client) ResolveSessionID(ctx context.Context, projectName string) (string, error) {
	if id, ok := c.sessionCache[projectName]; ok {
		return id, nil
	}
	resp, err := c.SDK.Sessions.List(ctx, langsmith.SessionListParams{
		Name:  langsmith.F(projectName),
		Limit: langsmith.F(int64(1)),
	})
	if err != nil {
		return "", fmt.Errorf("resolving project %q: %w", projectName, err)
	}
	if len(resp.Items) == 0 {
		return "", fmt.Errorf("project not found: %s", projectName)
	}
	id := resp.Items[0].ID
	c.sessionCache[projectName] = id
	return id, nil
}

// Self-hosted gains the v2 (SmithDB) run-query API at 0.16.
const minSelfHostedV2Minor = 16

// UseV2API reports whether the connected deployment's v2 (SmithDB) API should
// be used, resolved once from GET /info and cached.
func (c *Client) UseV2API(ctx context.Context) (bool, error) {
	if c.cachedUseV2API != nil {
		return *c.cachedUseV2API, nil
	}
	info, err := c.SDK.Info.List(ctx)
	if err != nil {
		return false, fmt.Errorf("fetching deployment info: %w", err)
	}
	v := useV2API(info.Version)
	c.cachedUseV2API = &v
	return v, nil
}

// useV2API decides whether to use the v2 API from the /info version. Cloud reports
// a non-release version (e.g. "dev") → v2; self-hosted reports a semver, v2 at
// >= 0.16 else v1.
func useV2API(version string) bool {
	major, minor, ok := parseReleaseVersion(version)
	if !ok {
		return true // Cloud / non-release version
	}
	if major != 0 {
		return true
	}
	return minor >= minSelfHostedV2Minor
}

// parseReleaseVersion pulls major.minor from a release version like "0.16.18rc1"
// or "v1.2.3"; ok=false for non-release versions (e.g. "dev", "").
func parseReleaseVersion(v string) (major, minor int, ok bool) {
	v = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "v"))
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(leadingDigits(parts[0]))
	if err != nil {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(leadingDigits(parts[1]))
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// leadingDigits returns the leading ASCII digits of s (e.g. "16rc1" → "16").
func leadingDigits(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i]
}

// PlatformPathPrefixEnv overrides the platform prefix.
// Multi-origin self-hosted needs the bare path.
const PlatformPathPrefixEnv = "LANGSMITH_PLATFORM_PATH_PREFIX"

// defaultPlatformPathPrefix suits SaaS and single-origin.
// Single-origin nginx rewrites it to /v1/platform.
const defaultPlatformPathPrefix = "/api/v1/platform"

// PlatformPath builds a platform-service path.
func PlatformPath(elem ...string) string {
	prefix := strings.TrimRight(os.Getenv(PlatformPathPrefixEnv), "/")
	if prefix == "" {
		prefix = defaultPlatformPathPrefix
	}
	for _, e := range elem {
		prefix += "/" + url.PathEscape(e)
	}
	return prefix
}

// CustomAppsPath is the custom-apps collection.
func CustomAppsPath() string { return PlatformPath("custom-apps") }

// CustomAppPath addresses one app.
func CustomAppPath(appID string) string { return PlatformPath("custom-apps", appID) }

// CustomAppRequest is the body of custom-app create (POST) and update (PATCH).
// Unset pointer fields are omitted, leaving the server value untouched.
type CustomAppRequest struct {
	Name          *string           `json:"name,omitempty"`
	Description   *string           `json:"description,omitempty"`
	Files         map[string]string `json:"files,omitempty"`
	Entrypoint    *string           `json:"entrypoint,omitempty"`
	SourceArchive *string           `json:"source_archive,omitempty"`
}

// --- Raw HTTP helpers for endpoints not covered by the SDK ---

// RawGet performs a GET request to the LangSmith API.
func (c *Client) RawGet(ctx context.Context, path string, result any) error {
	return c.rawRequest(ctx, http.MethodGet, path, nil, result)
}

// RawPost performs a POST request to the LangSmith API.
func (c *Client) RawPost(ctx context.Context, path string, body any, result any) error {
	return c.rawRequest(ctx, http.MethodPost, path, body, result)
}

// RawPatch performs a PATCH request to the LangSmith API.
func (c *Client) RawPatch(ctx context.Context, path string, body any, result any) error {
	return c.rawRequest(ctx, http.MethodPatch, path, body, result)
}

// RawDelete performs a DELETE request to the LangSmith API.
func (c *Client) RawDelete(ctx context.Context, path string, result any) error {
	return c.rawRequest(ctx, http.MethodDelete, path, nil, result)
}

// FetchCustomAppSource returns the stored .tar.gz bytes for a custom app.
// The response is binary, so it bypasses the JSON-decoding raw helpers.
func (c *Client) FetchCustomAppSource(ctx context.Context, appID string) ([]byte, error) {
	path := CustomAppPath(appID) + "/source"
	resp, err := c.doHTTP(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.statusCode >= 400 {
		return nil, &httpError{statusCode: resp.statusCode, body: resp.body}
	}
	if len(resp.body) == 0 {
		return nil, fmt.Errorf("custom app source response was empty")
	}
	return resp.body, nil
}

// httpResponse holds the parsed result of a raw HTTP call.
type httpResponse struct {
	statusCode int
	proto      string
	headers    http.Header
	body       []byte
}

type httpError struct {
	statusCode int
	body       []byte
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.statusCode, formatHTTPErrorBody(e.body))
}

func IsNotFound(err error) bool {
	var httpErr *httpError
	return errors.As(err, &httpErr) && httpErr.statusCode == http.StatusNotFound
}

func IsConflict(err error) bool {
	var httpErr *httpError
	return errors.As(err, &httpErr) && httpErr.statusCode == http.StatusConflict
}

func IsForbidden(err error) bool {
	var httpErr *httpError
	return errors.As(err, &httpErr) && httpErr.statusCode == http.StatusForbidden
}

func IsBadRequest(err error) bool {
	var httpErr *httpError
	return errors.As(err, &httpErr) && httpErr.statusCode == http.StatusBadRequest
}

type httpErrorBody struct {
	Error            string `json:"error"`
	Message          string `json:"message"`
	ErrorDescription string `json:"error_description"`
	Detail           any    `json:"detail"`
}

func (c *Client) requestURL(path string) (*url.URL, error) {
	base, err := url.Parse(c.apiURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("invalid API URL %q", c.apiURL)
	}
	ref, err := url.Parse(path)
	if err != nil || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || ref.IsAbs() || ref.Host != "" {
		return nil, fmt.Errorf("API path %q must be root-relative", path)
	}
	base.Path = ref.Path
	base.RawPath = ref.RawPath
	base.RawQuery = ref.RawQuery
	base.Fragment = ""
	return base, nil
}

// doHTTP is the shared low-level helper used by RawDo and rawRequest.
func (c *Client) doHTTP(ctx context.Context, method, path string, body io.Reader, extraHeaders http.Header) (*httpResponse, error) {
	requestURL, err := c.requestURL(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiURL, body)
	if err == nil {
		req.URL.Path = requestURL.Path
		req.URL.RawPath = requestURL.RawPath
		req.URL.RawQuery = requestURL.RawQuery
	}
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
	if c.oauthAccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.oauthAccessToken)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.workspaceID != "" {
		req.Header.Set("x-tenant-id", c.workspaceID)
	}
	for k, vals := range extraHeaders {
		req.Header[k] = vals
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &httpResponse{
		statusCode: resp.StatusCode,
		proto:      resp.Proto,
		headers:    resp.Header,
		body:       respBody,
	}, nil
}

// RawDo performs an arbitrary HTTP request and returns the raw response.
// Unlike RawGet/RawPost/RawDelete, it does not unmarshal the response and
// does not treat 4xx/5xx as errors — callers decide how to handle status codes.
// body may be nil. extraHeaders are merged on top of the default auth headers.
func (c *Client) RawDo(ctx context.Context, method, path string, body io.Reader, extraHeaders http.Header) (statusCode int, proto string, respHeaders http.Header, respBody []byte, err error) {
	resp, err := c.doHTTP(ctx, method, path, body, extraHeaders)
	if err != nil {
		return 0, "", nil, nil, err
	}
	return resp.statusCode, resp.proto, resp.headers, resp.body, nil
}

// APIKey returns the client's API key.
func (c *Client) APIKey() string { return c.apiKey }

// OAuthAccessToken returns the client's OAuth access token.
func (c *Client) OAuthAccessToken() string { return c.oauthAccessToken }

// APIURL returns the client's normalized API URL.
func (c *Client) APIURL() string { return c.apiURL }

func (c *Client) rawRequest(ctx context.Context, method, path string, body any, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	resp, err := c.doHTTP(ctx, method, path, bodyReader, nil)
	if err != nil {
		return err
	}

	if resp.statusCode >= 400 {
		return &httpError{statusCode: resp.statusCode, body: resp.body}
	}

	if result != nil {
		if err := json.Unmarshal(resp.body, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

func formatHTTPErrorBody(body []byte) string {
	raw := strings.TrimSpace(string(body))
	var parsed httpErrorBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return raw
	}

	code := strings.TrimSpace(parsed.Error)
	message := strings.TrimSpace(parsed.Message)
	if message == "" {
		message = strings.TrimSpace(parsed.ErrorDescription)
	}
	if message == "" && parsed.Detail != nil {
		switch detail := parsed.Detail.(type) {
		case string:
			message = strings.TrimSpace(detail)
		default:
			if data, err := json.Marshal(detail); err == nil {
				message = strings.TrimSpace(string(data))
			}
		}
	}

	switch {
	case code != "" && message != "":
		return code + ": " + message
	case code != "":
		return code
	case message != "":
		return message
	default:
		return raw
	}
}
