package channels

// GitHub channel setup during `langsmith deploy`.
//
// A GitHub channel needs a GitHub App, stored in Agent Auth as an app the
// deployment owns, with a `github_app` OAuth connection that mints its
// tokens. Agent Auth owns the App flow: it serves the page that sends the
// manifest to GitHub, exchanges GitHub's code for the App's credentials, and
// records the installation. The CLI never sees the private key of an App it
// creates.
//
// Setup runs once the deployment is live, because the App's webhook points at
// the Agent Server URL:
//
//  1. If the deployment has no installed App, start a hosted setup and wait
//     for the user to create and install the App in the browser, or store an
//     existing App's ID and key and record its installation.
//  2. Ask Agent Auth for the App's report, point its webhook at the
//     deployment, and report what is ready and what someone still has to fix
//     on GitHub.
//  3. When the channel needs other permissions than the agent's token has,
//     and the installation already grants them, renew the token with them.
//
// `mda delete` removes the App and its secrets.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/mda/connection"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// githubSetupExpired is a GitHub App setup session that expired.
const githubSetupExpired = "The GitHub App setup was not finished in time. If an organization owner has to approve the installation, ask them to, then run `langsmith deploy` again."

const (
	installPollInterval = 3 * time.Second
	installWaitLimit    = 30 * 60 * time.Second
	// syncAttempts: GitHub can briefly fail right after an App is created or
	// installed.
	syncAttempts   = 6
	syncRetryDelay = 5 * time.Second
)

// CLI-only settings for the GitHub channel; never uploaded.
const (
	// GitHubWebhookSecretEnv is a brought App's webhook secret, which deploy
	// stores in Agent Auth. The runtime reads it from there, unless the
	// deployment's own `GH_WEBHOOK_SECRET` overrides it; deploy never reads
	// that one.
	GitHubWebhookSecretEnv = "MDA_GITHUB_WEBHOOK_SECRET"
	// GitHubAppIDEnv and GitHubPrivateKeyFileEnv bring an existing App instead
	// of creating one, with [GitHubWebhookSecretEnv];
	// GitHubInstallationIDEnv is the installation it uses when GitHub reports
	// none or several.
	GitHubAppIDEnv          = "MDA_GITHUB_APP_ID"
	GitHubPrivateKeyFileEnv = "MDA_GITHUB_APP_PRIVATE_KEY_FILE"
	GitHubInstallationIDEnv = "MDA_GITHUB_APP_INSTALLATION_ID"
	// GitHubWebhookElsewhereEnv keeps a brought App's webhook at this URL
	// without a terminal to ask.
	GitHubWebhookElsewhereEnv = "MDA_GITHUB_WEBHOOK_ELSEWHERE"
	// GitHubAppOwnerEnv is where a new App is created: an organization login,
	// or [githubPersonalOwner]. Without a terminal deploy cannot ask, so this
	// must be set.
	GitHubAppOwnerEnv = "MDA_GITHUB_APP_OWNER"
)

// botIDSuffix names a secret connection next to the App's, which the runtime
// reads. Agent Auth stores the webhook secret ([webhookSecretSuffix]) with the
// App; deploy stores the bot ID once Agent Auth has recorded it.
const botIDSuffix = "/bot-id"

// webhookElsewhereSuffix: where a brought App's webhook stays on purpose
// instead of pointing at this deployment, e.g. a service that receives the
// events and forwards them. Deploy stores one marker per kept URL,
// `…/webhook-elsewhere/<digest>`, next to the App's connection, and only
// checks that it exists: it never reads a stored value back.
const webhookElsewhereSuffix = "/webhook-elsewhere/"

// webhookDigestLength is how many hex characters of a kept URL's SHA-256 are
// in its marker's slug.
const webhookDigestLength = 16

const githubNoHandlers = "The GitHub channel handles no events yet, so no GitHub App is set up. Add handlers with `channels.github({ onEvent: { ... } })` and deploy again."

const githubSkippedSetup = "Continuing without the GitHub channel. The deployment is live, but it receives no GitHub events until you run `langsmith deploy` again and finish setting up the GitHub App."

const githubPersonalOwner = "@me"

const githubOwnerRequired = "This agent's GitHub channel needs a GitHub App, and without a terminal deploy cannot ask for it. Set MDA_GITHUB_APP_OWNER in the shell or .env to the GitHub organization that should own a new App, or to @me for the personal account of whoever creates it. To use an existing App, set MDA_GITHUB_APP_ID, MDA_GITHUB_APP_PRIVATE_KEY_FILE and MDA_GITHUB_WEBHOOK_SECRET instead. Then run `langsmith deploy` again."

const (
	githubAppIDHint          = "Enter the numeric App ID from the App's settings page."
	githubInstallationIDHint = "Enter the numeric installation ID from the end of the installation's settings URL."
)

const existingAppNeedsTerminal = "This agent's GitHub App was brought in by its ID and key and is not installed in exactly one account yet. Install it on GitHub, or set MDA_GITHUB_APP_INSTALLATION_ID in the shell or .env to the installation this agent uses, then run `langsmith deploy` again. Or run `langsmith deploy` in a terminal once to choose or install it."

const missingWebhookSecret = "Agent Auth has no webhook secret for this GitHub App, so the channel rejects its events unless the deployment sets GH_WEBHOOK_SECRET. Set MDA_GITHUB_WEBHOOK_SECRET in the shell or .env, or run `langsmith deploy` in a terminal, to add it."

// existingAppEnv is an existing App from the CLI-only env vars. Formatting it
// prints only the App ID.
type existingAppEnv struct {
	appID         string
	privateKey    hostapi.PrivateKeyPem
	webhookSecret string
}

// String names the App without its key or secret.
func (e existingAppEnv) String() string {
	return fmt.Sprintf("ExistingAppEnv { app_id: %q, .. }", e.appID)
}

// GoString is [existingAppEnv.String].
func (e existingAppEnv) GoString() string {
	return e.String()
}

// existingGitHubAppEnv is [GitHubAppIDEnv] and [GitHubPrivateKeyFileEnv],
// both or neither (nil), checked, with [GitHubWebhookSecretEnv], which
// bringing an App needs.
func existingGitHubAppEnv(envVars map[string]string) (*existingAppEnv, error) {
	appID, hasAppID := overrideValue(GitHubAppIDEnv, envVars)
	keyFile, hasKeyFile := overrideValue(GitHubPrivateKeyFileEnv, envVars)
	switch {
	case !hasAppID && !hasKeyFile:
		return nil, nil
	case !hasAppID || !hasKeyFile:
		return nil, fmt.Errorf("Set both %s and %s, or neither.", GitHubAppIDEnv, GitHubPrivateKeyFileEnv)
	}
	id, err := parseGitHubID(appID, githubAppIDHint)
	if err != nil {
		return nil, fmt.Errorf("%s is not valid: %w", GitHubAppIDEnv, err)
	}
	privateKey, err := readPrivateKey(strings.TrimSpace(keyFile))
	if err != nil {
		return nil, fmt.Errorf("%s does not name a usable private key: %w", GitHubPrivateKeyFileEnv, err)
	}
	webhookSecret, ok := githubWebhookSecretEnv(envVars)
	if !ok {
		return nil, fmt.Errorf("%s brings an existing GitHub App, which needs its webhook secret too: set %s.", GitHubAppIDEnv, GitHubWebhookSecretEnv)
	}
	return &existingAppEnv{
		appID:         strconv.FormatUint(id, 10),
		privateKey:    privateKey,
		webhookSecret: webhookSecret,
	}, nil
}

func githubWebhookSecretEnv(envVars map[string]string) (string, bool) {
	secret, _ := overrideValue(GitHubWebhookSecretEnv, envVars)
	secret = strings.TrimSpace(secret)
	return secret, secret != ""
}

// installationIDEnv is [GitHubInstallationIDEnv], checked; "" when unset.
func installationIDEnv(envVars map[string]string) (string, error) {
	value, ok := overrideValue(GitHubInstallationIDEnv, envVars)
	if !ok {
		return "", nil
	}
	id, err := parseGitHubID(value, githubInstallationIDHint)
	if err != nil {
		return "", fmt.Errorf("%s is not valid: %w", GitHubInstallationIDEnv, err)
	}
	return strconv.FormatUint(id, 10), nil
}

// githubAppEnv is what the CLI-only env vars say about the App, all checked:
// a new App's owner, or an existing App, never both.
type githubAppEnv struct {
	owner    *appOwner
	existing *existingAppEnv
	// installationID is "" when unset.
	installationID string
}

