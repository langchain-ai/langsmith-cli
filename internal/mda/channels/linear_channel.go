package channels

// Linear channel setup during `langsmith deploy`.
//
// A Linear channel needs a Linear OAuth app installed with `actor=app` in the
// workspace, its webhook pointed at the deployment. Linear has no public API
// to create apps, so a workspace admin creates it from a page deploy fills
// in, and pastes back its client ID, client secret and webhook signing
// secret. Deploy brings them into Agent Auth as an app the deployment owns,
// which keeps:
//
//   - the app's OAuth connection, `mda/linear-app/{deployment}`, whose token
//     the runtime reads as `mda/linear-token` and Agent Auth renews;
//   - the signing secret next to it, at `{slug}/webhook-secret`, which the
//     runtime verifies deliveries with.
//
// Deleting the app takes both with it, and needs only the right to update
// the deployment. Deployments set up by an earlier mda hold the same two
// slugs as plain workspace connections, which only a workspace admin can
// delete: setup keeps using them, and says so when they have to go.
//
// The CLI never reads the app's token. It goes by what Agent Auth records
// about the agent's grant: none, active, or lapsed and waiting for consent
// again, which only needs the admin to approve the install once more.
//
// Setup runs once the deployment is live, because the app's webhook points at
// the Agent Server URL. `mda delete` removes the Agent Auth app; the app
// itself stays in Linear.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/mda/connection"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// linearTokenURL is Linear's token endpoint, which deploy asks whether a
// client ID and secret belong together. Agent Auth builds the app's OAuth
// provider itself. A variable so tests can stand in for Linear.
var linearTokenURL = "https://api.linear.app/oauth/token"

const (
	// linearCreateAppPath is where Linear's apps are created, scoped to a
	// workspace when deploy knows it.
	linearCreateAppPath = "settings/api/applications/new"
	// linearAppsPath is where a workspace's apps are listed, each with its
	// settings page.
	linearAppsPath = "settings/api"
)

// CLI-only settings that replace Linear setup's prompts, with or without a
// terminal; never uploaded.
const (
	LinearClientIDEnv      = "MDA_LINEAR_CLIENT_ID"
	LinearClientSecretEnv  = "MDA_LINEAR_CLIENT_SECRET"
	LinearWebhookSecretEnv = "MDA_LINEAR_WEBHOOK_SECRET"
	LinearWorkspaceEnv     = "MDA_LINEAR_WORKSPACE"
)

// linearSigningSecretPrefix starts every Linear webhook signing secret.
const linearSigningSecretPrefix = "lin_wh_"

// linearSlugs are the slug of a deployment's Agent Auth app, which names its
// OAuth connection, and of the signing secret stored next to it
// ([webhookSecretSuffix]).
var linearSlugs = appSlugs{prefix: "mda/linear-app/"}

var linearInstallMessages = connection.OAuthWaitMessages{
	Failed:     "The Linear app install did not complete. Check that a Linear workspace admin approved it and that the app's Redirect URIs include Agent Auth's callback, then run `langsmith deploy` again.",
	Expired:    "The Linear app install was not approved in time; run `langsmith deploy` again.",
	Unexpected: "The Linear app install returned an unexpected status",
}

var linearConsentMessages = connection.OAuthWaitMessages{
	Failed:     "Approving the Linear app again did not complete. Check that a Linear workspace admin approved it, then run `langsmith deploy` again. If the app was deleted in Linear, `mda delete` removes the deployment with its Linear app in Agent Auth; deploy again to set up a new app.",
	Expired:    "The Linear app was not approved again in time; run `langsmith deploy` again.",
	Unexpected: "Approving the Linear app again returned an unexpected status",
}

const linearNoEvents = "The Linear channel handles no events, so no Linear app is set up. Add handlers with `channels.linear({ onEvent: { ... } })`, or turn agent sessions on, and deploy again."

const lapsedGrant = "Linear stopped renewing this deployment's Linear app access, so a Linear workspace admin has to approve the app again. The app and its connection stay as they are."

const revokedGrant = "This deployment's access to the Linear app was revoked, so a Linear workspace admin has to approve the app again. The app and its connection stay as they are."

// unknownGrantWarning: an Agent Auth grant status this CLI does not know.
// Setup keeps the app as it is and says so, rather than treating the grant as
// healthy unremarked.
func unknownGrantWarning(status string) string {
	return fmt.Sprintf("Agent Auth reports this deployment's Linear app access as `%s`, a status this version of langsmith does not know. Keeping the app as it is; if Linear events do not arrive, have a Linear workspace admin approve the app again, or update langsmith.", status)
}

const linearMissingSecret = "The Linear app is installed, but its webhook signing secret is not stored. Set MDA_LINEAR_WEBHOOK_SECRET to the app's signing secret (lin_wh_…, on the app's settings page in Linear) and run `langsmith deploy` again."

const linearSkippedSetup = "Continuing without the Linear channel. The deployment is live, but it receives no Linear events until you run `langsmith deploy` again and finish setting up the Linear app."

// legacyConnectionsMessage is why the plain workspace connections an earlier
// mda set up must go, and who can delete them.
func legacyConnectionsMessage(deploymentID string) string {
	connection := linearSlugs.connection(deploymentID)
	secret := linearSlugs.secret(deploymentID, webhookSecretSuffix)
	return fmt.Sprintf("An earlier version of mda stored this deployment's Linear app as the workspace connections %s and %s, and setting the app up again means deleting them first, which only a LangSmith workspace admin can do. Ask one to run `mda connections delete %s` and `mda connections delete %s`, or `langsmith deploy` once themselves, then run `langsmith deploy` again.",
		connection, secret, secret, connection)
}

