// Package deployment holds what every `langsmith deploy` step and the commands
// beside it share about a LangSmith Deployment: where its LangSmith calls go
// and who they go as ([LangSmithContext], `deploy/langsmith_context.rs`), the
// pure deploy helpers (`deploy/helpers.rs`: deployment secrets, sandbox and
// recipe snapshot names, endpoint derivation, deployment lookups and status),
// and the credential and target helpers of `deploy/mod.rs` that other
// packages reach.
//
// # Authentication
//
// The credential is the LangSmith CLI's (see [LangSmithContext] for the
// precedence). `--api-key` comes first; a `LANGSMITH_API_KEY` in the shell or
// the project `.env` is still honoured ahead of the selected profile, as mda
// does; otherwise the profile's API key or OAuth token is used, and an
// interactive command may sign in through [CLI.Login]. The CLI's credentials
// only go to the LangSmith they are for. Nothing here prompts
// for, mints, caches or writes an API key: `resolve_api_key`'s paste prompt and
// device login, the CLI PAT cache and every helper that told a session PAT
// apart from a durable key (`ensure_project_langsmith_api_key`,
// `is_cli_session_env_entry`, the gateway-alias mirroring) are dropped, as is
// `client_without_prompt`, which only telemetry called.
// `resolve_api_key` is [ResolveCredential]; it returns [client.Options], from
// which [LangSmithContext] builds every client with [client.NewWithOptions].
//
// # Names
//
// Rust `Option<String>` results are a string, empty for None; an optional
// JSON object is a nil map. A deployment object or list response is the
// `map[string]any` the [hostapi.Client] methods return. Functions that took
// `&mut Vec<Secret>` take a pointer to the slice.
package deployment

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// AgentTarget resolves the target of an agent-mode command: in agent mode
// (environment non-nil) the deployment name is the agent ID. Returns the name
// of the deploy-owned resources (see [hostapi.DeploymentAgent.ResourceName])
// and the agent, nil outside agent mode.
func AgentTarget(name string, environment *hostapi.AgentEnvironment) (string, *hostapi.DeploymentAgent, error) {
	if environment == nil {
		return name, nil, nil
	}
	if err := project.ValidateAgentID(name); err != nil {
		return "", nil, err
	}
	agent := &hostapi.DeploymentAgent{AgentID: name, Environment: *environment}
	resourceName, err := agent.ResourceName()
	if err != nil {
		return "", nil, err
	}
	return resourceName, agent, nil
}

// FindTargetDeployment is the deployment this command targets: the agent's
// deployment in agent mode (agent deployments are server-named), otherwise the
// exact-name match; nil when there is none.
func FindTargetDeployment(ctx context.Context, c *hostapi.Client, name string, agent *hostapi.DeploymentAgent) (map[string]any, error) {
	if agent != nil {
		response, err := c.ListAgentDeployments(ctx, *agent)
		if err != nil {
			return nil, err
		}
		return FindAgentDeployment(response), nil
	}
	response, err := c.ListDeployments(ctx, name)
	if err != nil {
		return nil, err
	}
	return FindDeploymentByName(response, name), nil
}

// WithManagedMarker prefers a deployment object that actually carries the
// Managed Deep Agent marker.
//
// List responses are trimmed, so a marker missing from the listing means
// "unknown", not "not ours" — and reading unknown as not-ours would make every
// redeploy look like a collision. Re-read the full object once before letting
// the caller conclude anything. A failed or still-unmarked re-read keeps the
// listed object, which errs toward asking rather than silently overwriting.
func WithManagedMarker(ctx context.Context, c *hostapi.Client, deployment map[string]any) map[string]any {
	if _, ok := DeploymentManagedDeepAgentMarker(deployment); ok {
		return deployment
	}
	id, ok := deployment["id"].(string)
	if !ok {
		return deployment
	}
	full, err := c.GetDeployment(ctx, id)
	if err != nil {
		slog.Debug("deployment re-read for the Managed Deep Agent marker failed", "error", err)
		return deployment
	}
	if _, ok := DeploymentManagedDeepAgentMarker(full); ok {
		return full
	}
	return deployment
}