func readGitHubAppEnv(envVars map[string]string) (githubAppEnv, error) {
	var owner *appOwner
	if value, ok := overrideValue(GitHubAppOwnerEnv, envVars); ok {
		parsed, err := parseOwner(value)
		if err != nil {
			return githubAppEnv{}, err
		}
		owner = &parsed
	}
	existing, err := existingGitHubAppEnv(envVars)
	if err != nil {
		return githubAppEnv{}, err
	}
	if owner != nil && existing != nil {
		return githubAppEnv{}, fmt.Errorf("Set %s to create a new GitHub App, or %s and %s to use an existing one, not both.",
			GitHubAppOwnerEnv, GitHubAppIDEnv, GitHubPrivateKeyFileEnv)
	}
	installationID, err := installationIDEnv(envVars)
	if err != nil {
		return githubAppEnv{}, err
	}
	return githubAppEnv{owner: owner, existing: existing, installationID: installationID}, nil
}

// githubSlugs are the `github_app` connection that holds a deployment's App,
// and the values stored next to it. One App belongs to one connection, so the
// slug is per deployment.
var githubSlugs = appSlugs{prefix: "mda/github-app/"}

type githubAppSource int

const (
	githubAppNew githubAppSource = iota
	githubAppExisting
)

// githubAppConnection is the deployment's App in Agent Auth, with its
// `github_app` connection.
type githubAppConnection struct {
	app hostapi.AgentAuthApp
	// providerID is the connection's OAuth provider; "" while the App is
	// pending.
	providerID string
	state      hostapi.GitHubAppState
	// scopes are the `permission:level` scopes the agent's token is minted
	// with, sorted.
	scopes []string
	// installed reports whether the agent already holds a credential on it,
	// i.e. the App is installed and set up.
	installed bool
	// webhookSecretStored and botIDStored report whether the App's webhook
	// secret and bot ID are stored next to it.
	webhookSecretStored bool
	botIDStored         bool
	// keptWebhooks are the digests of the webhook URLs the App keeps on
	// purpose; see [webhookElsewhereSuffix].
	keptWebhooks map[string]bool
}

// githubSetupEnv is everything setup does that touches the terminal, the
// network or Agent Auth, so the flow itself is testable.
type githubSetupEnv interface {
	setupUI
	// appConnection is the deployment's GitHub App, nil when it has none.
	appConnection(ctx context.Context) (*githubAppConnection, error)
	chooseAppSource(ctx context.Context) (githubAppSource, error)
	// createApp creates a pending App built from the channel under owner,
	// with the session that creates and installs it.
	createApp(ctx context.Context, channel *githubChannel, owner appOwner) (githubAppConnection, hostapi.OAuthAuthorizationSession, error)
	// storeExistingApp prompts for an existing App and its webhook secret and
	// stores it.
	storeExistingApp(ctx context.Context) (githubAppConnection, error)
	// chooseOwner asks where a new App should be created.
	chooseOwner(ctx context.Context) (appOwner, error)
	// askInstallationID asks for the installation of an App the user brought.
	askInstallationID(ctx context.Context) (string, error)
	// installationFromEnv is the installation `MDA_GITHUB_APP_INSTALLATION_ID`
	// picks, so nobody is asked, with or without a terminal; "" when unset.
	installationFromEnv() string
	// chooseInstallation picks one of several installations of an App the
	// user brought.
	chooseInstallation(ctx context.Context, installations []hostapi.GitHubInstallation) (string, error)
	// appSourceFromEnv reports whether the CLI-only env vars choose a new App
	// (its owner) or an existing one, so nobody is asked.
	appSourceFromEnv() (githubAppSource, bool)
	// webhookSecret is the webhook secret of an App the user brought:
	// `MDA_GITHUB_WEBHOOK_SECRET` when set, else asked for. false when neither
	// is possible.
	webhookSecret(ctx context.Context) (string, bool, error)
	// storeAppValue stores a value the runtime reads as the secret connection
	// next to the App's, e.g. [webhookSecretSuffix].
	storeAppValue(ctx context.Context, suffix, value string) error
	startSession(ctx context.Context, connection *githubAppConnection, start hostapi.GitHubAppSessionStart) (hostapi.OAuthAuthorizationSession, error)
	// openSetupPage shows and opens the page that creates or installs the
	// App.
	openSetupPage(url string, creating bool) error
	// nextSession waits for the session to change and returns it.
	nextSession(ctx context.Context, sessionID string) (hostapi.OAuthAuthorizationSession, error)
	// sessionStatus is the session's status now, without waiting for it to
	// change.
	sessionStatus(ctx context.Context, sessionID string) (string, error)
	// updateTokenScopes mints the agent's token with scopes. Agent Auth drops
	// the current token, so a session has to renew it.
	updateTokenScopes(ctx context.Context, providerID string, scopes []string) error
	// sync is the App as GitHub reports it now.
	sync(ctx context.Context) hostapi.GitHubSyncOutcome
	// webhookURL is where this deployment receives the App's events.
	webhookURL() string
	// confirmWebhookReplace asks whether to point an App's webhook, which
	// goes to current, at this deployment.
	confirmWebhookReplace(ctx context.Context, current string) (bool, error)
	// pointWebhook points the App's webhook at this deployment.
	pointWebhook(ctx context.Context) error
	// webhookElsewhereOverride is `MDA_GITHUB_WEBHOOK_ELSEWHERE`: a webhook
	// URL the App keeps without a terminal to ask; "" when unset.
	webhookElsewhereOverride() string
	// forgetKeptWebhooks deletes the markers of webhook URLs the App kept.
	forgetKeptWebhooks(ctx context.Context) error
	// openInstallPage shows and opens the install page while setup waits for
	// the install.
	openInstallPage(url string) error
	wait(ctx context.Context, duration time.Duration) waitTick
}

// setUpGitHub runs GitHub App setup against a live deployment.
func setUpGitHub(ctx context.Context, client *hostapi.Client, channel *githubChannel, agentServerURL, deploymentID, projectRoot string, envVars map[string]string) error {
	webhook, err := webhookURL(agentServerURL, channel.eventsPath)
	if err != nil {
		return err
	}
	agentAuthClient, err := agentAuth(client, envVars)
	if err != nil {
		return err
	}
	if agentAuthURL, ok := overrideValue(AgentAuthURLEnv, envVars); ok {
		if err := ui.LogInfo("Using Agent Auth at " + agentAuthURL); err != nil {
			return err
		}
	}
	appEnv, err := readGitHubAppEnv(envVars)
	if err != nil {
		return err
	}
	webhookSecretOverride, _ := githubWebhookSecretEnv(envVars)
	webhookElsewhere, _ := overrideValue(GitHubWebhookElsewhereEnv, envVars)
	env := &githubLiveSetup{
		agentAuth:             agentAuthClient,
		deploymentID:          deploymentID,
		agentServerURL:        agentServerURL,
		webhook:               webhook,
		projectRoot:           projectRoot,
		isInteractive:         isInteractive(),
		webhookSecretOverride: webhookSecretOverride,
		ownerOverride:         appEnv.owner,
		existingOverride:      appEnv.existing,
		installationOverride:  appEnv.installationID,
		webhookElsewhereURL:   webhookElsewhere,
	}
	return runGitHubSetup(ctx, env, channel)
}

// preflightGitHub checks, before anything is deployed, that GitHub App setup
// will be able to finish. Malformed or incomplete CLI-only env vars always
// fail here. Without a terminal, setup that would need an answer nobody can
// give fails here too, instead of after the deployment is live. deploymentID
// is the existing deployment ("" for none), runtimeURL its Agent Server URL.
func preflightGitHub(ctx context.Context, client *hostapi.Client, channel *githubChannel, deploymentID, runtimeURL string, envVars map[string]string) error {
	appEnv, err := readGitHubAppEnv(envVars)
	if err != nil {
		return err
	}
	if isInteractive() || len(channel.handled) == 0 {
		return nil
	}
	if deploymentID == "" {
		if problem := unattendedSetupProblem(nil, appEnv); problem != "" {
			return errors.New(problem)
		}
		return nil
	}
	agentAuthClient, err := agentAuth(client, envVars)
	if err != nil {
		return err
	}
	connection, err := lookupGitHubAppConnection(ctx, agentAuthClient, deploymentID)
	if err != nil {
		return fmt.Errorf("failed to check the GitHub App before deploying: %w", err)
	}
	if problem := unattendedSetupProblem(connection, appEnv); problem != "" {
		return errors.New(problem)
	}
	// A brought App whose webhook points elsewhere would fail the deploy once
	// it is live; say so before anything is uploaded.
	if connection == nil || !connection.installed || runtimeURL == "" {
		return nil
	}
	expected, err := webhookURL(runtimeURL, channel.eventsPath)
	if err != nil {
		return nil
	}
	if !webhookNeedsConsent(connection, expected) {
		return nil
	}
	// A failed check is left to setup, which reports it after the deploy.
	outcome := agentAuthClient.GitHubAppReport(ctx, connection.app.ID)
	if outcome.Kind != hostapi.GitHubSyncOutcomeSynced {
		return nil
	}
	current := outcome.Status.App.WebhookURL
	kept, _ := overrideValue(GitHubWebhookElsewhereEnv, envVars)
	if current != "" && current != expected && !keepsWebhook(connection, current, kept) {
		return errors.New(webhookElsewhere(outcome.Status.App.Slug, current, expected))
	}
	return nil
}

