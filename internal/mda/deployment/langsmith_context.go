package deployment

// Where a command's LangSmith calls go and who they go as
// (`deploy/langsmith_context.rs`).
//
// Every command that talks to LangSmith resolves the same four things in the
// same order — host backend, platform API, credential, workspace — before it
// builds a [hostapi.Client] or [hostapi.ContextHubClient].
// [ResolveLangSmithContext] does that once so `--host-url` and
// `--workspace-id` mean the same thing in every command.

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

// WorkspaceFlagHint completes the removed-alias error for commands that take
// `--workspace-id`.
const WorkspaceFlagHint = " or pass --workspace-id instead"

// CLI is what the LangSmith CLI brings to this invocation, before the project
// had a say: the cmd layer owns the global flags and profiles, so it hands
// them in.
type CLI struct {
	// APIURL is `--api-url`, "" when not given. It names the platform API
	// ahead of every `LANGSMITH_ENDPOINT`.
	APIURL string
	// APIKey is `--api-key`, "" when not given. It is the credential ahead of
	// every `LANGSMITH_API_KEY`, for the platform API it belongs to.
	APIKey string
	// Options resolves the client options the CLI would use on its own, from
	// its flags (`--api-key`, `--api-url`, `--workspace`), the process env
	// and the selected profile (an API key or an OAuth token). It is called
	// only when they are needed, so a broken profile does not stop a command
	// that authenticates with a `LANGSMITH_API_KEY`. Nil when the CLI has
	// none.
	Options func() (client.Options, error)
	// Login signs in interactively, the way `langsmith auth login` does, at
	// platformAPIURL, and returns the options of the new credential. Nil when
	// the command cannot sign in.
	Login func(ctx context.Context, platformAPIURL string) (client.Options, error)
}

// options are [CLI.Options], none when the CLI has none.
func (c CLI) options() (client.Options, error) {
	if c.Options == nil {
		return client.Options{}, nil
	}
	return c.Options()
}

// LangSmithEndpoints is the LangSmith installation a command targets.
type LangSmithEndpoints struct {
	// HostURL is the host backend (Deployments) base URL.
	HostURL string
	// PlatformAPIURL is the platform API (Fleet, secrets, sandboxes) base URL.
	PlatformAPIURL string
}

// ResolveLangSmithEndpoints picks the installation: a non-blank hostOverride
// (`--host-url`) wins for the host backend; otherwise `LANGSMITH_ENDPOINT`
// picks it, taken from `--api-url`, then the process env, then envVars (the
// project `.env`), then the CLI's own API URL (the selected profile's). The
// platform API is `LANGSMITH_ENDPOINT` (same order), else derived from the
// chosen host.
//
// The CLI's API URL is only consulted when it is not the default
// (`https://api.smith.langchain.com`): the default is what an unset endpoint
// resolves to anyway, and it must not pin the platform API of a `--host-url`
// in another region.
func ResolveLangSmithEndpoints(cli CLI, envVars map[string]string, hostOverride string) LangSmithEndpoints {
	env := endpointLookup(cli, envVars)
	hostURL := strings.TrimSpace(hostOverride)
	if hostURL == "" {
		hostURL, _ = resolveLangsmithAPIBasesWith(env)
	}
	return LangSmithEndpoints{
		HostURL:        hostURL,
		PlatformAPIURL: resolvePlatformAPIURLWith(hostURL, env),
	}
}

// endpointLookup is [processEnvLookup] with `--api-url` as the first source
// of `LANGSMITH_ENDPOINT` and the CLI's own API URL as the last.
func endpointLookup(cli CLI, envVars map[string]string) envLookup {
	project := processEnvLookup(envVars)
	return func(key string) (string, bool) {
		if key != LangsmithEndpointEnv {
			return project(key)
		}
		if apiURL, ok := hostapi.NormalizePlatformAPIURL(cli.APIURL); ok {
			return apiURL, true
		}
		if value, ok := project(key); ok {
			return value, true
		}
		options, err := cli.options()
		if err != nil {
			// The credential step reports it if it needs the CLI's options.
			slog.Debug("the CLI's API URL is unavailable", "error", err)
			return "", false
		}
		apiURL, ok := hostapi.NormalizePlatformAPIURL(options.APIURL)
		if !ok || apiURL == hostapi.DefaultAPIURL {
			return "", false
		}
		return apiURL, true
	}
}

// WithAuth pairs these endpoints with an already-resolved credential and
// workspace. A blank workspaceID leaves the credential's own workspace (the
// CLI's `--workspace` or its profile's default), if any.
func (e LangSmithEndpoints) WithAuth(credential client.Options, workspaceID string) LangSmithContext {
	options := credential
	options.APIURL = e.PlatformAPIURL
	if workspaceID != "" {
		options.WorkspaceID = workspaceID
	}
	return LangSmithContext{Endpoints: e, Options: options}
}

