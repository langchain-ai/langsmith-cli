package channels

// The Slack channel, which Trigger Server receives events for and delivers to
// the deployment.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

const (
	slackIconImageMaxBytes      = 1_000_000
	slackIconImageSizePx        = 512
	slackAppNameMaxChars        = 35
	defaultSlackBackgroundColor = "#09090f"
	pngMinBytes                 = 33
	pngSignature                = "\x89PNG\r\n\x1a\n"
)

func pngDimensions(image []byte) (width, height uint32, ok bool) {
	if len(image) < 24 {
		return 0, 0, false
	}
	return binary.BigEndian.Uint32(image[16:20]), binary.BigEndian.Uint32(image[20:24]), true
}

func validateSlackIcon(image []byte) error {
	if len(image) > slackIconImageMaxBytes {
		return errors.New("Slack channel icon must be at most 1 MB")
	}
	if len(image) < pngMinBytes || !bytes.HasPrefix(image, []byte(pngSignature)) || string(image[12:16]) != "IHDR" {
		return errors.New("Slack channel icon must be a valid PNG")
	}
	if width, height, ok := pngDimensions(image); !ok || width != slackIconImageSizePx || height != slackIconImageSizePx {
		return errors.New("Slack channel icon must be 512x512px")
	}
	return nil
}

type resolvedSlackIcon struct {
	image           []byte
	backgroundColor string
}

// resolveSlackIcon is the authored icon, or one generated from name. An empty
// configuredBackgroundColor is none.
func resolveSlackIcon(projectRoot, name string, icon, configuredBackgroundColor *string) (resolvedSlackIcon, error) {
	if icon != nil {
		image, err := readSlackIcon(projectRoot, *icon)
		if err != nil {
			return resolvedSlackIcon{}, err
		}
		background := defaultSlackBackgroundColor
		if configuredBackgroundColor != nil {
			background = *configuredBackgroundColor
		}
		return resolvedSlackIcon{image: image, backgroundColor: background}, nil
	}
	generated, err := generateIcon(name)
	if err != nil {
		return resolvedSlackIcon{}, err
	}
	background := generated.backgroundColor
	if configuredBackgroundColor != nil {
		background = *configuredBackgroundColor
	}
	return resolvedSlackIcon{image: generated.png, backgroundColor: background}, nil
}

func readSlackIcon(projectRoot, icon string) ([]byte, error) {
	resolved, err := channelIconFile(projectRoot, icon, "Slack channel icon", slackIconImageMaxBytes)
	if err != nil {
		return nil, err
	}
	image, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read Slack channel icon %s: %w", icon, err)
	}
	if err := validateSlackIcon(image); err != nil {
		return nil, err
	}
	return image, nil
}

type channelManifestHeader struct {
	Name      string  `json:"name" strict:"required"`
	Provider  string  `json:"provider" strict:"required"`
	Transport *string `json:"transport" strict:"nullable"`
}

type slackManifest struct {
	Name                string   `json:"name" strict:"required"`
	Provider            string   `json:"provider" strict:"required"`
	Transport           string   `json:"transport" strict:"required"`
	AutoReply           bool     `json:"autoReply" strict:"required"`
	RequiredPermissions []string `json:"requiredPermissions" strict:"required"`
	RequiredEnv         []string `json:"requiredEnv" strict:"required"`
	// AgentConnections is read by the connection checks, not by setup.
	AgentConnections any                 `json:"agentConnections" strict:"nullable"`
	ProviderConfig   slackProviderConfig `json:"providerConfig" strict:"required"`
}

type slackProviderConfig struct {
	App                  slackProviderAppConfig `json:"app" strict:"required"`
	TriggerOnAllMessages bool                   `json:"triggerOnAllMessages" strict:"required"`
	// TriggerMode is only emitted by SDKs for modes older CLIs cannot
	// express.
	TriggerMode      *hostapi.SlackTriggerMode `json:"triggerMode" strict:"nullable"`
	AllowBotTriggers bool                      `json:"allowBotTriggers" strict:"required"`
}

// UnmarshalJSON decodes strictly.
func (c *slackProviderConfig) UnmarshalJSON(data []byte) error {
	type plain slackProviderConfig
	return decodeStrict(data, (*plain)(c), true)
}