// linearAppConnection is the deployment's Linear app in Agent Auth.
type linearAppConnection struct {
	// connectionID is the app's OAuth connection, which the agent's grant is
	// on.
	connectionID string
	store        linearAppStore
	// grant is the agent's grant on it: whether the app is installed, and
	// whether it needs consent again.
	grant hostapi.AgentGrant
}

// linearAppStore is how Agent Auth holds the deployment's Linear app: an
// Agent Auth app the deployment owns (appID set), which keeps the signing
// secret next to its connection and takes both when deleted; or plain
// workspace connections, as an earlier mda set them up, which only a
// workspace admin can delete.
type linearAppStore struct {
	// appID is the Agent Auth app's ID; "" for workspace connections.
	appID string
	// webhookSecretID is the webhook secret's connection of workspace
	// connections, when stored.
	webhookSecretID string
}

// hasWebhookSecret reports whether Agent Auth has the app's webhook signing
// secret. An app always has it: deploy stores the app with it.
func (c linearAppConnection) hasWebhookSecret() bool {
	return c.store.appID != "" || c.store.webhookSecretID != ""
}

// linearAppSource is where the Linear app comes from when the deployment has
// none.
type linearAppSource int

const (
	// linearAppNew is one an admin creates from the page deploy fills in.
	linearAppNew linearAppSource = iota
	// linearAppExisting is one the workspace already has, pointed at this
	// deployment.
	linearAppExisting
)

// linearAppCredentials is what a Linear admin copies from a new app's
// settings page. Formatting it prints only the client ID.
type linearAppCredentials struct {
	clientID      string
	clientSecret  string
	webhookSecret string
}

// String names the app without its secrets.
func (c linearAppCredentials) String() string {
	return fmt.Sprintf("AppCredentials { client_id: %q, .. }", c.clientID)
}

// GoString is [linearAppCredentials.String].
func (c linearAppCredentials) GoString() string {
	return c.String()
}

// linearSetupEnv is everything setup does that touches the terminal, the
// network or Agent Auth, so the flow itself is testable.
type linearSetupEnv interface {
	setupUI
	// appConnection is the deployment's app connection, nil when none.
	appConnection(ctx context.Context) (*linearAppConnection, error)
	// credentialsFromEnv is the new app's credentials from the CLI-only env
	// vars, when all are set.
	credentialsFromEnv() (*linearAppCredentials, error)
	// webhookSecretFromEnv is the signing secret alone from
	// [LinearWebhookSecretEnv], for an installed app that is missing only
	// that; "" when unset.
	webhookSecretFromEnv() (string, error)
	// workspaceKey is the workspace's URL key, from [LinearWorkspaceEnv] or
	// asked for in a terminal; without either, "".
	workspaceKey(ctx context.Context) (string, error)
	// configuredWorkspaceKey is the workspace's URL key from
	// [LinearWorkspaceEnv] alone, never asked for; "" when unset.
	configuredWorkspaceKey() string
	// chooseAppSource asks whether to create a new app or use one the
	// workspace has.
	chooseAppSource(ctx context.Context) (linearAppSource, error)
	// showExistingAppSettings shows what an existing app must be set to.
	showExistingAppSettings(settings string) error
	// openCreatePage shows and opens the page that creates the app, filled
	// in.
	openCreatePage(url string) error
	// askCredentials asks for what the new app's settings page shows.
	askCredentials(ctx context.Context) (linearAppCredentials, error)
	// askWebhookSecret asks for the app's webhook signing secret alone.
	askWebhookSecret(ctx context.Context) (string, error)
	// createApp brings the app into Agent Auth as one the deployment owns,
	// with its signing secret, and returns it with its connection.
	createApp(ctx context.Context, channel *linearChannel, credentials linearAppCredentials) (linearAppConnection, error)
	// storeWebhookSecret stores the signing secret next to an earlier mda's
	// app connection.
	storeWebhookSecret(ctx context.Context, secret string) error
	// startInstall starts the install, or the consent again, that a
	// workspace admin approves.
	startInstall(ctx context.Context, connectionID string) (hostapi.OAuthAuthorizationSession, error)
	// confirmRedirectURI asks whether the app lists the redirect URI Agent
	// Auth sends Linear.
	confirmRedirectURI(ctx context.Context, actual string) (bool, error)
	// confirmAppSettings asks whether an app from the env is set as deploy
	// listed.
	confirmAppSettings(ctx context.Context) (bool, error)
	// openInstallPage shows and opens the consent page.
	openInstallPage(url string) error
	// nextInstallStatus waits for the install to change and returns its
	// status.
	nextInstallStatus(ctx context.Context, sessionID string) (string, error)
	// deleteApp removes the deployment's Agent Auth app with everything it
	// holds.
	deleteApp(ctx context.Context, appID string) error
	// deleteConnection removes one of an earlier mda's workspace
	// connections.
	deleteConnection(ctx context.Context, connectionID string) error
	// deploymentID is the deployment the app belongs to.
	deploymentID() string
	// webhookURL is where this deployment receives the app's events.
	webhookURL() string
	// redirectURI is Agent Auth's OAuth callback, which a new app lists as
	// its redirect URI.
	redirectURI() string
}