// webhookNeedsConsent reports whether moving the App's webhook to expected
// takes it from a receiver someone else set up. Agent Auth points an App it
// created at the deployment from the start, and records where deploy last
// pointed it.
func webhookNeedsConsent(connection *githubAppConnection, expected string) bool {
	return connection.app.Manifest.WebhookURL != expected
}

// claimWebhook: an App has one webhook URL. An App Agent Auth points at this
// deployment follows it; one whose webhook serves something else only moves
// with consent, and without a terminal deploy fails instead. A "No" is
// remembered for that URL, so later deploys, with a terminal or without, keep
// it there and don't ask again.
func claimWebhook(ctx context.Context, env githubSetupEnv, connection *githubAppConnection, status *hostapi.GitHubChannelStatus) error {
	current := status.App.WebhookURL
	if current == env.webhookURL() {
		return forgetKeptWebhooks(ctx, env, connection)
	}
	if current != "" && webhookNeedsConsent(connection, env.webhookURL()) {
		if keepsWebhook(connection, current, env.webhookElsewhereOverride()) {
			if !connection.keptWebhooks[webhookDigest(current)] {
				if err := rememberKeptWebhook(ctx, env, current); err != nil {
					return err
				}
			}
			return env.info(webhookKept(status.App.Slug, current, env.webhookURL()))
		}
		if !env.interactive() {
			return errors.New(webhookElsewhere(status.App.Slug, current, env.webhookURL()))
		}
		replace, err := env.confirmWebhookReplace(ctx, current)
		switch {
		// The deployment is already live: Escape or Ctrl-C keeps the webhook
		// where it is for now, instead of failing the deploy, and the next
		// deploy asks again.
		case err != nil && errs.IsCancellation(err):
			return env.warning(fmt.Sprintf(
				"GitHub App %s keeps sending its events to %s, so this deployment receives none. Run `langsmith deploy` again to point it here.",
				status.App.Slug, current))
		case err != nil:
			return err
		case !replace:
			if err := rememberKeptWebhook(ctx, env, current); err != nil {
				return err
			}
			return env.info(webhookKept(status.App.Slug, current, env.webhookURL()))
		}
	}
	if err := env.pointWebhook(ctx); err != nil {
		return env.warning(fmt.Sprintf(
			"Could not point GitHub App %s's webhook at this deployment (%v), so it receives no events. Run `langsmith deploy` again to retry.",
			status.App.Slug, err))
	}
	status.App.WebhookURL = env.webhookURL()
	return forgetKeptWebhooks(ctx, env, connection)
}

// webhookDigest is the SHA-256 of a webhook URL, shortened, for its marker's
// slug.
func webhookDigest(url string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(url)))
	return hex.EncodeToString(sum[:])[:webhookDigestLength]
}

// keepsWebhook reports whether the App's webhook stays at url on purpose: a
// marker an earlier deploy stored for it, or `MDA_GITHUB_WEBHOOK_ELSEWHERE`
// naming it ("" for unset).
func keepsWebhook(connection *githubAppConnection, url, kept string) bool {
	return connection.keptWebhooks[webhookDigest(url)] ||
		(kept != "" && strings.TrimSpace(kept) == strings.TrimSpace(url))
}

// rememberKeptWebhook remembers that the App's webhook stays at url. Never
// fails the deploy: without the marker, the next deploy just asks again.
func rememberKeptWebhook(ctx context.Context, env githubSetupEnv, url string) error {
	suffix := webhookElsewhereSuffix + webhookDigest(url)
	if err := env.storeAppValue(ctx, suffix, strings.TrimSpace(url)); err != nil {
		return env.warning(fmt.Sprintf(
			"Could not remember to keep the GitHub App's webhook at %s (%v), so the next deploy asks again.", url, err))
	}
	return nil
}

// forgetKeptWebhooks: the webhook points at this deployment now, so an
// earlier "keep it there" no longer applies. Never fails the deploy.
func forgetKeptWebhooks(ctx context.Context, env githubSetupEnv, connection *githubAppConnection) error {
	if len(connection.keptWebhooks) == 0 {
		return nil
	}
	if err := env.forgetKeptWebhooks(ctx); err != nil {
		return env.warning(fmt.Sprintf(
			"Could not forget the webhook URL the GitHub App kept before (%v). Deploy ignores it while the webhook points here.", err))
	}
	return nil
}

func webhookKept(slug, current, expected string) string {
	return fmt.Sprintf("GitHub App %s keeps sending its events to %s. Forward the ones this agent handles to %s, with the body and GitHub's headers unchanged. Deploy doesn't ask again while the webhook stays there; to receive the events here instead, set the App's webhook URL to %s.",
		slug, current, expected, expected)
}

func webhookElsewhere(slug, current, expected string) string {
	return fmt.Sprintf("GitHub App %s sends its events to %s. An App has one webhook URL, so deploy does not move it without asking. Change it to %s on the App's settings page, or run `langsmith deploy` in a terminal to confirm the change, then deploy again. If it stays there on purpose, for example because that service forwards the events here, run `langsmith deploy` in a terminal once and keep it, or set %s=%s.",
		slug, current, expected, GitHubWebhookElsewhereEnv, current)
}

// permissionsRejected is how Agent Auth reports that GitHub refused the
// permissions a token asks for.
const permissionsRejected = "GitHub rejected the requested permissions"

// explainRefusedToken: GitHub refused the agent's first token because the
// installation lacks a permission the channel needs; the channel config is
// the other way out.
func explainRefusedToken(err error) error {
	var api *hostapi.APIError
	if !errors.As(err, &api) || api.Status != http.StatusBadGateway || !strings.Contains(api.Detail, permissionsRejected) {
		return err
	}
	return fmt.Errorf("GitHub refused the agent's token. Grant what is missing to the App on GitHub and accept it on the installation, or remove the handlers or app.permissions that need it from the channel, then try again: %w", err)
}

// unattendedSetupProblem is why setup without a terminal cannot finish, ""
// if it can. connection is nil when the deployment has no App.
func unattendedSetupProblem(connection *githubAppConnection, appEnv githubAppEnv) string {
	switch {
	case connection == nil:
		if appEnv.owner != nil || appEnv.existing != nil {
			return ""
		}
		return githubOwnerRequired
	case connection.installed:
		return ""
	// The install page only needs a link.
	case connection.state == hostapi.GitHubAppStateCreated:
		return ""
	case connection.state == hostapi.GitHubAppStateExternal:
		if appEnv.installationID != "" {
			return ""
		}
		return existingAppNeedsTerminal
	// An App GitHub has not created yet needs its owner.
	case appEnv.owner == nil:
		return githubOwnerRequired
	default:
		return ""
	}
}

// lookupGitHubAppConnection is the deployment's App and what is stored next
// to it; nil when it has none.
func lookupGitHubAppConnection(ctx context.Context, agentAuth *hostapi.Client, deploymentID string) (*githubAppConnection, error) {
	apps, err := agentAuth.AgentApps(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up the GitHub App: %w", err)
	}
	index := slices.IndexFunc(apps, func(app hostapi.AgentAuthApp) bool { return app.Provider == githubProvider })
	if index < 0 {
		return nil, nil
	}
	app := apps[index]
	listed, err := agentAuth.ListConnectionsRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to look up the GitHub App connection: %w", err)
	}
	stored := func(suffix string) bool {
		_, found := findConnection(listed, app.Slug+suffix)
		return found
	}
	keptPrefix := app.Slug + webhookElsewhereSuffix
	keptWebhooks := map[string]bool{}
	for _, connection := range listed {
		if digest, ok := strings.CutPrefix(connection.Slug, keptPrefix); ok {
			keptWebhooks[digest] = true
		}
	}
	found := &githubAppConnection{
		app:                 app,
		state:               app.State(),
		scopes:              []string{},
		webhookSecretStored: stored(webhookSecretSuffix),
		botIDStored:         stored(botIDSuffix),
		keptWebhooks:        keptWebhooks,
	}
	if app.ConnectionID == "" {
		// Pending: GitHub has not created the App, so it has no connection.
		return found, nil
	}
	connectionIndex := slices.IndexFunc(listed, func(connection hostapi.ConnectionSummary) bool {
		return connection.ConnectionID == app.ConnectionID
	})
	if connectionIndex < 0 || listed[connectionIndex].OAuthProviderID == nil {
		return nil, errors.New("the GitHub App's connection has no OAuth provider")
	}
	found.providerID = *listed[connectionIndex].OAuthProviderID
	if found.scopes, err = agentAuth.GitHubAppScopes(ctx, found.providerID); err != nil {
		return nil, err
	}
	if found.installed, err = agentAuth.ConnectionHasAgentCredential(ctx, app.ConnectionID, deploymentID); err != nil {
		return nil, err
	}
	return found, nil
}