type slackProviderAppConfig struct {
	Name            *string `json:"name" strict:"nullable"`
	Description     *string `json:"description" strict:"nullable"`
	Icon            *string `json:"icon" strict:"nullable"`
	BackgroundColor *string `json:"backgroundColor" strict:"nullable"`
}

// UnmarshalJSON decodes strictly.
func (c *slackProviderAppConfig) UnmarshalJSON(data []byte) error {
	type plain slackProviderAppConfig
	return decodeStrict(data, (*plain)(c), true)
}

type preparedSlackApp struct {
	name            string
	description     string
	icon            []byte
	backgroundColor string
}

// preparedTriggerChannel is the project's Slack channel, validated, with its
// app's name, description and icon resolved.
type preparedTriggerChannel struct {
	name                 string
	app                  preparedSlackApp
	triggerOnAllMessages bool
	triggerMode          *hostapi.SlackTriggerMode
	allowBotTriggers     bool
}

type setupStateKind int

const (
	setupActive setupStateKind = iota
	setupActionRequired
)

type setupState struct {
	kind   setupStateKind
	action requiredAction
}

type requiredActionKind int

const (
	actionAuthorize requiredActionKind = iota
	actionApprove
)

// requiredAction is what the user does in Slack before setup can finish. An
// authorization always has a URL; an approval may not.
type requiredAction struct {
	kind    requiredActionKind
	message string
	url     *url.URL
}

func (a requiredAction) heading() string {
	if a.kind == actionAuthorize {
		return "Connect Slack"
	}
	return "Approve Slack"
}

// instruction is what the user does with the action's link.
func (a requiredAction) instruction() string {
	if a.kind == actionAuthorize {
		return "Authorize LangSmith to set up your Slack app."
	}
	return "Install the Slack app. A workspace admin may need to approve it first."
}

func (a requiredAction) retryGuidance() string {
	action := "Approve"
	if a.kind == actionAuthorize {
		action = "Authorize"
	}
	if a.url != nil {
		return fmt.Sprintf("%s %s the Slack app at %s, then rerun `langsmith deploy`.", a.message, action, a.url)
	}
	return fmt.Sprintf("%s %s the Slack app, then rerun `langsmith deploy`.", a.message, action)
}

func prepareTriggerChannel(requirements introspect.ChannelRequirements, wait bool, projectRoot string, language project.Language, assistantID string) (*preparedTriggerChannel, error) {
	var mapName string
	var header *channelManifestHeader
	for _, name := range sortedKeys(requirements) {
		var candidate channelManifestHeader
		if err := decodeManifest(requirements[name], &candidate, false); err != nil {
			return nil, fmt.Errorf("channel requirements must include valid name, provider, and optional transport: %w", err)
		}
		if candidate.Transport == nil || *candidate.Transport != "trigger_server" {
			continue
		}
		if header != nil {
			return nil, errors.New("Deploy currently supports one channel per agent.")
		}
		mapName, header = name, &candidate
	}
	if header == nil {
		return nil, nil
	}

	if mapName != header.Name {
		return nil, errors.New("Channel name must match its manifest key.")
	}
	if header.Provider != "slack" {
		return nil, errors.New("Deploy currently supports only Slack channels.")
	}

	// Either autoReply deploys: the channel itself posts the run's answer, or
	// with `reply: "none"` leaves it to the agent; Trigger Server never does.
	var manifest slackManifest
	if err := decodeManifest(requirements[mapName], &manifest, true); err != nil {
		return nil, fmt.Errorf("Slack channel configuration is invalid.: %w", err)
	}
	if len(manifest.RequiredPermissions) > 0 {
		return nil, fmt.Errorf("Slack channel configuration requires %s=[].", language.Ident("requiredPermissions", "required_permissions"))
	}
	if len(manifest.RequiredEnv) > 0 {
		return nil, fmt.Errorf("Slack channel configuration requires %s=[].", language.Ident("requiredEnv", "required_env"))
	}
	if !wait {
		return nil, errors.New("An agent with a Slack channel cannot be deployed with --no-wait.")
	}
	config := manifest.ProviderConfig
	triggerOnAllMessages := config.TriggerOnAllMessages
	if mode := config.TriggerMode; mode != nil {
		if config.TriggerOnAllMessages != (*mode == hostapi.SlackTriggerModeAll) {
			return nil, fmt.Errorf("Slack channel configuration has a %s that conflicts with %s; set only %s.",
				language.Ident("triggerOnAllMessages", "trigger_on_all_messages"),
				language.Ident("triggerMode", "trigger_mode"),
				language.Ident("triggerMode", "trigger_mode"))
		}
		triggerOnAllMessages = *mode == hostapi.SlackTriggerModeAll
	}
	app := config.App
	appName := defaultSlackAppName(assistantID)
	if app.Name != nil {
		appName = *app.Name
	}
	icon, err := resolveSlackIcon(projectRoot, appName, app.Icon, app.BackgroundColor)
	if err != nil {
		return nil, err
	}
	description := ""
	if app.Description != nil {
		description = *app.Description
	}
	// "all" travels as the legacy flag so older Trigger Servers read it.
	triggerMode := config.TriggerMode
	if triggerMode != nil && *triggerMode == hostapi.SlackTriggerModeAll {
		triggerMode = nil
	}

	return &preparedTriggerChannel{
		name: manifest.Name,
		app: preparedSlackApp{
			name:            appName,
			description:     description,
			icon:            icon.image,
			backgroundColor: icon.backgroundColor,
		},
		triggerOnAllMessages: triggerOnAllMessages,
		triggerMode:          triggerMode,
		allowBotTriggers:     config.AllowBotTriggers,
	}, nil
}