// setUpLinear runs Linear app setup against a live deployment.
func setUpLinear(ctx context.Context, client *hostapi.Client, channel *linearChannel, agentServerURL, deploymentID string, envVars map[string]string) error {
	webhook, err := webhookURL(agentServerURL, channel.eventsPath)
	if err != nil {
		return err
	}
	agentAuthClient, err := agentAuth(client, envVars)
	if err != nil {
		return err
	}
	redirectURI, err := oauthCallbackURL(ctx, agentAuthClient, envVars)
	if err != nil {
		return err
	}
	env := &linearLiveSetup{
		agentAuth:     agentAuthClient,
		deployment:    deploymentID,
		webhook:       webhook,
		redirect:      redirectURI,
		isInteractive: isInteractive(),
		envVars:       envVars,
	}
	return runLinearSetup(ctx, env, channel)
}

// preflightLinear checks, before anything is deployed, that setup will be
// able to finish. Malformed or incomplete CLI-only env vars always fail
// here. Without a terminal, a new app's credentials must come from the env.
// deploymentID is the existing deployment, "" for none.
func preflightLinear(ctx context.Context, client *hostapi.Client, channel *linearChannel, deploymentID string, envVars map[string]string) error {
	if !channelNeedsApp(channel) {
		return nil
	}
	if key, ok := overrideValue(LinearWorkspaceEnv, envVars); ok {
		if _, err := workspaceKey(key); err != nil {
			return fmt.Errorf("%s is not valid: %w", LinearWorkspaceEnv, err)
		}
	}
	if _, err := linearWebhookSecretEnv(envVars); err != nil {
		return err
	}
	// A wrong client ID or secret fails here, before anything is uploaded or
	// stored in Agent Auth.
	if credentials, err := linearCredentialsEnv(envVars); err == nil && credentials != nil {
		if err := checkCredentials(ctx, *credentials); err != nil {
			return err
		}
	}
	// A new app's credentials cover every case. Without a terminal, a partial
	// set fails below only once it is known that a new app is needed: the
	// signing secret alone repairs an installed app.
	if isInteractive() {
		return interactiveCredentialsEnv(envVars)
	}
	credentials, credentialsErr := linearCredentialsEnv(envVars)
	if credentialsErr == nil && credentials != nil {
		return nil
	}
	if deploymentID != "" {
		agentAuthClient, err := agentAuth(client, envVars)
		if err != nil {
			return err
		}
		connection, err := lookupLinearAppConnection(ctx, agentAuthClient, deploymentID)
		if err != nil {
			return fmt.Errorf("failed to check the Linear app before deploying: %w", err)
		}
		// An installed app is kept, also one waiting for consent again, which
		// deploy prints the link for. It may need its signing secret alone.
		if connection != nil && connection.grant.Kind != hostapi.AgentGrantMissing {
			secret, err := linearWebhookSecretEnv(envVars)
			if err != nil {
				return err
			}
			if !connection.hasWebhookSecret() && secret == "" {
				return errors.New(linearMissingSecret)
			}
			return nil
		}
	}
	if credentialsErr != nil {
		return credentialsErr
	}
	_, workspaceKnown := overrideValue(LinearWorkspaceEnv, envVars)
	return errors.New(unattendedLinearSetupMessage("", workspaceKnown))
}

// credentialsCheckTimeout is how long the credential check may take before
// deploy goes on without it.
const credentialsCheckTimeout = 10 * time.Second

// checkCredentials asks whether Linear knows the app's client ID and secret.
// Linear has no API for that, but its token endpoint checks the client
// before the code, so a code that cannot exist gets an answer naming a wrong
// client or secret. Only such an answer fails: anything else, such as a
// network error or an answer Linear may change, says nothing, and deploy goes
// on.
func checkCredentials(ctx context.Context, credentials linearAppCredentials) error {
	ctx, cancel := context.WithTimeout(ctx, credentialsCheckTimeout)
	defer cancel()
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"mda-deploy-credentials-check"},
		"client_id":     {credentials.clientID},
		"client_secret": {credentials.clientSecret},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, linearTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil
	}
	body, readErr := io.ReadAll(response.Body)
	if closeErr := response.Body.Close(); readErr == nil {
		readErr = closeErr
	}
	if readErr != nil {
		body = nil
	}
	return credentialsVerdict(response.StatusCode, string(body))
}

// credentialsVerdict is what Linear's token endpoint says about a client ID
// and secret.
func credentialsVerdict(status int, body string) error {
	if status != http.StatusBadRequest && status != http.StatusUnauthorized {
		return nil
	}
	var answer struct {
		Error *string `json:"error"`
	}
	if json.Unmarshal([]byte(body), &answer) != nil || answer.Error == nil {
		return nil
	}
	switch *answer.Error {
	case "invalid_secret":
		return fmt.Errorf("Linear refused %s: it is not the client secret of the app %s names. Copy it from the app's settings page and run `langsmith deploy` again.", LinearClientSecretEnv, LinearClientIDEnv)
	case "invalid_client":
		return fmt.Errorf("Linear knows no app with %s and %s. Copy both from the app's settings page and run `langsmith deploy` again.", LinearClientIDEnv, LinearClientSecretEnv)
	default:
		return nil
	}
}

func channelNeedsApp(channel *linearChannel) bool {
	return len(channel.handled) > 0
}

// unattendedLinearSetupMessage says what to set without a terminal.
// createURL is the page that creates the app, "" when not known.
func unattendedLinearSetupMessage(createURL string, workspaceKnown bool) string {
	page := ""
	if createURL != "" {
		page = " at " + createURL
	}
	// Linear opens an unscoped link in the workspace the person last used,
	// where they may not be an admin.
	workspace := ""
	if !workspaceKnown {
		workspace = fmt.Sprintf(" Set %s to the workspace's URL key, as in https://linear.app/<key>/, so the link opens in that workspace.", LinearWorkspaceEnv)
	}
	return fmt.Sprintf("This agent's Linear channel needs a Linear app, and without a terminal deploy cannot ask for it. Have an admin of the Linear workspace create one%s, or use an app the workspace has, then set %s, %s and %s in the shell or .env and run `langsmith deploy` again. Deploy prints the install link to approve.%s",
		page, LinearClientIDEnv, LinearClientSecretEnv, LinearWebhookSecretEnv, workspace)
}