// appHolder is the deployment whose Agent Auth app holds the GitHub App
// appID, with that app: the deployment named by the connection on the
// provider Agent Auth registered for the App, once it is confirmed to own an
// app with that App ID. false when no deployment's GitHub channel holds it.
func appHolder(ctx context.Context, agentAuth *hostapi.Client, appID string) (string, hostapi.AgentAuthApp, bool, error) {
	providers, err := agentAuth.GitHubAppProviderIDs(ctx, appID)
	if err != nil || len(providers) == 0 {
		return "", hostapi.AgentAuthApp{}, false, err
	}
	connections, err := agentAuth.ListConnectionsRaw(ctx)
	if err != nil {
		return "", hostapi.AgentAuthApp{}, false, err
	}
	for _, connection := range connections {
		if connection.OAuthProviderID == nil || !slices.Contains(providers, *connection.OAuthProviderID) {
			continue
		}
		deploymentID, ok := githubSlugs.deploymentOf(connection.Slug)
		if !ok {
			continue
		}
		apps, err := agentAuth.AgentApps(ctx, deploymentID)
		if err != nil {
			return "", hostapi.AgentAuthApp{}, false, err
		}
		for _, app := range apps {
			if app.Provider == githubProvider && app.AppID == appID {
				return deploymentID, app, true, nil
			}
		}
	}
	return "", hostapi.AgentAuthApp{}, false, nil
}

// appTaken reports whether Agent Auth refused the App because another of its
// apps holds it.
func appTaken(err error) bool {
	var api *hostapi.APIError
	return errors.As(err, &api) && api.Status == http.StatusConflict
}

func runGitHubSetup(ctx context.Context, env githubSetupEnv, channel *githubChannel) error {
	if len(channel.handled) == 0 {
		return env.info(githubNoHandlers)
	}
	connection, err := env.appConnection(ctx)
	if err != nil {
		return err
	}
	if connection == nil || !connection.installed {
		// The deployment is already live: a failed step offers to try again
		// or to continue without GitHub. Without a terminal nobody can
		// choose, so the error ends the deploy.
		setUp, ok, err := retryOrSkip(ctx, env, connection, githubSkippedSetup,
			func(current *githubAppConnection) (*githubAppConnection, error) {
				return setUpGitHubApp(ctx, env, channel, current)
			},
			// GitHub may have created the App before the step failed.
			func() (*githubAppConnection, error) { return env.appConnection(ctx) })
		if err != nil || !ok {
			return err
		}
		connection = setUp
	}

	status, ok, err := syncOnce(ctx, env)
	if err != nil || !ok {
		return err
	}
	if err := claimWebhook(ctx, env, connection, status); err != nil {
		return err
	}
	if len(status.Installations) == 0 && env.interactive() {
		if status, err = waitForInstallation(ctx, env, status); err != nil {
			return err
		}
	}

	report := buildGitHubReport(channel, status)
	if report.ready != "" {
		if err := env.success(report.ready); err != nil {
			return err
		}
	}
	for _, warning := range report.warnings {
		if err := env.warning(warning); err != nil {
			return err
		}
	}
	for _, note := range report.notes {
		if err := env.info(note); err != nil {
			return err
		}
	}
	if err := storeAppValues(ctx, env, connection, status); err != nil {
		return err
	}
	return syncTokenScopes(ctx, env, channel, connection, status)
}

// storeAppValues: the runtime reads the App's webhook secret and bot ID from
// Agent Auth, so nobody puts them in `.env`. Agent Auth stores the webhook
// secret with the App and records its bot ID; deploy stores the bot ID where
// the runtime reads it. Never fails the deploy.
func storeAppValues(ctx context.Context, env githubSetupEnv, connection *githubAppConnection, status *hostapi.GitHubChannelStatus) error {
	if !connection.webhookSecretStored && connection.state == hostapi.GitHubAppStateExternal {
		stored, err := storeWebhookSecret(ctx, env)
		var warning string
		switch {
		case err != nil && errs.IsCancellation(err):
			warning = "Skipped adding the GitHub App's webhook secret. The channel rejects its events until you run `langsmith deploy` again and add it."
		case err != nil:
			warning = fmt.Sprintf("Could not store the GitHub App's webhook secret (%v). Run `langsmith deploy` again to add it.", err)
		case !stored:
			warning = missingWebhookSecret
		}
		if warning != "" {
			if err := env.warning(warning); err != nil {
				return err
			}
		}
	}
	if !connection.botIDStored {
		var stored error
		if status.App.BotID != 0 {
			stored = env.storeAppValue(ctx, botIDSuffix, strconv.FormatUint(status.App.BotID, 10))
		} else {
			stored = errors.New("GitHub did not report the App's bot account")
		}
		if stored != nil {
			return env.warning(fmt.Sprintf(
				"Could not store the GitHub App's bot ID (%v), so the channel cannot handle its events yet. Run `langsmith deploy` again.", stored))
		}
	}
	return nil
}

// storeWebhookSecret asks for a brought App's webhook secret and stores it.
// false when it cannot be asked for.
func storeWebhookSecret(ctx context.Context, env githubSetupEnv) (bool, error) {
	secret, ok, err := env.webhookSecret(ctx)
	if err != nil || !ok {
		return false, err
	}
	if err := env.storeAppValue(ctx, webhookSecretSuffix, secret); err != nil {
		return false, err
	}
	return true, nil
}

// activeInstallations are the installations GitHub has not suspended.
func activeInstallations(status *hostapi.GitHubChannelStatus) []hostapi.GitHubInstallation {
	var active []hostapi.GitHubInstallation
	for _, installation := range status.Installations {
		if installation.SuspendedAt == "" {
			active = append(active, installation)
		}
	}
	return active
}

// syncTokenScopes reconciles the agent's token with the channel's
// permissions. A change that drops a permission or lowers its level never
// leaves the agent with the broader token: it is narrowed now, or the agent
// has no token until a later deploy can mint the narrower one.
func syncTokenScopes(ctx context.Context, env githubSetupEnv, channel *githubChannel, connection *githubAppConnection, status *hostapi.GitHubChannelStatus) error {
	active := activeInstallations(status)
	// Without an installation there is no token to describe or renew.
	if len(active) == 0 {
		return nil
	}
	scopes := permissionScopes(channel)
	if slices.Equal(connection.scopes, scopes) {
		return env.info(tokenSummary(channel, status, active[0], scopes, false))
	}
	narrowing := narrows(connection.scopes, scopes)
	if len(active) > 1 {
		if narrowing {
			return dropToken(ctx, env, connection, scopes,
				"The GitHub App is installed more than once, so the agent's GitHub token cannot be renewed with fewer permissions")
		}
		return env.info("The GitHub App is installed more than once, so the agent's GitHub token keeps its current permissions.")
	}
	installation := active[0]
	target := scopes
	if !grants(channel.permissions, installation.Permissions) {
		if !narrowing {
			return env.info(tokenSummary(channel, status, installation, connection.scopes, false))
		}
		// Narrow now to what the installation grants; the rest follows once
		// it grants them.
		target = grantedScopes(channel.permissions, installation.Permissions)
		if len(target) == 0 {
			return dropToken(ctx, env, connection, scopes, "The installation grants none of the permissions above")
		}
	}
	failure := renewToken(ctx, env, connection, target, installation.ID)
	switch {
	case failure == nil:
		return env.info(tokenSummary(channel, status, installation, target, true))
	case failure.tokenKept && narrowing:
		return env.warning(fmt.Sprintf(
			"Could not update the agent's GitHub token (%v), so it still has its previous, broader permissions. Run `langsmith deploy` again to retry.", failure.err))
	case failure.tokenKept:
		return env.warning(fmt.Sprintf(
			"Could not update the agent's GitHub token (%v), so it keeps its current permissions. Run `langsmith deploy` again to retry.", failure.err))
	default:
		return env.warning(fmt.Sprintf(
			"Could not update the agent's GitHub token (%v), and the agent has no GitHub token until it is renewed. Run `langsmith deploy` again to set it up.", failure.err))
	}
}