// LangSmithContext is the endpoints plus the credential and workspace every
// LangSmith request carries.
//
// [ResolveLangSmithContext] resolves it in this order:
//
//  1. Host backend: `--host-url`, else derived from `LANGSMITH_ENDPOINT` read
//     from `--api-url`, then the process env, then the project `.env`, then
//     the CLI's own API URL; else `https://api.host.langchain.com`.
//  2. Platform API: `LANGSMITH_ENDPOINT` in the same order, else derived from
//     the host backend (`api.host.*` → `api.smith.*`).
//  3. Credential ([ResolveCredential]): `--api-key`, then `LANGSMITH_API_KEY`
//     from the process env, then the project `.env` (`LANGCHAIN_API_KEY`
//     there is an error); else the CLI's credential (the selected profile's
//     API key or OAuth token); else, on a terminal, [CLI.Login]; else an
//     authentication error naming `langsmith auth login`. The CLI's
//     credentials (`--api-key` included) only go to the platform API they
//     belong to.
//  4. Workspace ([ResolveWorkspaceID]): `--workspace-id`, else
//     `LANGSMITH_WORKSPACE_ID` from the process env, then the project `.env`
//     (removed aliases are an error); else, with the CLI's credential, the
//     CLI's workspace (`--workspace` or the profile's default); else the
//     credential's default workspace.
//
// The process env wins over the project `.env` as everywhere in mda, with a
// warning when the two disagree.
type LangSmithContext struct {
	Endpoints LangSmithEndpoints
	// Options authenticate every request: the credential, the workspace, and
	// the platform API as the API URL.
	Options client.Options
}

// ResolveLangSmithContext resolves endpoints, then the credential, then the
// workspace (see [LangSmithContext]).
//
// envVars is the project `.env` at envPath, which the missing-credential error
// names. The workspace is workspaceFlag when non-blank; workspaceHint completes
// the error for a removed alias.
func ResolveLangSmithContext(ctx context.Context, cli CLI, envPath string, envVars map[string]string, hostOverride, workspaceFlag, workspaceHint string) (LangSmithContext, error) {
	endpoints := ResolveLangSmithEndpoints(cli, envVars, hostOverride)
	credential, err := ResolveCredential(ctx, cli, envPath, envVars, endpoints.PlatformAPIURL)
	if err != nil {
		return LangSmithContext{}, err
	}
	workspaceID, err := ResolveWorkspaceID(envVars, workspaceFlag, workspaceHint)
	if err != nil {
		return LangSmithContext{}, err
	}
	return endpoints.WithAuth(credential, workspaceID), nil
}

// HostURL is the host backend base URL.
func (c LangSmithContext) HostURL() string {
	return c.Endpoints.HostURL
}

// PlatformAPIURL is the platform API base URL.
func (c LangSmithContext) PlatformAPIURL() string {
	return c.Endpoints.PlatformAPIURL
}

// WorkspaceID is the workspace requests are scoped to, "" when the
// credential's default workspace applies.
func (c LangSmithContext) WorkspaceID() string {
	return c.Options.WorkspaceID
}

// LangSmithClient is the LangSmith CLI client that authenticates as this
// context, for typed SDK calls.
func (c LangSmithContext) LangSmithClient() *client.Client {
	options := c.Options
	options.Isolated = true
	return client.NewWithOptions(options)
}

// Client is the client for the host backend and platform API.
func (c LangSmithContext) Client() (*hostapi.Client, error) {
	return hostapi.New(c.LangSmithClient(), c.HostURL(), c.PlatformAPIURL())
}

// ContextHubClient is the client for the Context Hub that pairs with these
// endpoints.
func (c LangSmithContext) ContextHubClient() (*hostapi.ContextHubClient, error) {
	return hostapi.NewContextHubClient(
		c.LangSmithClient(),
		ResolveContextHubBaseURL(c.HostURL(), c.PlatformAPIURL()),
	)
}

// ResolveWorkspaceID is workspaceFlag when non-blank, else
// `LANGSMITH_WORKSPACE_ID` from the process env, then envVars; "" when none is
// set. Errors on a removed alias.
func ResolveWorkspaceID(envVars map[string]string, workspaceFlag, workspaceHint string) (string, error) {
	workspaceID, _, err := hostapi.ResolvePreferredEnv(
		WorkspaceIDEnv,
		RemovedWorkspaceIDEnvs,
		workspaceFlag,
		envVars,
		os.LookupEnv,
		workspaceHint,
	)
	return workspaceID, err
}