// linearAppsPage is where a workspace's apps are listed: its page when the
// workspace key is known ("" for none), else where it is in Linear.
func linearAppsPage(workspace string) string {
	if workspace == "" {
		return "Settings → Administration → API in Linear"
	}
	return fmt.Sprintf("https://linear.app/%s/%s", workspace, linearAppsPath)
}

func runLinearSetup(ctx context.Context, env linearSetupEnv, channel *linearChannel) error {
	if !channelNeedsApp(channel) {
		return env.info(linearNoEvents)
	}
	current, err := env.appConnection(ctx)
	if err != nil {
		return err
	}
	kept, setUp, err := retryOrSkip(ctx, env, current, linearSkippedSetup,
		func(current *linearAppConnection) (bool, error) { return setUpLinearApp(ctx, env, channel, current) },
		func() (*linearAppConnection, error) { return env.appConnection(ctx) })
	if err != nil || !setUp {
		return err
	}
	if err := env.success("Linear channel ready · the Linear app sends its events to " + env.webhookURL()); err != nil {
		return err
	}
	if kept {
		return nil
	}
	// Said once, when the app is installed, rather than on every deploy.
	return env.info(fmt.Sprintf("Linear has no API to change an app, so later deploys keep it as it is now. If the channel's app name, description or icon change, or this deployment moves to another URL, change them on the app's settings page (%s).",
		linearAppsPage(env.configuredWorkspaceKey())))
}

// setUpLinearApp installs the app, or finishes what an earlier setup left
// undone. Returns whether an app created earlier is kept.
func setUpLinearApp(ctx context.Context, env linearSetupEnv, channel *linearChannel, current *linearAppConnection) (bool, error) {
	if current == nil {
		return false, installNewApp(ctx, env, channel)
	}
	switch current.grant.Kind {
	case hostapi.AgentGrantActive:
		if err := storeMissingSecret(ctx, env, current); err != nil {
			return false, err
		}
	// The grant lapsed or was revoked: the app and its secret stay, and the
	// admin approves it again.
	case hostapi.AgentGrantAuthorizationRequired, hostapi.AgentGrantRevoked:
		reason := lapsedGrant
		if current.grant.Kind == hostapi.AgentGrantRevoked {
			reason = revokedGrant
		}
		if err := env.warning(reason); err != nil {
			return false, err
		}
		if err := storeMissingSecret(ctx, env, current); err != nil {
			return false, err
		}
		if err := approve(ctx, env, current.connectionID, linearConsentMessages); err != nil {
			return false, err
		}
	// A status Agent Auth added after this CLI: warn, naming it, and keep
	// going without consent, so a harmless new status does not break
	// deploys.
	case hostapi.AgentGrantUnknown:
		if err := env.warning(unknownGrantWarning(current.grant.Status)); err != nil {
			return false, err
		}
		if err := storeMissingSecret(ctx, env, current); err != nil {
			return false, err
		}
	// An install that did not finish: start again with fresh credentials.
	default:
		if err := env.info("A previous Linear app setup did not finish; starting it again."); err != nil {
			return false, err
		}
		if err := remove(ctx, env, current); err != nil {
			return false, err
		}
		return false, installNewApp(ctx, env, channel)
	}
	return true, nil
}

// storeMissingSecret stores the signing secret of an installed app, when an
// earlier setup did not store it. Only an earlier mda's connections can lack
// it.
func storeMissingSecret(ctx context.Context, env linearSetupEnv, connection *linearAppConnection) error {
	if connection.hasWebhookSecret() {
		return nil
	}
	secret, err := env.webhookSecretFromEnv()
	if err != nil {
		return err
	}
	if secret == "" {
		if !env.interactive() {
			return errors.New(linearMissingSecret)
		}
		if secret, err = env.askWebhookSecret(ctx); err != nil {
			return err
		}
	}
	return env.storeWebhookSecret(ctx, secret)
}

func installNewApp(ctx context.Context, env linearSetupEnv, channel *linearChannel) error {
	credentials, err := newAppCredentials(ctx, env, channel)
	if err != nil {
		return err
	}
	app, err := env.createApp(ctx, channel, credentials)
	if err != nil {
		return err
	}
	result := approve(ctx, env, app.connectionID, linearInstallMessages)
	if result != nil {
		// The next attempt asks for the credentials again, so leave nothing
		// behind that could hold wrong ones.
		if cleanup := remove(ctx, env, &app); cleanup != nil {
			if err := env.warning(fmt.Sprintf("Could not remove the Linear app's unfinished setup from Agent Auth (%v).", cleanup)); err != nil {
				return err
			}
		}
	}
	return result
}