func defaultSlackAppName(assistantID string) string {
	runes := []rune(assistantID)
	if len(runes) > slackAppNameMaxChars {
		runes = runes[:slackAppNameMaxChars]
	}
	return strings.TrimRight(string(runes), "-")
}

func endpointURL(agentServerURL, channelName string) (*url.URL, error) {
	u, err := deployment.ValidatedAgentServerURL(agentServerURL)
	if err != nil {
		return nil, err
	}
	return u.JoinPath("v1", "channels", channelName, "events"), nil
}

func buildSetupRequest(prepared *preparedTriggerChannel, agentServerURL, deploymentID, assistantID string) (hostapi.ChannelSetupRequest, error) {
	endpoint, err := endpointURL(agentServerURL, prepared.name)
	if err != nil {
		return hostapi.ChannelSetupRequest{}, err
	}
	return hostapi.ChannelSetupRequest{
		Provider: "slack",
		Target: hostapi.ChannelSetupTarget{
			DeploymentID: deploymentID,
			EndpointURL:  endpoint.String(),
			ChannelName:  prepared.name,
			AgentID:      assistantID,
		},
		Slack: slackSetup(prepared),
	}, nil
}

func slackSetup(prepared *preparedTriggerChannel) hostapi.SlackChannelSetup {
	return hostapi.SlackChannelSetup{
		App: hostapi.SlackChannelApp{
			Name:            prepared.app.name,
			Description:     prepared.app.description,
			Image:           "data:image/png;base64," + base64.StdEncoding.EncodeToString(prepared.app.icon),
			BackgroundColor: prepared.app.backgroundColor,
		},
		Options: hostapi.SlackChannelOptions{
			TriggerOnAllMessages: prepared.triggerOnAllMessages,
			TriggerMode:          prepared.triggerMode,
			AllowBotTriggers:     prepared.allowBotTriggers,
		},
	}
}

const slackSkippedSetup = "Continuing without the Slack channel. The deployment is live, but it receives no Slack events until you run `langsmith deploy` again and finish setting up the Slack app."

// preflightTriggerChannel fails before uploading when registering the channel
// could not finish. The env credentials are checked always. Without a
// terminal, a new channel needs them unless LangSmith can create the Slack
// app on its own: it cannot on a self-hosted LangSmith, and first needs the
// user to authorize it in Slack when they have not yet. deploymentID is ""
// for a new deployment.
func preflightTriggerChannel(ctx context.Context, client *hostapi.Client, prepared *preparedTriggerChannel, deploymentID string, envVars map[string]string) error {
	credentials, err := slackCredentialsEnv(envVars)
	if err != nil {
		return err
	}
	if credentials != nil || isInteractive() {
		return nil
	}
	var registered *registrationLookup
	if deploymentID != "" {
		found, err := registration(ctx, client, deploymentID, prepared.name)
		registered = &registrationLookup{registered: found, err: err}
	}
	return unattendedOutcome(registered, func() (string, bool) { return triggerUnattendedProblem(ctx, client) })
}