// ResolveDeploymentID is the ID of the deployment named name. Fails when
// nothing matches; `delete` calls [FindDeploymentIDByName] directly instead,
// because a missing deployment is a normal teardown outcome there rather than
// an error.
func ResolveDeploymentID(ctx context.Context, c *hostapi.Client, name string) (string, error) {
	listed, err := c.ListDeployments(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to reach LangSmith: %w", err)
	}
	if id := FindDeploymentIDByName(listed, name); id != "" {
		return id, nil
	}
	dashboard := ""
	if url, err := c.DashboardURL(ctx); err == nil {
		dashboard = " (" + url + ")"
	} else {
		slog.Debug("dashboard URL lookup failed", "error", err)
	}
	return "", fmt.Errorf("No deployment named '%s' exists in this workspace.\n"+
		"Deploy it with `langsmith deploy` first, or pass `-n <name>` / `--workspace-id <id>` "+
		"if it lives under a different name or workspace%s.", name, dashboard)
}

// ResolveCredential resolves the LangSmith credential for deploy and the
// commands beside it (`resolve_api_key`).
//
// Order:
//  1. `--api-key`.
//  2. `LANGSMITH_API_KEY` from the process env, then the project `.env`
//     (envVars, read from envPath); `LANGCHAIN_API_KEY` is an error.
//  3. The CLI's credential: the selected profile's API key or OAuth token.
//  4. On a terminal, [CLI.Login] at platformAPIURL.
//
// The CLI's credentials (1 and 3) are for the CLI's API URL, so they are only
// used when platformAPIURL has its origin: a project's `.env` must not send
// them to another server. Otherwise it fails with an authentication error.
//
// The options returned carry the credential and, for the CLI's, the
// workspace that came with it; a `LANGSMITH_API_KEY` comes with none, as its
// workspace is `LANGSMITH_WORKSPACE_ID`'s ([ResolveWorkspaceID]).
// [LangSmithEndpoints.WithAuth] sets the API URL and the resolved workspace.
func ResolveCredential(ctx context.Context, cli CLI, envPath string, envVars map[string]string, platformAPIURL string) (client.Options, error) {
	if cli.APIKey != "" {
		options, err := cli.options()
		if err != nil {
			return client.Options{}, err
		}
		if client.SameOrigin(options.APIURL, platformAPIURL) {
			return options, nil
		}
	}
	key, ok, err := EnvAPIKey(envVars)
	if err != nil {
		return client.Options{}, err
	}
	if ok {
		return client.Options{APIKey: key}, nil
	}
	options, err := cli.options()
	if err != nil {
		return client.Options{}, err
	}
	var unusable error
	switch {
	case options.HasAuth() && client.SameOrigin(options.APIURL, platformAPIURL):
		return options, nil
	case options.HasAuth():
		unusable = errs.Categorized(errs.Authentication, fmt.Sprintf(
			"The LangSmith CLI's credentials are for %s, not %s, so they are not sent there. "+
				"Run `langsmith auth login --api-url %s`, or set %s in the environment or in %s",
			options.APIURL, platformAPIURL, platformAPIURL, hostapi.APIKeyEnv, envPath))
	default:
		unusable = errs.Categorized(errs.Authentication, fmt.Sprintf(
			"No LangSmith credentials found. Run `langsmith auth login`, or set %s in the "+
				"environment or in %s", hostapi.APIKeyEnv, envPath))
	}
	if cli.Login == nil || !ui.Interactive() {
		return client.Options{}, unusable
	}
	signedIn, err := cli.Login(ctx, platformAPIURL)
	if err != nil {
		return client.Options{}, errs.CategorizeUnclassified(errs.Authentication, err)
	}
	if !signedIn.HasAuth() {
		return client.Options{}, unusable
	}
	return signedIn, nil
}

// EnvAPIKey is `LANGSMITH_API_KEY` from the process env, then the env file.
// Errors when only a removed name for it is set.
func EnvAPIKey(envVars map[string]string) (string, bool, error) {
	return hostapi.ResolvePreferredEnv(
		hostapi.APIKeyEnv,
		hostapi.RemovedAPIKeyEnvs,
		"",
		envVars,
		os.LookupEnv,
		" instead (same value, renamed)",
	)
}