// newAppCredentials are a new app's credentials: from the env, else asked
// for after showing an existing app's settings or the page that creates one.
func newAppCredentials(ctx context.Context, env linearSetupEnv, channel *linearChannel) (linearAppCredentials, error) {
	credentials, err := env.credentialsFromEnv()
	if err != nil {
		return linearAppCredentials{}, err
	}
	if credentials != nil {
		if err := confirmAppSettings(ctx, env, channel); err != nil {
			return linearAppCredentials{}, err
		}
		return *credentials, nil
	}
	if env.interactive() {
		source, err := env.chooseAppSource(ctx)
		if err != nil {
			return linearAppCredentials{}, err
		}
		if source == linearAppExisting {
			settings := linearExistingAppSettings(channel, env.webhookURL(), env.redirectURI(), env.configuredWorkspaceKey())
			if err := env.showExistingAppSettings(settings); err != nil {
				return linearAppCredentials{}, err
			}
			return env.askCredentials(ctx)
		}
	}
	workspace, err := env.workspaceKey(ctx)
	if err != nil {
		return linearAppCredentials{}, err
	}
	createURL, err := createAppURL(workspace, channel, env.webhookURL(), env.redirectURI())
	if err != nil {
		return linearAppCredentials{}, err
	}
	if !env.interactive() {
		return linearAppCredentials{}, errors.New(unattendedLinearSetupMessage(createURL, workspace != ""))
	}
	if err := env.openCreatePage(createURL); err != nil {
		return linearAppCredentials{}, err
	}
	return env.askCredentials(ctx)
}

// approve has a workspace admin approve the app on its connection: the first
// install, or consent again once the grant lapsed.
func approve(ctx context.Context, env linearSetupEnv, connectionID string, messages connection.OAuthWaitMessages) error {
	session, err := env.startInstall(ctx, connectionID)
	if err != nil {
		return err
	}
	if session.Status != "completed" {
		if actual, ok := redirectURIOf(session.VerificationURL); ok && actual != env.redirectURI() {
			if err := confirmRedirectURI(ctx, env, actual); err != nil {
				return err
			}
		}
		if err := env.openInstallPage(session.VerificationURL); err != nil {
			return err
		}
	}
	return connection.PollOAuthSession(session.Status, func() (string, error) {
		return env.nextInstallStatus(ctx, session.ID)
	}, messages)
}

// confirmAppSettings: an app from the env was made outside deploy, so it may
// not point at this deployment, and Linear has no API that says what it has
// set: list the settings, and in a terminal wait until they are set. "No"
// asks again, since Linear refuses the install without the redirect URI;
// Escape stops setting up the channel.
func confirmAppSettings(ctx context.Context, env linearSetupEnv, channel *linearChannel) error {
	settings := linearAppSettings(channel, env.webhookURL(), env.redirectURI(), env.configuredWorkspaceKey())
	if err := env.warning(settings + " Linear can't tell deploy what the app has set, so check these before the install is approved."); err != nil {
		return err
	}
	if !env.interactive() {
		return nil
	}
	for {
		confirmed, err := env.confirmAppSettings(ctx)
		if err != nil || confirmed {
			return err
		}
		if err := env.info("Linear refuses the install until the app lists the redirect URI above. Set it, then answer Yes, or press Esc to stop setting up the Linear channel."); err != nil {
			return err
		}
	}
}

// confirmRedirectURI: Agent Auth sends Linear a redirect URI the new app may
// not list: say so, and in a terminal wait until it is added. "No" asks
// again, since Linear refuses the install until then; Escape stops setting up
// the channel.
func confirmRedirectURI(ctx context.Context, env linearSetupEnv, actual string) error {
	if err := env.warning(fmt.Sprintf("Agent Auth expects Linear to redirect to %s. Add it under Redirect URIs on the Linear app's settings page before approving the install, or Linear answers \"Invalid redirect_uri\".", actual)); err != nil {
		return err
	}
	if !env.interactive() {
		return nil
	}
	for {
		confirmed, err := env.confirmRedirectURI(ctx, actual)
		if err != nil || confirmed {
			return err
		}
		if err := env.info(fmt.Sprintf("Linear refuses the install until the app lists %s under Redirect URIs. Add it, then answer Yes, or press Esc to stop setting up the Linear channel.", actual)); err != nil {
			return err
		}
	}
}

// remove deletes the deployment's app from Agent Auth. An earlier mda's
// connections need a workspace admin, so a refusal says who can delete them.
func remove(ctx context.Context, env linearSetupEnv, connection *linearAppConnection) error {
	if connection.store.appID != "" {
		return env.deleteApp(ctx, connection.store.appID)
	}
	var err error
	if connection.store.webhookSecretID != "" {
		err = env.deleteConnection(ctx, connection.store.webhookSecretID)
	}
	if err == nil {
		err = env.deleteConnection(ctx, connection.connectionID)
	}
	if err != nil && isForbidden(err) {
		return errors.New(legacyConnectionsMessage(env.deploymentID()))
	}
	return err
}

func isForbidden(err error) bool {
	var api *hostapi.APIError
	return errors.As(err, &api) && api.Status == http.StatusForbidden
}

// redirectURIOf is the redirect URI an authorize URL sends Linear's answer
// to.
func redirectURIOf(verificationURL string) (string, bool) {
	u, err := url.Parse(verificationURL)
	if err != nil {
		return "", false
	}
	values, ok := u.Query()["redirect_uri"]
	if !ok || len(values) == 0 {
		return "", false
	}
	return values[0], true
}