// dropToken sets the narrower scopes without minting a token, which drops
// the agent's current one: no token beats a broader one.
func dropToken(ctx context.Context, env githubSetupEnv, connection *githubAppConnection, scopes []string, reason string) error {
	if err := env.updateTokenScopes(ctx, connection.providerID, scopes); err != nil {
		return env.warning(fmt.Sprintf(
			"Could not update the agent's GitHub token (%v), so it still has its previous, broader permissions. Run `langsmith deploy` again to retry.", err))
	}
	return env.warning(reason + ", and it may not keep its broader ones, so the agent has no GitHub token for now. Run `langsmith deploy` again once that is fixed.")
}

type renewalFailure struct {
	err error
	// tokenKept reports whether the agent still has a working token.
	tokenKept bool
}

// renewToken changes the token's scopes and mints it again. When minting
// fails, the old scopes come back only if they are not broader than the new
// ones; a narrowing leaves the agent without a token instead.
func renewToken(ctx context.Context, env githubSetupEnv, connection *githubAppConnection, scopes []string, installationID uint64) *renewalFailure {
	if err := env.updateTokenScopes(ctx, connection.providerID, scopes); err != nil {
		return &renewalFailure{err: err, tokenKept: true}
	}
	err := mintAgentToken(ctx, env, connection, installationID)
	if err == nil {
		return nil
	}
	if narrows(connection.scopes, scopes) {
		return &renewalFailure{err: err, tokenKept: false}
	}
	restored := env.updateTokenScopes(ctx, connection.providerID, connection.scopes)
	if restored == nil {
		restored = mintAgentToken(ctx, env, connection, installationID)
	}
	return &renewalFailure{err: err, tokenKept: restored == nil}
}

// mintAgentToken gives the agent a token on the installation with the
// provider's scopes.
func mintAgentToken(ctx context.Context, env githubSetupEnv, connection *githubAppConnection, installationID uint64) error {
	start := hostapi.GitHubAppSessionStart{
		Kind:           hostapi.GitHubAppSessionStartInstallation,
		InstallationID: strconv.FormatUint(installationID, 10),
	}
	session, err := env.startSession(ctx, connection, start)
	if err != nil {
		return err
	}
	if session.Status != "completed" {
		return errors.New("Agent Auth did not renew the token")
	}
	return nil
}

// setUpGitHubApp gets the agent a credential on the App's connection,
// creating the connection first when there is none. An abandoned setup
// resumes on the connection it left behind.
func setUpGitHubApp(ctx context.Context, env githubSetupEnv, channel *githubChannel, connection *githubAppConnection) (*githubAppConnection, error) {
	// The owner is chosen before anything is created, so a missing owner
	// leaves no App behind.
	var current githubAppConnection
	var created *hostapi.OAuthAuthorizationSession
	if connection != nil {
		current = *connection
	} else {
		source, err := chooseGitHubAppSource(ctx, env)
		if err != nil {
			return nil, err
		}
		switch source {
		case githubAppNew:
			owner, err := env.chooseOwner(ctx)
			if err != nil {
				return nil, err
			}
			app, session, err := env.createApp(ctx, channel, owner)
			if err != nil {
				return nil, err
			}
			current, created = app, &session
		default:
			// Asked with the App's ID, key and webhook secret.
			app, err := env.storeExistingApp(ctx)
			if err != nil {
				return nil, err
			}
			current = app
		}
	}
	var session hostapi.OAuthAuthorizationSession
	var err error
	switch {
	case created != nil:
		session = *created
	case current.state == hostapi.GitHubAppStatePending:
		owner, ownerErr := env.chooseOwner(ctx)
		if ownerErr != nil {
			return nil, ownerErr
		}
		session, err = startGitHubSession(ctx, env, &current, hostapi.GitHubAppSessionStart{
			Kind:         hostapi.GitHubAppSessionStartCreate,
			Organization: owner.organization,
		})
	case current.state == hostapi.GitHubAppStateCreated:
		session, err = startGitHubSession(ctx, env, &current, hostapi.GitHubAppSessionStart{Kind: hostapi.GitHubAppSessionStartInstall})
	default:
		session, err = broughtAppSession(ctx, env, &current)
	}
	if err != nil {
		return nil, err
	}
	if session.Status != "completed" {
		if err := env.openSetupPage(session.VerificationURL, current.state == hostapi.GitHubAppStatePending); err != nil {
			return nil, err
		}
		if err := followSetupSession(ctx, env, session); err != nil {
			return nil, err
		}
	}
	if err := env.success("The GitHub App is set up for this agent."); err != nil {
		return nil, err
	}
	// GitHub created the App, or the agent now holds its credential.
	refreshed, err := env.appConnection(ctx)
	if err != nil {
		return nil, err
	}
	if refreshed == nil {
		return &current, nil
	}
	return refreshed, nil
}

// chooseGitHubAppSource is where a deployment without an App gets one: what
// the env says, else asked in a terminal. An existing App needs its key,
// which only a prompt or the env gives, so without either deploy creates a
// new one.
func chooseGitHubAppSource(ctx context.Context, env githubSetupEnv) (githubAppSource, error) {
	if source, ok := env.appSourceFromEnv(); ok {
		return source, nil
	}
	if !env.interactive() {
		return githubAppNew, nil
	}
	return env.chooseAppSource(ctx)
}

// followSetupSession follows a setup session until GitHub has created (when
// needed) and installed the App.
func followSetupSession(ctx context.Context, env githubSetupEnv, session hostapi.OAuthAuthorizationSession) error {
	for {
		switch session.Status {
		case "pending":
			next, err := env.nextSession(ctx, session.ID)
			if err != nil {
				return err
			}
			session = next
		case "completed":
			return nil
		case "failed":
			// Agent Auth lists every App it could not store on the app, so
			// their pages can be named.
			var stranded []string
			if session.ErrorCode == "app_store_failed" {
				if connection, err := env.appConnection(ctx); err == nil && connection != nil {
					stranded = connection.app.StrandedURLs()
				}
			}
			return errors.New(setupFailureMessage(session.ErrorCode, stranded))
		case "expired":
			return errors.New(githubSetupExpired)
		default:
			return fmt.Errorf("GitHub App setup returned an unexpected status: %s.", session.Status)
		}
	}
}

// setupFailureMessage is why a setup session failed, from its `error_code`
// ("" for none). stranded are the pages of the Apps GitHub created that Agent
// Auth could not store, newest last.
func setupFailureMessage(code string, stranded []string) string {
	switch code {
	case "app_creation_denied":
		return "GitHub did not create the App. Create it on the GitHub page, or choose another account."
	case "conversion_failed":
		return "GitHub could not hand over the new App; its one-time code may have expired. If GitHub shows the App, delete it there before trying again."
	case "connection_failed":
		return "Agent Auth could not look up the GitHub App it was setting up, so it did not finish creating it. If GitHub shows the App, delete it there before trying again."
	case "app_store_failed":
		const message = "The App was created on GitHub, but Agent Auth could not store it. Delete it on GitHub before trying again."
		switch len(stranded) {
		case 0:
			return message
		case 1:
			return message + "\n" + stranded[0]
		default:
			return message + " Earlier attempts left Apps on GitHub too; delete them all (newest last):\n" + strings.Join(stranded, "\n")
		}
	case "":
		return "GitHub App setup failed. If GitHub created the App, delete it on GitHub before trying again."
	default:
		return fmt.Sprintf("GitHub App setup failed (%s).", code)
	}
}

func startGitHubSession(ctx context.Context, env githubSetupEnv, connection *githubAppConnection, start hostapi.GitHubAppSessionStart) (hostapi.OAuthAuthorizationSession, error) {
	session, err := env.startSession(ctx, connection, start)
	if err != nil {
		return hostapi.OAuthAuthorizationSession{}, explainRefusedToken(err)
	}
	return session, nil
}

// installationStart records the installation id.
func installationStart(id string) hostapi.GitHubAppSessionStart {
	return hostapi.GitHubAppSessionStart{Kind: hostapi.GitHubAppSessionStartInstallation, InstallationID: id}
}

// broughtAppSession is the session that gives the agent a credential on an
// App the user brought. GitHub does not tell Agent Auth about installs made
// outside its sessions, so it records the one active installation GitHub
// reports, or one the user picks. With none, the user installs the App
// through an install session. The user names the installation only when
// GitHub cannot be asked.
func broughtAppSession(ctx context.Context, env githubSetupEnv, connection *githubAppConnection) (hostapi.OAuthAuthorizationSession, error) {
	if installation := env.installationFromEnv(); installation != "" {
		return startGitHubSession(ctx, env, connection, installationStart(installation))
	}
	outcome, err := syncWithRetry(ctx, env)
	if err != nil {
		return hostapi.OAuthAuthorizationSession{}, err
	}
	var installation string
	if outcome.Kind == hostapi.GitHubSyncOutcomeSynced {
		active := activeInstallations(outcome.Status)
		switch {
		case len(active) == 1:
			installation = strconv.FormatUint(active[0].ID, 10)
		case len(active) == 0 && env.interactive():
			return installBroughtApp(ctx, env, connection)
		case len(active) > 1 && env.interactive():
			installation, err = env.chooseInstallation(ctx, active)
		default:
			installation, err = env.askInstallationID(ctx)
		}
	} else {
		installation, err = env.askInstallationID(ctx)
	}
	if err != nil {
		return hostapi.OAuthAuthorizationSession{}, err
	}
	return startGitHubSession(ctx, env, connection, installationStart(installation))
}