// triggerUnattendedProblem is why LangSmith can't create the Slack app
// without a terminal, if it can't.
func triggerUnattendedProblem(ctx context.Context, client *hostapi.Client) (string, bool) {
	selfHosted := client.IsSelfHosted()
	// Not knowing whether LangSmith can create the app leaves it to setup.
	var manager *hostapi.SlackManager
	if !selfHosted {
		if found, err := client.SlackManager(ctx); err == nil {
			manager = found
		}
	}
	return slackUnattendedProblem(selfHosted, manager)
}

// registration reports whether Trigger Server already has the deployment's
// channel, so deploy keeps its Slack app instead of asking for one; an error
// when it can't say.
func registration(ctx context.Context, client *hostapi.Client, deploymentID, channelName string) (bool, error) {
	channels, err := client.TriggerChannels(ctx, deploymentID)
	if err != nil {
		return false, err
	}
	for _, channel := range channels {
		if channel.Provider == "slack" && channel.ChannelName == channelName {
			return true, nil
		}
	}
	return false, nil
}

// registrationLookup is what [registration] answered.
type registrationLookup struct {
	registered bool
	err        error
}

// unattendedOutcome is whether a deploy without a terminal or the user's own
// Slack app can set up the channel: one Trigger Server already has keeps its
// app, and otherwise LangSmith has to be able to create one (problem says why
// it can't). A failed lookup then says so, not that the channel needs an app:
// it may well work already. registered is nil for a new deployment.
func unattendedOutcome(registered *registrationLookup, problem func() (string, bool)) error {
	if registered != nil && registered.err == nil && registered.registered {
		return nil
	}
	message, ok := problem()
	switch {
	case !ok:
		return nil
	case registered != nil && registered.err != nil:
		return lookupFailed(registered.err)
	default:
		return errors.New(message)
	}
}

func lookupFailed(err error) error {
	return fmt.Errorf("Could not check whether this agent's Slack channel is already set up (%v). Rerun `langsmith deploy` to try again, or set MDA_SLACK_BOT_TOKEN and MDA_SLACK_SIGNING_SECRET to your app's bot token and signing secret.", err)
}

// registerTriggerChannel registers the channel with Trigger Server for the
// live deployment. As for the GitHub and Linear channels, a failed attempt
// offers to try again or to continue without the channel; without a terminal
// the error ends setup.
func registerTriggerChannel(ctx context.Context, client *hostapi.Client, prepared *preparedTriggerChannel, agentServerURL, deploymentID, assistantID string, envVars map[string]string) error {
	live := slackLiveUI{isInteractive: isInteractive()}
	_, registered, err := retryOrSkip(ctx, live, struct{}{}, slackSkippedSetup,
		func(struct{}) (struct{}, error) {
			return struct{}{}, registerOnce(ctx, client, prepared, agentServerURL, deploymentID, assistantID, envVars, live.isInteractive)
		},
		func() (struct{}, error) { return struct{}{}, nil })
	if err != nil {
		return err
	}
	if registered {
		return live.success(slackReady)
	}
	return nil
}

const slackReady = "Slack channel ready · the Slack app sends its events to this deployment through LangSmith"

// slackLiveUI is what Slack setup shows in a terminal.
type slackLiveUI struct {
	terminalUI
	isInteractive bool
}

func (u slackLiveUI) interactive() bool {
	return u.isInteractive
}

func (u slackLiveUI) retryOrSkip(ctx context.Context, err error) (bool, error) {
	return askRetryOrSkip(ctx, "Slack channel setup", "Slack", err)
}