// createAppURL is the page that creates the app, filled in from an app
// manifest so the admin only has to create it. Linear's pages are scoped to
// the workspace ("" for none).
func createAppURL(workspace string, channel *linearChannel, webhookURL, redirectURI string) (string, error) {
	base := "https://linear.app/" + linearCreateAppPath
	if workspace != "" {
		key, err := workspaceKey(workspace)
		if err != nil {
			return "", err
		}
		base = fmt.Sprintf("https://linear.app/%s/%s", key, linearCreateAppPath)
	}
	display := map[string]any{"description": channel.appDescription}
	if channel.appIcon != "" {
		// The deployment serves the channel's icon next to its events route.
		eventsBase, ok := strings.CutSuffix(webhookURL, channel.eventsPath)
		if !ok {
			return "", errors.New("the Linear webhook URL does not end in the channel's events path")
		}
		display["iconUrl"] = fmt.Sprintf("%s/channels/%s/icon", eventsBase, channel.name)
	}
	manifest := map[string]any{
		"$schema":       "https://linear.app/.well-known/oauth-app-manifest.schema.json",
		"schemaVersion": "1.0.0",
		"distribution":  "private",
		"display":       display,
		"developer":     map[string]any{"name": "LangSmith"},
		"oauth": map[string]any{
			"client_name":   channel.appName,
			"client_uri":    "https://www.langchain.com/langsmith",
			"redirect_uris": []string{redirectURI},
			"grant_types":   []string{"authorization_code"},
		},
		"webhook": map[string]any{
			"url":           webhookURL,
			"enabled":       true,
			"resourceTypes": channel.resourceTypes,
		},
	}
	encoded, err := marshalUnescaped(manifest)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid Linear app page URL: %w", err)
	}
	u.RawQuery = "manifest=" + url.QueryEscape(encoded)
	return u.String(), nil
}

// linearExistingAppSettings is what an app the workspace already has must be
// set to, the same as a new app's page would fill in. Linear has no API to
// change an app, so an admin sets these on its settings page.
func linearExistingAppSettings(channel *linearChannel, webhookURL, redirectURI, workspace string) string {
	return linearAppSettings(channel, webhookURL, redirectURI, workspace) +
		" Then copy its client ID, client secret and webhook signing secret into the prompts below."
}

// linearAppSettings are the settings an app must have for this deployment,
// and what changing its webhook does.
func linearAppSettings(channel *linearChannel, webhookURL, redirectURI, workspace string) string {
	return fmt.Sprintf("Point your Linear app at this deployment on its settings page (%s):\n"+
		"\n  Redirect URIs: add %s\n"+
		"  Webhooks: turn on, URL %s\n"+
		"    events: %s\n"+
		"\nLinear sends an app's webhooks to one URL, so whatever received this app's "+
		"events stops receiving them.",
		linearAppsPage(workspace), redirectURI, webhookURL, strings.Join(channel.resourceTypes, ", "))
}

// workspaceKey is a workspace's URL key, from the key itself or any
// `linear.app` URL in it.
func workspaceKey(input string) (string, error) {
	input = strings.TrimSpace(input)
	key := input
	if rest, ok := strings.CutPrefix(input, "https://linear.app/"); ok {
		key, _, _ = strings.Cut(rest, "/")
	}
	key = strings.ToLower(key)
	valid := key != "" && len(key) <= 64 && strings.IndexFunc(key, func(c rune) bool {
		return !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-')
	}) < 0
	if !valid {
		return "", errors.New("Enter the workspace's URL key, as in https://linear.app/<key>/.")
	}
	return key, nil
}

// linearCredentialsEnv is the new app's credentials from the CLI-only env
// vars: all three, or none (nil).
func linearCredentialsEnv(envVars map[string]string) (*linearAppCredentials, error) {
	clientID, hasClientID := overrideValue(LinearClientIDEnv, envVars)
	clientSecret, hasClientSecret := overrideValue(LinearClientSecretEnv, envVars)
	webhookSecret, hasWebhookSecret := overrideValue(LinearWebhookSecretEnv, envVars)
	switch {
	case hasClientID && hasClientSecret && hasWebhookSecret:
		if err := validateLinearSigningSecret(webhookSecret); err != nil {
			return nil, err
		}
		return &linearAppCredentials{clientID: clientID, clientSecret: clientSecret, webhookSecret: webhookSecret}, nil
	case !hasClientID && !hasClientSecret && !hasWebhookSecret:
		return nil, nil
	default:
		return nil, fmt.Errorf("Set all of %s, %s and %s, or none of them.", LinearClientIDEnv, LinearClientSecretEnv, LinearWebhookSecretEnv)
	}
}

// interactiveCredentialsEnv: in a terminal, a partial set of a new app's
// credentials fails, but the signing secret alone doesn't: it repairs an
// installed app, and a new app's other credentials are asked for.
func interactiveCredentialsEnv(envVars map[string]string) error {
	_, hasClientID := overrideValue(LinearClientIDEnv, envVars)
	_, hasClientSecret := overrideValue(LinearClientSecretEnv, envVars)
	if hasClientID || hasClientSecret {
		_, err := linearCredentialsEnv(envVars)
		return err
	}
	return nil
}

// linearWebhookSecretEnv is the signing secret alone, for an installed app
// that is missing only that; "" when unset.
func linearWebhookSecretEnv(envVars map[string]string) (string, error) {
	secret, ok := overrideValue(LinearWebhookSecretEnv, envVars)
	if !ok {
		return "", nil
	}
	if err := validateLinearSigningSecret(secret); err != nil {
		return "", fmt.Errorf("%s is not valid: %w", LinearWebhookSecretEnv, err)
	}
	return secret, nil
}

func validateLinearSigningSecret(value string) error {
	if strings.HasPrefix(strings.TrimSpace(value), linearSigningSecretPrefix) {
		return nil
	}
	return errors.New("Enter the webhook signing secret: it starts with lin_wh_ and is shown under Webhooks on the app's page.")
}