// installBroughtApp installs a brought App through an install session, so
// GitHub's redirect completes it in Agent Auth. An App whose setup URL does
// not lead back to Agent Auth installs without completing the session, so
// GitHub's report is watched too, and whichever shows the installation first
// wins.
func installBroughtApp(ctx context.Context, env githubSetupEnv, connection *githubAppConnection) (hostapi.OAuthAuthorizationSession, error) {
	session, err := startGitHubSession(ctx, env, connection, hostapi.GitHubAppSessionStart{Kind: hostapi.GitHubAppSessionStartInstall})
	if err != nil || session.Status == "completed" {
		return session, err
	}
	if err := env.openInstallPage(session.VerificationURL); err != nil {
		return hostapi.OAuthAuthorizationSession{}, err
	}
	reportedRequest := false
	for {
		// Short polls, so setup goes on soon after the click even when
		// GitHub's redirect never reaches Agent Auth.
		if env.wait(ctx, installPollInterval) == waitInterrupted {
			return hostapi.OAuthAuthorizationSession{}, context.Canceled
		}
		status, err := env.sessionStatus(ctx, session.ID)
		if err != nil {
			return hostapi.OAuthAuthorizationSession{}, err
		}
		switch status {
		case "pending":
		case "completed":
			session.Status = "completed"
			return session, nil
		case "failed":
			return hostapi.OAuthAuthorizationSession{}, errors.New(setupFailureMessage("", nil))
		case "expired":
			return hostapi.OAuthAuthorizationSession{}, errors.New(githubSetupExpired)
		default:
			return hostapi.OAuthAuthorizationSession{}, fmt.Errorf("GitHub App setup returned an unexpected status: %s.", status)
		}
		outcome := env.sync(ctx)
		if outcome.Kind != hostapi.GitHubSyncOutcomeSynced {
			continue
		}
		var installation string
		switch active := activeInstallations(outcome.Status); len(active) {
		case 0:
			if requests := outcome.Status.InstallationRequests; !reportedRequest && len(requests) > 0 {
				if err := env.info(fmt.Sprintf("Waiting for an owner of %s to approve the installation. Press Ctrl-C to stop waiting.", requests[0].Account.Login)); err != nil {
					return hostapi.OAuthAuthorizationSession{}, err
				}
				reportedRequest = true
			}
			continue
		case 1:
			installation = strconv.FormatUint(active[0].ID, 10)
		default:
			if installation, err = env.chooseInstallation(ctx, active); err != nil {
				return hostapi.OAuthAuthorizationSession{}, err
			}
		}
		return startGitHubSession(ctx, env, connection, installationStart(installation))
	}
}

// syncWithRetry syncs, retrying while a freshly deployed Agent Server warms
// up.
func syncWithRetry(ctx context.Context, env githubSetupEnv) (hostapi.GitHubSyncOutcome, error) {
	outcome := env.sync(ctx)
	for attempt := 1; attempt < syncAttempts; attempt++ {
		if outcome.Kind != hostapi.GitHubSyncOutcomeUnavailable {
			break
		}
		if attempt == 1 {
			if err := env.info("Retrying the GitHub App check…"); err != nil {
				return hostapi.GitHubSyncOutcome{}, err
			}
		}
		if env.wait(ctx, syncRetryDelay) == waitInterrupted {
			break
		}
		outcome = env.sync(ctx)
	}
	return outcome, nil
}

// syncOnce is one sync (with retries). false means the deploy should finish
// without a GitHub report.
func syncOnce(ctx context.Context, env githubSetupEnv) (*hostapi.GitHubChannelStatus, bool, error) {
	outcome, err := syncWithRetry(ctx, env)
	if err != nil {
		return nil, false, err
	}
	var warning string
	switch outcome.Kind {
	case hostapi.GitHubSyncOutcomeSynced:
		return outcome.Status, true, nil
	case hostapi.GitHubSyncOutcomeMissingCredentials:
		warning = "Agent Auth has no GitHub App for this agent. Check its connection with `mda connections list`, then run `langsmith deploy` again."
	case hostapi.GitHubSyncOutcomeInvalidCredentials:
		warning = "GitHub rejected the stored App credentials. Delete the deployment with `mda delete` to remove its App from Agent Auth, then deploy again to set up a new one."
	case hostapi.GitHubSyncOutcomeNotCreated:
		warning = "GitHub has not created this agent's App yet. Run `langsmith deploy` again to finish setting it up."
	default:
		warning = fmt.Sprintf("The deployment is live, but the GitHub App could not be checked (%s). Run `langsmith deploy` again to retry.", outcome.Reason)
	}
	return nil, false, env.warning(warning)
}

func waitForInstallation(ctx context.Context, env githubSetupEnv, status *hostapi.GitHubChannelStatus) (*hostapi.GitHubChannelStatus, error) {
	if err := env.openInstallPage(githubInstallURL(status.App.Slug)); err != nil {
		return nil, err
	}
	var waited time.Duration
	reportedRequest := false
	for waited < installWaitLimit {
		if env.wait(ctx, installPollInterval) == waitInterrupted {
			return status, env.warning("Skipped waiting for the GitHub App to be installed.")
		}
		waited += installPollInterval
		outcome := env.sync(ctx)
		if outcome.Kind != hostapi.GitHubSyncOutcomeSynced {
			continue
		}
		next := outcome.Status
		if len(next.Installations) > 0 {
			return next, nil
		}
		if requests := next.InstallationRequests; !reportedRequest && len(requests) > 0 {
			if err := env.info(fmt.Sprintf("Waiting for an owner of %s to approve the installation. Press Ctrl-C to stop waiting.", requests[0].Account.Login)); err != nil {
				return nil, err
			}
			reportedRequest = true
		}
		status = next
	}
	return status, env.warning("Stopped waiting for the GitHub App to be installed.")
}

// permissionScopes are the GitHub App permissions as `permission:level`
// scopes, sorted.
func permissionScopes(channel *githubChannel) []string {
	scopes := []string{}
	for _, name := range sortedKeys(channel.permissions) {
		scopes = append(scopes, name+":"+channel.permissions[name].String())
	}
	return scopes
}

func githubAppManifest(channel *githubChannel, agentServerURL, webhookURL string) hostapi.GitHubAppManifest {
	permissions := map[string]string{}
	for name, level := range channel.permissions {
		permissions[name] = level.String()
	}
	return hostapi.GitHubAppManifest{
		Name:        channel.appName,
		URL:         agentServerURL,
		WebhookURL:  webhookURL,
		Permissions: permissions,
		Events:      channel.events,
	}
}

// githubLiveSetup is GitHub setup in a terminal, against Agent Auth.
type githubLiveSetup struct {
	terminalUI
	// agentAuth is the client for Agent Auth calls; see [agentAuth].
	agentAuth      *hostapi.Client
	deploymentID   string
	agentServerURL string
	webhook        string
	projectRoot    string
	isInteractive  bool
	// appID is the App's ID in Agent Auth once known, so reports need no
	// lookup.
	appID string
	// webhookSecretOverride is `MDA_GITHUB_WEBHOOK_SECRET` from the shell or
	// `.env`, for a brought App.
	webhookSecretOverride string
	// ownerOverride is `MDA_GITHUB_APP_OWNER` from the shell or `.env`.
	ownerOverride *appOwner
	// existingOverride is `MDA_GITHUB_APP_ID` and
	// `MDA_GITHUB_APP_PRIVATE_KEY_FILE`, with the webhook secret, from the
	// shell or `.env`.
	existingOverride *existingAppEnv
	// installationOverride is `MDA_GITHUB_APP_INSTALLATION_ID` from the
	// shell or `.env`.
	installationOverride string
	// webhookElsewhereURL is `MDA_GITHUB_WEBHOOK_ELSEWHERE` from the shell or
	// `.env`.
	webhookElsewhereURL string
}

// track is the App Agent Auth just stored, as setup tracks it. Its
// connection and provider are looked up once setup needs them.
func (s *githubLiveSetup) track(app hostapi.AgentAuthApp, webhookSecretStored bool) githubAppConnection {
	s.appID = app.ID
	return githubAppConnection{
		app:                 app,
		state:               app.State(),
		scopes:              []string{},
		webhookSecretStored: webhookSecretStored,
		keptWebhooks:        map[string]bool{},
	}
}