// registerOnce is one attempt at registering the channel, choosing its app
// when Trigger Server has none for it yet.
func registerOnce(ctx context.Context, client *hostapi.Client, prepared *preparedTriggerChannel, agentServerURL, deploymentID, assistantID string, envVars map[string]string, interactive bool) error {
	request, err := buildSetupRequest(prepared, agentServerURL, deploymentID, assistantID)
	if err != nil {
		return err
	}
	registered, err := registration(ctx, client, deploymentID, prepared.name)
	if err != nil {
		// Without a terminal, deploy can't ask for an app instead.
		if !interactive {
			credentials, credentialsErr := slackCredentialsEnv(envVars)
			if credentialsErr != nil {
				return credentialsErr
			}
			if _, problem := triggerUnattendedProblem(ctx, client); credentials == nil && problem {
				return lookupFailed(err)
			}
		}
		// Asking again is harmless: Trigger Server keeps a registered app.
		warnBestEffort(fmt.Sprintf("Could not check whether the Slack channel is already set up: %v", err))
		registered = false
	}
	var customerApp *hostapi.SlackCustomerApp
	if registered {
		// Trigger Server keeps the app's credentials; ones in the env must
		// match.
		if customerApp, err = slackCredentialsEnv(envVars); err != nil {
			return err
		}
	} else {
		choice := newSlackLiveChoice(client, &request.Slack, envVars, interactive)
		app, err := chooseSlackApp(ctx, choice)
		if err != nil {
			return err
		}
		customerApp = app.customer
	}
	if customerApp != nil {
		request.Slack.CustomerApp = customerApp
		return registerCustomerApp(interactive,
			func(moveFrom string) (hostapi.ChannelSetupResponse, error) {
				request.Slack.CustomerApp.MoveFromDeployment = moveFrom
				return client.SetupTriggerChannel(ctx, &request)
			},
			func(deploymentID string) (bool, error) { return confirmSlackAppMove(ctx, deploymentID) })
	}
	return runRegistration(
		func() (hostapi.ChannelSetupResponse, error) { return client.SetupTriggerChannel(ctx, &request) },
		func(action requiredAction) error { return showAction(action, interactive) },
		func(duration time.Duration) waitTick { return wait(ctx, duration) })
}

// registerCustomerApp registers the user's own app, which Trigger Server
// activates at once. When another deployment in the workspace uses the app,
// offer to move it here. setup takes the deployment to move the app from, ""
// for none.
func registerCustomerApp(interactive bool, setup func(moveFrom string) (hostapi.ChannelSetupResponse, error), confirmMove func(deploymentID string) (bool, error)) error {
	response, err := setup("")
	if err != nil {
		var inUse *hostapi.SlackAppInUse
		if !errors.As(err, &inUse) {
			return err
		}
		deploymentID := inUse.DeploymentID
		if !interactive {
			return fmt.Errorf("Slack app is connected to deployment %s and stays there. Rerun `langsmith deploy` in a terminal to move it to this deployment, or use another Slack app.", deploymentID)
		}
		move, err := confirmMove(deploymentID)
		if err != nil {
			return err
		}
		if !move {
			return fmt.Errorf("Slack app stays connected to deployment %s. Use another Slack app for this deployment.", deploymentID)
		}
		if response, err = setup(deploymentID); err != nil {
			return err
		}
		infoBestEffort(fmt.Sprintf("Moved the Slack app from deployment %s.", deploymentID))
	}
	state, err := classifySetupResponse(response)
	if err != nil {
		return err
	}
	if state.kind == setupActionRequired {
		return errors.New("Slack channel setup did not finish with your Slack app. Check its bot token and signing secret, then rerun `langsmith deploy`.")
	}
	return nil
}

func confirmSlackAppMove(ctx context.Context, deploymentID string) (bool, error) {
	move, err := ui.Confirm(fmt.Sprintf("Slack app is connected to deployment %s. Move it to this deployment? That deployment's Slack channel stops receiving events.", deploymentID)).
		InitialValue(true).
		Interact(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to confirm moving the Slack app: %w", err)
	}
	return move, nil
}

// showAction shows what the user does in Slack before setup can finish. In a
// terminal the link also opens in the browser; without one, whoever runs
// deploy (a coding agent, say) hands it to the person who can act on it.
func showAction(action requiredAction, interactive bool) error {
	heading := action.heading()
	instruction := action.instruction()
	var message string
	switch {
	case action.url != nil && interactive:
		message = fmt.Sprintf("%s\n%s If your browser does not open, visit:\n%s\nSetup continues here once Slack reports it. Press Ctrl-C to skip.", heading, instruction, action.url)
	case action.url != nil:
		message = fmt.Sprintf("%s\n%s Open this link in a browser signed in to Slack. Deploy waits up to an hour for it to finish.\n%s", heading, instruction, action.url)
	case interactive:
		message = fmt.Sprintf("%s\nWaiting for a Slack workspace admin to approve the Slack app. Press Ctrl-C to skip.", heading)
	default:
		message = fmt.Sprintf("%s\nWaiting for a Slack workspace admin to approve the Slack app. Deploy waits up to an hour for it.", heading)
	}
	if err := ui.LogStep(message); err != nil {
		return err
	}
	if action.url != nil && interactive {
		hostapi.OpenBrowser(action.url.String())
	}
	return nil
}