// configuredWorkspaceKey is [LinearWorkspaceEnv] as a workspace key, when set
// and valid; "" otherwise.
func configuredWorkspaceKey(envVars map[string]string) string {
	value, ok := overrideValue(LinearWorkspaceEnv, envVars)
	if !ok {
		return ""
	}
	key, err := workspaceKey(value)
	if err != nil {
		return ""
	}
	return key
}

// lookupLinearAppConnection is the deployment's Linear app and the agent's
// grant on it: its Agent Auth app, or else the workspace connections an
// earlier mda set up; nil when neither.
func lookupLinearAppConnection(ctx context.Context, agentAuth *hostapi.Client, deploymentID string) (*linearAppConnection, error) {
	apps, _, err := agentAuth.AgentAppsIfSupported(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	var found linearAppConnection
	appFound := false
	for _, app := range apps {
		if app.Provider != linearProvider {
			continue
		}
		if app.ConnectionID == "" {
			return nil, errors.New("Agent Auth returned the deployment's Linear app without its connection")
		}
		found = linearAppConnection{connectionID: app.ConnectionID, store: linearAppStore{appID: app.ID}}
		appFound = true
		break
	}
	if !appFound {
		listed, err := agentAuth.ListConnectionsRaw(ctx)
		if err != nil {
			return nil, err
		}
		appConnection, ok := findConnection(listed, linearSlugs.connection(deploymentID))
		if !ok {
			return nil, nil
		}
		found = linearAppConnection{connectionID: appConnection.ConnectionID}
		if secret, ok := findConnection(listed, linearSlugs.secret(deploymentID, webhookSecretSuffix)); ok {
			found.store.webhookSecretID = secret.ConnectionID
		}
	}
	if found.grant, err = agentAuth.AgentOAuthGrant(ctx, found.connectionID, deploymentID); err != nil {
		return nil, err
	}
	return &found, nil
}

// linearLiveSetup is Linear setup in a terminal, against Agent Auth.
type linearLiveSetup struct {
	terminalUI
	agentAuth     *hostapi.Client
	deployment    string
	webhook       string
	redirect      string
	isInteractive bool
	envVars       map[string]string
}

func (s *linearLiveSetup) interactive() bool {
	return s.isInteractive
}

func (s *linearLiveSetup) retryOrSkip(ctx context.Context, err error) (bool, error) {
	return askRetryOrSkip(ctx, "Linear app setup", "Linear", err)
}

func (s *linearLiveSetup) appConnection(ctx context.Context) (*linearAppConnection, error) {
	found, err := lookupLinearAppConnection(ctx, s.agentAuth, s.deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to look up the Linear app in Agent Auth: %w", err)
	}
	return found, nil
}

func (s *linearLiveSetup) credentialsFromEnv() (*linearAppCredentials, error) {
	credentials, err := linearCredentialsEnv(s.envVars)
	if err != nil {
		return nil, err
	}
	if credentials != nil {
		if err := ui.LogInfo(envAppNotice("Linear app", "Agent Auth", []string{LinearClientIDEnv, LinearClientSecretEnv, LinearWebhookSecretEnv})); err != nil {
			return nil, err
		}
	}
	return credentials, nil
}

func (s *linearLiveSetup) webhookSecretFromEnv() (string, error) {
	secret, err := linearWebhookSecretEnv(s.envVars)
	if err != nil {
		return "", err
	}
	if secret != "" {
		if err := ui.LogInfo(fmt.Sprintf("Using the Linear app's webhook signing secret from %s. Agent Auth keeps it, so you can remove %s afterwards.",
			LinearWebhookSecretEnv, LinearWebhookSecretEnv)); err != nil {
			return "", err
		}
	}
	return secret, nil
}

func (s *linearLiveSetup) workspaceKey(ctx context.Context) (string, error) {
	if key, ok := overrideValue(LinearWorkspaceEnv, s.envVars); ok {
		return workspaceKey(key)
	}
	if !s.interactive() {
		return "", nil
	}
	// Required: Linear opens a link without a workspace in the one the
	// person last used, where they may not be an admin.
	input, err := ui.Input("Linear workspace for the app (its URL or key, as in https://linear.app/<key>/)").
		Placeholder("acme, or https://linear.app/acme/team/ENG/active").
		Validate(func(value string) error { _, err := workspaceKey(value); return err }).
		Interact(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read the Linear workspace: %w", err)
	}
	return workspaceKey(input)
}

func (s *linearLiveSetup) configuredWorkspaceKey() string {
	return configuredWorkspaceKey(s.envVars)
}

func (s *linearLiveSetup) chooseAppSource(ctx context.Context) (linearAppSource, error) {
	source, err := ui.Select[linearAppSource]("This agent's Linear channel needs a Linear app").
		Item(linearAppNew, "Create a new Linear app", "recommended: deploy opens Linear's create page, filled in").
		Item(linearAppExisting, "Use an existing Linear app", "enter its client ID, client secret and webhook signing secret").
		Interact(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to choose a Linear app: %w", err)
	}
	return source, nil
}

func (s *linearLiveSetup) showExistingAppSettings(settings string) error {
	return ui.LogStep(settings)
}

func (s *linearLiveSetup) openCreatePage(url string) error {
	if err := ui.LogStep(fmt.Sprintf("Create the Linear app for this agent. Only admins of the Linear workspace can open this page; anyone else sees \"Not found\". The page opens filled in, so check it and click Create. If your browser does not open, visit:\n%s\nThen copy the app's client ID, client secret and webhook signing secret into the prompts below.\n\nNot an admin? Send the link to one: they click Create and send you the three values, or set %s, %s and %s and run `langsmith deploy` themselves.",
		url, LinearClientIDEnv, LinearClientSecretEnv, LinearWebhookSecretEnv)); err != nil {
		return err
	}
	hostapi.OpenBrowser(url)
	return nil
}

func (s *linearLiveSetup) askCredentials(ctx context.Context) (linearAppCredentials, error) {
	clientID, err := ui.Input("Client ID").
		Validate(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New("Enter the client ID shown on the app's page.")
			}
			return nil
		}).
		Interact(ctx)
	if err != nil {
		return linearAppCredentials{}, fmt.Errorf("failed to read the Linear client ID: %w", err)
	}
	clientSecret, err := ui.Password("Client secret").
		Mask('•').
		Validate(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New("Enter the client secret shown on the app's page.")
			}
			return nil
		}).
		Interact(ctx)
	if err != nil {
		return linearAppCredentials{}, fmt.Errorf("failed to read the Linear client secret: %w", err)
	}
	webhookSecret, err := s.askWebhookSecret(ctx)
	if err != nil {
		return linearAppCredentials{}, err
	}
	return linearAppCredentials{
		clientID:      strings.TrimSpace(clientID),
		clientSecret:  strings.TrimSpace(clientSecret),
		webhookSecret: webhookSecret,
	}, nil
}