// releaseApp: GitHub sends an App's events to one webhook, so the App serves
// one deployment at a time. When another deployment holds it, offer to move
// it here by deleting only that deployment's Agent Auth app for it: its link
// to the App, with the key, token and webhook secret stored for it.
func (s *githubLiveSetup) releaseApp(ctx context.Context, appID string) error {
	deploymentID, app, found, err := appHolder(ctx, s.agentAuth, appID)
	if err != nil {
		return fmt.Errorf("failed to look up which deployment holds the GitHub App: %w", err)
	}
	if !found || deploymentID == s.deploymentID {
		return fmt.Errorf("GitHub App %s is already connected to another Agent Auth app in this "+
			"workspace. Use another App, or remove that app's connection first.", appID)
	}
	// Agent Auth uninstalls the App when it lets go of it, so the
	// installation is made again once the App is here.
	moved, err := ui.Confirm(fmt.Sprintf("GitHub App %s is connected to deployment %s. Move it to this "+
		"deployment? That deployment's GitHub channel stops receiving events, and the App is "+
		"uninstalled on GitHub, so you install it again next.", appID, deploymentID)).
		InitialValue(true).
		Interact(ctx)
	if err != nil {
		return fmt.Errorf("failed to confirm moving the GitHub App: %w", err)
	}
	if !moved {
		return fmt.Errorf("GitHub App %s stays connected to deployment %s. Use another "+
			"App for this deployment.", appID, deploymentID)
	}
	if _, err := s.agentAuth.DeleteAgentApp(ctx, app.ID); err != nil {
		return fmt.Errorf("failed to release GitHub App %s from deployment %s: %w", appID, deploymentID, err)
	}
	return ui.LogInfo(fmt.Sprintf("Released GitHub App %s from deployment %s.", appID, deploymentID))
}

// askExistingApp asks for an existing App's ID, private key and webhook
// secret.
func (s *githubLiveSetup) askExistingApp(ctx context.Context) (string, hostapi.PrivateKeyPem, string, error) {
	appID, err := ui.Input("GitHub App ID").
		Placeholder("shown under About on the App's settings page").
		Validate(func(value string) error { _, err := parseGitHubID(value, githubAppIDHint); return err }).
		Interact(ctx)
	if err != nil {
		return "", hostapi.PrivateKeyPem{}, "", fmt.Errorf("failed to read the GitHub App ID: %w", err)
	}
	path, err := ui.Input("Path to the App's private key (.pem)").
		Validate(func(value string) error { _, err := readPrivateKey(strings.TrimSpace(value)); return err }).
		Interact(ctx)
	if err != nil {
		return "", hostapi.PrivateKeyPem{}, "", fmt.Errorf("failed to read the private key path: %w", err)
	}
	id, err := parseGitHubID(appID, githubAppIDHint)
	if err != nil {
		return "", hostapi.PrivateKeyPem{}, "", err
	}
	key, err := readPrivateKey(strings.TrimSpace(path))
	if err != nil {
		return "", hostapi.PrivateKeyPem{}, "", err
	}
	webhookSecret, ok, err := s.webhookSecret(ctx)
	if err != nil {
		return "", hostapi.PrivateKeyPem{}, "", err
	}
	if !ok {
		return "", hostapi.PrivateKeyPem{}, "", errors.New("The GitHub App's webhook secret is required.")
	}
	return strconv.FormatUint(id, 10), key, webhookSecret, nil
}

// ownerChoice is an answer to where a new App is created.
type ownerChoice int

const (
	ownerSuggested ownerChoice = iota
	ownerOtherOrganization
	ownerPersonal
)

// askOwner asks where to register the new App. No GitHub lookup: the user
// says whether it belongs to an organization, and GitHub's own page reports
// an org that does not exist or that the user cannot create Apps in.
func (s *githubLiveSetup) askOwner(ctx context.Context) (appOwner, error) {
	suggested, hasSuggestion := githubOwnerFromRemote(ctx, s.projectRoot)
	prompt := ui.Select[ownerChoice]("Where should the GitHub App be created?")
	if hasSuggestion {
		prompt = prompt.Item(ownerSuggested, "Organization "+suggested, "from this repository")
	}
	choice, err := prompt.
		Item(ownerOtherOrganization, "Another organization…", "").
		Item(ownerPersonal, "My personal GitHub account", "the account signed in to GitHub in your browser").
		Interact(ctx)
	if err != nil {
		return appOwner{}, fmt.Errorf("failed to choose where to create the GitHub App: %w", err)
	}
	switch {
	case choice == ownerSuggested && hasSuggestion:
		return appOwner{organization: suggested}, nil
	case choice == ownerPersonal:
		return appOwner{}, nil
	}
	org, err := ui.Input("Organization name").
		Placeholder("e.g. acme").
		Validate(func(value string) error { return hostapi.ValidateLogin(strings.TrimSpace(value)) }).
		Interact(ctx)
	if err != nil {
		return appOwner{}, fmt.Errorf("failed to read the organization name: %w", err)
	}
	return appOwner{organization: strings.TrimSpace(org)}, nil
}

func (s *githubLiveSetup) interactive() bool {
	return s.isInteractive
}

func (s *githubLiveSetup) retryOrSkip(ctx context.Context, err error) (bool, error) {
	return askRetryOrSkip(ctx, "GitHub App setup", "GitHub", err)
}

func (s *githubLiveSetup) appConnection(ctx context.Context) (*githubAppConnection, error) {
	connection, err := lookupGitHubAppConnection(ctx, s.agentAuth, s.deploymentID)
	if err != nil {
		return nil, err
	}
	s.appID = ""
	if connection != nil {
		s.appID = connection.app.ID
	}
	return connection, nil
}

func (s *githubLiveSetup) chooseAppSource(ctx context.Context) (githubAppSource, error) {
	source, err := ui.Select[githubAppSource]("This agent's GitHub channel needs a GitHub App").
		Item(githubAppNew, "Create a new GitHub App", "recommended").
		Item(githubAppExisting, "Use an existing GitHub App", "enter its App ID and private key file").
		Interact(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to choose a GitHub App: %w", err)
	}
	return source, nil
}

func (s *githubLiveSetup) createApp(ctx context.Context, channel *githubChannel, owner appOwner) (githubAppConnection, hostapi.OAuthAuthorizationSession, error) {
	manifest := githubAppManifest(channel, s.agentServerURL, s.webhook)
	created, err := s.agentAuth.CreateGitHubApp(ctx, s.deploymentID, githubSlugs.connection(s.deploymentID), manifest, owner.organization)
	if err != nil {
		return githubAppConnection{}, hostapi.OAuthAuthorizationSession{}, err
	}
	// Agent Auth stores the webhook secret once GitHub creates the App.
	return s.track(created.App, true), created.Session, nil
}

func (s *githubLiveSetup) storeExistingApp(ctx context.Context) (githubAppConnection, error) {
	var appID, webhookSecret string
	var key hostapi.PrivateKeyPem
	switch {
	case s.existingOverride != nil:
		if err := ui.LogInfo(envAppNotice("GitHub App", "Agent Auth", []string{GitHubAppIDEnv, GitHubPrivateKeyFileEnv, GitHubWebhookSecretEnv})); err != nil {
			return githubAppConnection{}, err
		}
		appID, key, webhookSecret = s.existingOverride.appID, s.existingOverride.privateKey, s.existingOverride.webhookSecret
	case !s.isInteractive:
		return githubAppConnection{}, errors.New(githubOwnerRequired)
	default:
		var err error
		if appID, key, webhookSecret, err = s.askExistingApp(ctx); err != nil {
			return githubAppConnection{}, err
		}
	}
	// The token starts with the App's permissions; the scope sync after setup
	// narrows it to the channel's once the installation grants them.
	slug := githubSlugs.connection(s.deploymentID)
	existing := hostapi.ExistingApp{AppID: appID, PrivateKey: key.Expose(), WebhookSecret: webhookSecret}
	app, err := s.agentAuth.BringGitHubApp(ctx, s.deploymentID, slug, existing)
	if err != nil && appTaken(err) {
		if err := s.releaseApp(ctx, appID); err != nil {
			return githubAppConnection{}, err
		}
		app, err = s.agentAuth.BringGitHubApp(ctx, s.deploymentID, slug, existing)
	}
	if err != nil {
		return githubAppConnection{}, err
	}
	return s.track(app, true), nil
}

func (s *githubLiveSetup) appSourceFromEnv() (githubAppSource, bool) {
	switch {
	case s.existingOverride != nil:
		return githubAppExisting, true
	case s.ownerOverride != nil:
		return githubAppNew, true
	default:
		return 0, false
	}
}

