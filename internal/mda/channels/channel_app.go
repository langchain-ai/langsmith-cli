package channels

// What the channels that set up an app during `langsmith deploy` share:
// GitHub (github_channel.go) and Linear (linear_channel.go), which keep
// theirs in Agent Auth, and Slack (trigger_channels.go), which Trigger Server
// keeps.
//
//   - the Agent Auth client, with the hidden overrides that point only its
//     calls at another LangSmith;
//   - CLI-only settings from the project's `.env` or the shell;
//   - the webhook URL an app sends the deployment's events to;
//   - the slugs of the app's connection and the secrets stored next to it;
//   - the loop that offers to try a failed setup step again, or to continue
//     the deploy without the channel;
//   - finding the provider's one channel in the project and the manifest
//     checks every app channel shares.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// AgentAuthURLEnv is the hidden override that sends only Agent Auth calls to
// another LangSmith, e.g. a local Agent Auth while the deployment itself runs
// in the cloud. [agentAuthAPIKeyEnv] and [agentAuthTenantEnv] go with it.
const AgentAuthURLEnv = "MDA_AGENT_AUTH_URL"

const (
	agentAuthAPIKeyEnv = "MDA_AGENT_AUTH_API_KEY"
	agentAuthTenantEnv = "MDA_AGENT_AUTH_TENANT_ID"
)

// OAuthCallbackEnv is where Agent Auth's OAuth callback is, when it is not
// where the CLI derives it from the LangSmith URL. The local stack sets it
// (`mda-local env`), since its Agent Auth is configured with a callback the
// CLI cannot know.
const OAuthCallbackEnv = "MDA_AGENT_AUTH_OAUTH_CALLBACK_URL"

// webhookSecretSuffix names the secret connection next to an app's, holding
// its webhook secret.
const webhookSecretSuffix = "/webhook-secret"

// appChannelManifest is a channel manifest of an app channel; C is the
// provider's config.
type appChannelManifest[C any] struct {
	Name string `json:"name" strict:"required"`
	// Provider is selected by before decoding.
	Provider            string   `json:"provider" strict:"required"`
	Transport           *string  `json:"transport" strict:"nullable"`
	AutoReply           bool     `json:"autoReply"`
	EventsPath          string   `json:"eventsPath" strict:"required"`
	RequiredPermissions []string `json:"requiredPermissions"`
	RequiredEnv         []string `json:"requiredEnv"`
	// UserConnections is part of the channel manifest schema; unused by app
	// setup.
	UserConnections any `json:"userConnections" strict:"nullable"`
	// AgentConnections is part of the channel manifest schema; read by
	// connection checks.
	AgentConnections any `json:"agentConnections" strict:"nullable"`
	ProviderConfig   C   `json:"providerConfig" strict:"required"`
}

// findAppChannelManifest is the provider's channel, if the project declares
// one, after the checks every app channel shares: there is only one, it is
// named by its key, it receives events directly on an absolute path, and
// declares no permissions or env. label names the provider in errors, such as
// `GitHub`.
func findAppChannelManifest[C any](requirements introspect.ChannelRequirements, provider, label string) (*appChannelManifest[C], error) {
	var names []string
	for _, name := range sortedKeys(requirements) {
		if value, ok := stringField(requirements[name], "provider"); ok && value == provider {
			names = append(names, name)
		}
	}
	switch len(names) {
	case 0:
		return nil, nil
	case 1:
	default:
		return nil, fmt.Errorf("An agent can have only one %s channel (found: %s).", label, strings.Join(names, ", "))
	}
	key := names[0]
	var manifest appChannelManifest[C]
	if err := decodeManifest(requirements[key], &manifest, true); err != nil {
		return nil, fmt.Errorf("%s channel configuration is invalid.: %w", label, err)
	}
	if manifest.Name != key {
		return nil, errors.New("Channel name must match its manifest key.")
	}
	if manifest.Transport != nil {
		return nil, fmt.Errorf("%s channels receive events directly and must not set a transport.", label)
	}
	if len(manifest.RequiredPermissions) > 0 || len(manifest.RequiredEnv) > 0 {
		return nil, fmt.Errorf("%s channel configuration must not declare requiredPermissions or requiredEnv.", label)
	}
	path := manifest.EventsPath
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "://") || strings.Contains(path, "?") {
		return nil, fmt.Errorf("%s channel events path must be an absolute path on the Agent Server.", label)
	}
	return &manifest, nil
}