func (s *linearLiveSetup) askWebhookSecret(ctx context.Context) (string, error) {
	secret, err := ui.Password("Webhook signing secret").
		Mask('•').
		Validate(validateLinearSigningSecret).
		Interact(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read the Linear webhook signing secret: %w", err)
	}
	return strings.TrimSpace(secret), nil
}

func (s *linearLiveSetup) createApp(ctx context.Context, channel *linearChannel, credentials linearAppCredentials) (linearAppConnection, error) {
	app, err := s.agentAuth.BringLinearApp(ctx, s.deployment, linearSlugs.connection(s.deployment), hostapi.ExistingLinearApp{
		ClientID:      credentials.clientID,
		ClientSecret:  credentials.clientSecret,
		WebhookSecret: credentials.webhookSecret,
		Scopes:        channel.scopes,
		AllowedScopes: channel.allowedScopes,
	})
	if err != nil {
		return linearAppConnection{}, fmt.Errorf("failed to register the Linear app with Agent Auth: %w", err)
	}
	if app.ConnectionID == "" {
		return linearAppConnection{}, errors.New("Agent Auth returned the Linear app without its connection")
	}
	return linearAppConnection{
		connectionID: app.ConnectionID,
		store:        linearAppStore{appID: app.ID},
		grant:        hostapi.AgentGrant{Kind: hostapi.AgentGrantMissing},
	}, nil
}

func (s *linearLiveSetup) storeWebhookSecret(ctx context.Context, secret string) error {
	if _, err := s.agentAuth.CreateWorkspaceConnection(ctx, linearSlugs.secret(s.deployment, webhookSecretSuffix),
		hostapi.ConnectionOwnerPolicyAgent, s.deployment, secret); err != nil {
		return fmt.Errorf("failed to store the Linear webhook signing secret: %w", err)
	}
	return nil
}

func (s *linearLiveSetup) startInstall(ctx context.Context, connectionID string) (hostapi.OAuthAuthorizationSession, error) {
	session, err := s.agentAuth.CreateAgentOAuthAuthorizationSession(ctx, connectionID, s.deployment)
	if err != nil {
		return hostapi.OAuthAuthorizationSession{}, fmt.Errorf("failed to start the Linear app install: %w", err)
	}
	return session, nil
}

func (s *linearLiveSetup) confirmRedirectURI(ctx context.Context, _ string) (bool, error) {
	added, err := ui.Confirm("Added it? Deploy opens the install page next.").InitialValue(true).Interact(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to confirm the Linear app's redirect URI: %w", err)
	}
	return added, nil
}

func (s *linearLiveSetup) confirmAppSettings(ctx context.Context) (bool, error) {
	set, err := ui.Confirm("Set them? Deploy opens the install page next.").InitialValue(true).Interact(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to confirm the Linear app's settings: %w", err)
	}
	return set, nil
}

func (s *linearLiveSetup) openInstallPage(url string) error {
	if !s.isInteractive {
		// Whoever runs deploy, e.g. a coding agent, hands this link to a
		// Linear workspace admin.
		return ui.LogStep(fmt.Sprintf("Install the Linear app: a Linear workspace admin opens this link and approves it. Deploy waits for it.\n%s", url))
	}
	if err := ui.LogStep(fmt.Sprintf("Install the Linear app in your browser: a Linear workspace admin approves it. If it does not open, visit:\n%s\nPress Ctrl-C to skip.", url)); err != nil {
		return err
	}
	hostapi.OpenBrowser(url)
	return nil
}

func (s *linearLiveSetup) nextInstallStatus(ctx context.Context, sessionID string) (string, error) {
	return connection.WaitForOAuthStatus(ctx, s.agentAuth, sessionID)
}

func (s *linearLiveSetup) deleteApp(ctx context.Context, appID string) error {
	if _, err := s.agentAuth.DeleteAgentApp(ctx, appID); err != nil {
		return fmt.Errorf("failed to delete the deployment's Linear app from Agent Auth: %w", err)
	}
	return nil
}

func (s *linearLiveSetup) deleteConnection(ctx context.Context, connectionID string) error {
	_, err := s.agentAuth.DeleteConnection(ctx, connectionID)
	return err
}

func (s *linearLiveSetup) deploymentID() string {
	return s.deployment
}

func (s *linearLiveSetup) webhookURL() string {
	return s.webhook
}

func (s *linearLiveSetup) redirectURI() string {
	return s.redirect
}