func (s *githubLiveSetup) webhookSecret(ctx context.Context) (string, bool, error) {
	if s.webhookSecretOverride != "" {
		if err := ui.LogInfo(fmt.Sprintf("Using the GitHub App's webhook secret from %s. Agent Auth keeps it, so you can remove %s afterwards.",
			GitHubWebhookSecretEnv, GitHubWebhookSecretEnv)); err != nil {
			return "", false, err
		}
		return s.webhookSecretOverride, true, nil
	}
	if !s.isInteractive {
		return "", false, nil
	}
	secret, err := ui.Password("The App's webhook secret (under Webhook on its settings page)").
		Mask('•').
		Validate(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New("Enter the webhook secret. If the App has none, set one on its settings page first.")
			}
			return nil
		}).
		Interact(ctx)
	if err != nil {
		return "", false, fmt.Errorf("failed to read the webhook secret: %w", err)
	}
	return strings.TrimSpace(secret), true, nil
}

func (s *githubLiveSetup) storeAppValue(ctx context.Context, suffix, value string) error {
	_, err := s.agentAuth.CreateWorkspaceConnection(ctx, githubSlugs.secret(s.deploymentID, suffix),
		hostapi.ConnectionOwnerPolicyAgent, s.deploymentID, value)
	return err
}

func (s *githubLiveSetup) chooseOwner(ctx context.Context) (appOwner, error) {
	if s.ownerOverride != nil {
		return *s.ownerOverride, nil
	}
	if !s.isInteractive {
		return appOwner{}, errors.New(githubOwnerRequired)
	}
	return s.askOwner(ctx)
}

func (s *githubLiveSetup) installationFromEnv() string {
	return s.installationOverride
}

func (s *githubLiveSetup) askInstallationID(ctx context.Context) (string, error) {
	if s.installationOverride != "" {
		return s.installationOverride, nil
	}
	if !s.isInteractive {
		return "", errors.New(existingAppNeedsTerminal)
	}
	id, err := ui.Input("GitHub App installation ID").
		Placeholder("the number at the end of the installation's settings URL").
		Validate(func(value string) error { _, err := parseGitHubID(value, githubInstallationIDHint); return err }).
		Interact(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read the installation ID: %w", err)
	}
	parsed, err := parseGitHubID(id, githubInstallationIDHint)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(parsed, 10), nil
}

func (s *githubLiveSetup) chooseInstallation(ctx context.Context, installations []hostapi.GitHubInstallation) (string, error) {
	prompt := ui.Select[uint64]("The GitHub App is installed in several accounts. Which one should this agent use?")
	for _, installation := range installations {
		kind := "personal account"
		if installation.Account.IsOrganization() {
			kind = "organization"
		}
		prompt = prompt.Item(installation.ID, installation.Account.Login, kind)
	}
	id, err := prompt.Interact(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to choose the GitHub App installation: %w", err)
	}
	return strconv.FormatUint(id, 10), nil
}

func (s *githubLiveSetup) startSession(ctx context.Context, connection *githubAppConnection, start hostapi.GitHubAppSessionStart) (hostapi.OAuthAuthorizationSession, error) {
	return s.agentAuth.StartGitHubAppSession(ctx, connection.app, s.deploymentID, start)
}

func (s *githubLiveSetup) openSetupPage(url string, creating bool) error {
	action := "Install the GitHub App"
	if creating {
		action = "Create and install the GitHub App"
	}
	if !s.isInteractive {
		// Whoever runs deploy, e.g. a coding agent, hands this link to the
		// person who can create the App on GitHub.
		return ui.LogStep(fmt.Sprintf("%s for this agent: open this link in a browser signed in to GitHub. Deploy waits up to an hour for it to finish.\n%s", action, url))
	}
	if err := ui.LogStep(fmt.Sprintf("%s in your browser. If it does not open, visit:\n%s\nPress Ctrl-C to skip.", action, url)); err != nil {
		return err
	}
	hostapi.OpenBrowser(url)
	return nil
}

func (s *githubLiveSetup) nextSession(ctx context.Context, sessionID string) (hostapi.OAuthAuthorizationSession, error) {
	return connection.WaitForOAuthSession(ctx, s.agentAuth, sessionID)
}

func (s *githubLiveSetup) sessionStatus(ctx context.Context, sessionID string) (string, error) {
	session, err := s.agentAuth.GetOAuthAuthorizationSession(ctx, sessionID, 0)
	if err != nil {
		return "", fmt.Errorf("failed to check the GitHub App installation: %w", err)
	}
	return session.Status, nil
}

func (s *githubLiveSetup) updateTokenScopes(ctx context.Context, providerID string, scopes []string) error {
	return s.agentAuth.UpdateGitHubAppScopes(ctx, providerID, scopes)
}

func (s *githubLiveSetup) sync(ctx context.Context) hostapi.GitHubSyncOutcome {
	if s.appID == "" {
		return hostapi.GitHubSyncOutcome{Kind: hostapi.GitHubSyncOutcomeMissingCredentials}
	}
	return s.agentAuth.GitHubAppReport(ctx, s.appID)
}

func (s *githubLiveSetup) webhookURL() string {
	return s.webhook
}

func (s *githubLiveSetup) confirmWebhookReplace(ctx context.Context, current string) (bool, error) {
	replace, err := ui.Confirm(fmt.Sprintf("This GitHub App sends its events to %s. Point its webhook at this deployment instead? Whatever receives them now stops getting them.", current)).
		InitialValue(false).
		Interact(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to confirm the GitHub App webhook: %w", err)
	}
	return replace, nil
}

func (s *githubLiveSetup) pointWebhook(ctx context.Context) error {
	if s.appID == "" {
		return errors.New("the GitHub App is not set up yet")
	}
	return s.agentAuth.PointGitHubAppWebhook(ctx, s.appID, s.webhook)
}

func (s *githubLiveSetup) webhookElsewhereOverride() string {
	return s.webhookElsewhereURL
}

func (s *githubLiveSetup) forgetKeptWebhooks(ctx context.Context) error {
	prefix := githubSlugs.secret(s.deploymentID, webhookElsewhereSuffix)
	listed, err := s.agentAuth.ListConnectionsRaw(ctx)
	if err != nil {
		return err
	}
	for _, connection := range listed {
		if strings.HasPrefix(connection.Slug, prefix) {
			if _, err := s.agentAuth.DeleteConnection(ctx, connection.ConnectionID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *githubLiveSetup) openInstallPage(url string) error {
	if err := ui.LogStep(fmt.Sprintf("Install the GitHub App on your repositories. If your browser does not open, visit:\n%s\nSetup continues here once GitHub reports the installation, so you can close the page GitHub opens after it. Press Ctrl-C to skip.", url)); err != nil {
		return err
	}
	hostapi.OpenBrowser(url)
	return nil
}

func (s *githubLiveSetup) wait(ctx context.Context, duration time.Duration) waitTick {
	return wait(ctx, duration)
}

// parseOwner reads `MDA_GITHUB_APP_OWNER`: an organization login, or `@me`.
func parseOwner(value string) (appOwner, error) {
	value = strings.TrimSpace(value)
	if value == githubPersonalOwner {
		return appOwner{}, nil
	}
	if err := hostapi.ValidateLogin(value); err != nil {
		return appOwner{}, fmt.Errorf("%s must be a GitHub organization name or %s.: %w", GitHubAppOwnerEnv, githubPersonalOwner, err)
	}
	return appOwner{organization: value}, nil
}

// parseGitHubID reads a GitHub App or installation ID; hint says where to
// find it.
func parseGitHubID(value, hint string) (uint64, error) {
	value = strings.TrimSpace(value)
	digits := value != "" && strings.IndexFunc(value, func(c rune) bool { return c < '0' || c > '9' }) < 0
	id, err := strconv.ParseUint(value, 10, 64)
	if !digits || err != nil || id == 0 {
		return 0, errors.New(hint)
	}
	return id, nil
}

func readPrivateKey(path string) (hostapi.PrivateKeyPem, error) {
	value, err := ui.ValueFromFile(path)
	if err != nil {
		return hostapi.PrivateKeyPem{}, err
	}
	return hostapi.ParsePrivateKeyPem(value)
}

// githubOwnerFromRemote is the owner of the project's `origin` remote when it
// is hosted on github.com.
func githubOwnerFromRemote(ctx context.Context, root string) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", "config", "--get", "remote.origin.url")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return parseGitHubOwner(strings.TrimSpace(string(output)))
}

func parseGitHubOwner(remote string) (string, bool) {
	var path string
	found := false
	for _, prefix := range []string{"git@github.com:", "ssh://git@github.com/", "https://github.com/", "http://github.com/"} {
		if rest, ok := strings.CutPrefix(remote, prefix); ok {
			path, found = rest, true
			break
		}
	}
	if !found {
		return "", false
	}
	owner, _, _ := strings.Cut(path, "/")
	if hostapi.ValidateLogin(owner) != nil {
		return "", false
	}
	return owner, true
}