// isInteractive reports whether deploy can ask questions: both a terminal to
// read from and one to show prompts on.
func isInteractive() bool {
	return ui.Interactive()
}

// overrideValue is a CLI-only setting from the shell, else from the project's
// `.env`, like other CLI settings. The deploy never uploads these as secrets
// (see [deployment.ReservedEnvVars]).
func overrideValue(key string, envVars map[string]string) (string, bool) {
	return hostapi.LookupEnvOrProcess(key, envVars)
}

// agentAuth is the client for Agent Auth calls, honoring the hidden overrides
// in the shell or the project's `.env`.
func agentAuth(deploy *hostapi.Client, envVars map[string]string) (*hostapi.Client, error) {
	return agentAuthClient(deploy, func(key string) (string, bool) { return overrideValue(key, envVars) })
}

// agentAuthClient is the client for Agent Auth calls: the deploy client,
// unless the hidden [AgentAuthURLEnv] override points them at another
// LangSmith.
//
// That LangSmith is not the one the deploy is signed in to, so neither the
// deploy's credential nor any other the CLI has goes there: the override
// needs [agentAuthAPIKeyEnv] too.
func agentAuthClient(deploy *hostapi.Client, lookup func(string) (string, bool)) (*hostapi.Client, error) {
	agentAuthURL, ok := lookup(AgentAuthURLEnv)
	if !ok {
		return deploy, nil
	}
	apiKey, ok := lookup(agentAuthAPIKeyEnv)
	if !ok {
		return nil, fmt.Errorf("%s is set, so %s must be set too", AgentAuthURLEnv, agentAuthAPIKeyEnv)
	}
	tenant, _ := lookup(agentAuthTenantEnv)
	ls := client.NewWithOptions(client.Options{
		APIKey:      apiKey,
		APIURL:      agentAuthURL,
		WorkspaceID: tenant,
		Isolated:    true,
	})
	return hostapi.New(ls, agentAuthURL, agentAuthURL)
}

// oauthCallbackURL is where Agent Auth's OAuth callback is, for an app that
// must list it before Agent Auth starts its first authorization:
// [OAuthCallbackEnv] when set, else what Agent Auth publishes, else the
// callback on the LangSmith API Agent Auth is reached at. The install session
// later names the redirect URI Agent Auth really uses, and setup checks it
// then.
func oauthCallbackURL(ctx context.Context, agentAuth *hostapi.Client, envVars map[string]string) (string, error) {
	configured, _ := overrideValue(OAuthCallbackEnv, envVars)
	callback, err := configuredOAuthCallback(configured)
	if err != nil || callback != "" {
		return callback, err
	}
	// Only a 404 (not found) means "not published": an older Agent Auth, or a
	// callback it does not publish (not HTTPS). Any other failure, such as a
	// network error, a 5xx or a rejected credential, says nothing about the
	// callback, and guessing it would break the app's OAuth install later.
	published, found, err := agentAuth.PublishedOAuthRedirectURI(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read Agent Auth's OAuth callback from its client metadata: %w", err)
	}
	if found {
		return published, nil
	}
	return agentAuth.OAuthRedirectURI()
}

// configuredOAuthCallback is [OAuthCallbackEnv], checked; "" when unset.
func configuredOAuthCallback(configured string) (string, error) {
	if configured == "" {
		return "", nil
	}
	parsed, err := url.Parse(configured)
	valid := err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
	if !valid {
		return "", fmt.Errorf("%s must be an http(s) URL", OAuthCallbackEnv)
	}
	return configured, nil
}

// joinNames is `a`, `a and b`, `a, b and c`.
func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
}

// envAppNotice is what deploy says when a channel's app comes from CLI-only
// env vars: which ones, and that every one of them can go once store keeps
// the app.
func envAppNotice(app, store string, vars []string) string {
	names := joinNames(vars)
	return fmt.Sprintf("Using the %s from %s. %s keeps its credentials, so you can remove %s afterwards.", app, names, store, names)
}

// webhookURL is the validated `https://` Agent Server URL plus the channel's
// events path.
func webhookURL(agentServerURL, eventsPath string) (string, error) {
	if _, err := deployment.ValidatedAgentServerURL(agentServerURL); err != nil {
		return "", err
	}
	return strings.TrimRight(agentServerURL, "/") + eventsPath, nil
}

// appSlugs are the slugs of a channel's app connection, one per deployment,
// and of the secrets stored next to it.
type appSlugs struct {
	// prefix is what every deployment's app connection slug starts with, e.g.
	// `mda/github-app/`.
	prefix string
}