func classifySetupResponse(response hostapi.ChannelSetupResponse) (setupState, error) {
	invalid := errors.New("Channel setup returned an invalid response.")
	for _, id := range []string{response.ChannelID, response.SourceRegistrationID} {
		if id == "" {
			continue
		}
		if _, err := uuid.Parse(id); err != nil {
			return setupState{}, invalid
		}
	}
	var message string
	switch response.Status {
	case hostapi.ChannelSetupStatusActive:
		if response.ChannelID == "" {
			return setupState{}, invalid
		}
		return setupState{kind: setupActive}, nil
	case hostapi.ChannelSetupStatusAuthorizationRequired:
		message = "Slack channel setup requires authorization."
	default:
		message = "Slack channel setup is pending workspace approval."
	}
	if detail, ok := safeMessage(response.Message); ok {
		message += " " + detail
	}
	authorizationURL := safeAuthorizationURL(response.AuthorizationURL)
	if response.Status == hostapi.ChannelSetupStatusAuthorizationRequired {
		if authorizationURL == nil {
			return setupState{}, fmt.Errorf("%s Authorize the Slack app, then rerun `langsmith deploy`.", message)
		}
		return setupState{kind: setupActionRequired, action: requiredAction{kind: actionAuthorize, message: message, url: authorizationURL}}, nil
	}
	return setupState{kind: setupActionRequired, action: requiredAction{kind: actionApprove, message: message, url: authorizationURL}}, nil
}

const (
	// actionPollInterval is how often setup asks Trigger Server again while
	// it waits for the user. Each check retries the setup, which calls Slack,
	// so it is not as eager as the GitHub installation check.
	actionPollInterval = 5 * time.Second
	// actionWaitLimit is how long setup waits for the user, as GitHub setup
	// does.
	actionWaitLimit = 60 * 60 * time.Second
)

// runRegistration asks Trigger Server to set the channel up until it is
// active. While it waits for the user to authorize LangSmith or to install
// the app, show shows what to do (again only when that changes), and setup
// asks again after each wait. Once the user has been shown what to do, a
// request that may succeed when made again (a timeout, 5xx or 429) doesn't
// end the wait, as the GitHub installation check keeps going; any other
// error does. Ctrl-C stops waiting; so does the hour running out.
func runRegistration(setup func() (hostapi.ChannelSetupResponse, error), show func(requiredAction) error, wait func(time.Duration) waitTick) error {
	type step struct {
		heading string
		hasURL  bool
	}
	var shown *step
	var last *requiredAction
	var waited time.Duration
	for {
		response, err := setup()
		switch {
		case err == nil:
			state, err := classifySetupResponse(response)
			if err != nil {
				return err
			}
			if state.kind == setupActive {
				return nil
			}
			action := state.action
			// Every answer carries a fresh link, and the one shown still works.
			current := step{heading: action.heading(), hasURL: action.url != nil}
			if shown == nil || *shown != current {
				if err := show(action); err != nil {
					return err
				}
				shown = &current
			}
			last = &action
		case last != nil && hostapi.IsTransientSetupError(err):
		default:
			return err
		}
		if waited >= actionWaitLimit {
			guidance := ""
			if last != nil {
				guidance = last.retryGuidance()
			}
			return fmt.Errorf("Stopped waiting for Slack after an hour. %s", guidance)
		}
		if wait(actionPollInterval) == waitInterrupted {
			return context.Canceled
		}
		waited += actionPollInterval
	}
}

func safeMessage(message string) (string, bool) {
	trimmed := strings.TrimSpace(message)
	ok := trimmed != "" && utf8.RuneCountInString(trimmed) <= 256 && !hasControl(trimmed)
	return trimmed, ok
}

func safeAuthorizationURL(rawURL string) *url.URL {
	if rawURL == "" || hasControl(rawURL) {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || strings.Contains(rawURL, "#") {
		return nil
	}
	return u
}