// connection is the connection that holds the deployment's app.
func (s appSlugs) connection(deploymentID string) string {
	return s.prefix + deploymentID
}

// secret is a value stored next to the app's connection, e.g.
// [webhookSecretSuffix].
func (s appSlugs) secret(deploymentID, suffix string) string {
	return s.connection(deploymentID) + suffix
}

// deploymentOf is the deployment an app connection slug belongs to.
func (s appSlugs) deploymentOf(slug string) (string, bool) {
	deploymentID, ok := strings.CutPrefix(slug, s.prefix)
	if !ok || deploymentID == "" || strings.Contains(deploymentID, "/") {
		return "", false
	}
	return deploymentID, true
}

// findConnection is the listed connection with slug.
func findConnection(listed []hostapi.ConnectionSummary, slug string) (hostapi.ConnectionSummary, bool) {
	for _, connection := range listed {
		if connection.Slug == slug {
			return connection, true
		}
	}
	return hostapi.ConnectionSummary{}, false
}

// setupUI is what a channel's setup shows, and how it asks what to do after
// a failed step. Each channel's setup environment builds on it, so its flow
// stays testable.
type setupUI interface {
	interactive() bool
	// retryOrSkip shows why a setup step failed and asks whether to try again
	// (true) or continue the deploy without the channel (false).
	retryOrSkip(ctx context.Context, err error) (bool, error)
	success(message string) error
	info(message string) error
	warning(message string) error
}

// retryOrSkip runs attempt until it succeeds. The deployment is already live,
// so a failed attempt offers to try again, from what refresh finds then, or to
// continue without the channel: false, after warning skipped. Without a
// terminal nobody can choose, so the error ends the deploy.
func retryOrSkip[C, T any](ctx context.Context, env setupUI, current C, skipped string, attempt func(C) (T, error), refresh func() (C, error)) (T, bool, error) {
	var zero T
	for {
		done, err := attempt(current)
		if err == nil {
			return done, true, nil
		}
		if !env.interactive() {
			return zero, false, err
		}
		retry := false
		if !errs.IsCancellation(err) {
			if retry, err = env.retryOrSkip(ctx, err); err != nil {
				return zero, false, err
			}
		}
		if !retry {
			if err := env.warning(skipped); err != nil {
				return zero, false, err
			}
			return zero, false, nil
		}
		if current, err = refresh(); err != nil {
			return zero, false, err
		}
	}
}

// waitTick is how a wait while setup polls for the user's action ended.
type waitTick int

const (
	waitElapsed waitTick = iota
	waitInterrupted
)

// wait sleeps for duration, or until Ctrl-C cancels ctx.
func wait(ctx context.Context, duration time.Duration) waitTick {
	proc.SleepUntilInterrupted(ctx, duration)
	if ctx.Err() != nil {
		return waitInterrupted
	}
	return waitElapsed
}

// askRetryOrSkip is [setupUI.retryOrSkip] in a terminal: setup names what did
// not finish ("GitHub App setup"), channel the channel ("GitHub").
func askRetryOrSkip(ctx context.Context, setup, channel string, err error) (bool, error) {
	if logErr := ui.LogWarning(fmt.Sprintf("%s did not finish: %v", setup, err)); logErr != nil {
		return false, logErr
	}
	retry, promptErr := ui.Select[bool]("How do you want to continue?").
		Item(true, "Try again", "").
		Item(false, fmt.Sprintf("Continue without the %s channel", channel), "set it up on the next deploy").
		Interact(ctx)
	// Escape or Ctrl-C at this prompt means "skip".
	if promptErr != nil {
		return false, nil
	}
	return retry, nil
}

// warnBestEffort shows a warning that must not end setup when the terminal
// cannot show it.
func warnBestEffort(message string) {
	if err := ui.LogWarning(message); err != nil {
		slog.Debug("failed to show a warning", "warning", message, "error", err)
	}
}

// infoBestEffort is [warnBestEffort] for an info message.
func infoBestEffort(message string) {
	if err := ui.LogInfo(message); err != nil {
		slog.Debug("failed to show a message", "message", message, "error", err)
	}
}

// terminalUI is [setupUI]'s messages in a terminal.
type terminalUI struct{}

func (terminalUI) success(message string) error { return ui.LogSuccess(message) }

func (terminalUI) info(message string) error { return ui.LogInfo(message) }

func (terminalUI) warning(message string) error { return ui.LogWarning(message) }
