package channels

// Which Slack app a Slack channel uses, asked when `langsmith deploy`
// registers the channel for the first time, as the GitHub and Linear channels
// ask for theirs:
//
//   - one LangSmith creates and manages, in the workspace the user authorized
//     LangSmith in (hosted LangSmith only);
//   - a new one the user creates from the manifest Trigger Server prepares;
//   - one the user already has, pointed at Trigger Server.
//
// The user's own app's bot token and signing secret go to Trigger Server,
// which keeps them in Agent Auth, so a redeploy asks nothing.
// `MDA_SLACK_BOT_TOKEN` and `MDA_SLACK_SIGNING_SECRET` supply them instead of
// the prompts, with or without a terminal.

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// CLI-only: a Slack app's bot token and signing secret when deploy cannot ask.
const (
	SlackBotTokenEnv      = "MDA_SLACK_BOT_TOKEN"
	SlackSigningSecretEnv = "MDA_SLACK_SIGNING_SECRET"
)

const slackBotTokenPrefix = "xoxb-"

// slackUnattendedMessage is why a new Slack channel cannot get an app without
// a terminal: LangSmith cannot create one here (selfHosted, or not offered),
// or would first need the user to authorize it in Slack.
func slackUnattendedMessage(selfHosted bool) string {
	managed := ""
	if !selfHosted {
		managed = " To have LangSmith create and manage the app instead, run `langsmith deploy` in a terminal once."
	}
	return fmt.Sprintf("This agent's Slack channel needs a Slack app, and without a terminal deploy cannot ask for it. Create one, or use one you have, then set %s and %s in the shell or .env to its bot token and signing secret and run `langsmith deploy` again.%s",
		SlackBotTokenEnv, SlackSigningSecretEnv, managed)
}

// slackUnattendedProblem says whether a new Slack channel can get an app
// without a terminal and without the env credentials: only one LangSmith
// creates in a workspace the user already authorized it in. An unknown (nil)
// manager is tried.
func slackUnattendedProblem(selfHosted bool, manager *hostapi.SlackManager) (string, bool) {
	switch {
	case selfHosted:
		return slackUnattendedMessage(true), true
	case manager == nil:
		return "", false
	case manager.Kind == hostapi.SlackManagerUnavailable:
		return slackUnattendedMessage(true), true
	case manager.Kind == hostapi.SlackManagerNotAuthorized:
		return slackUnattendedMessage(false), true
	default:
		return "", false
	}
}

// slackApp is the Slack app a channel is registered with: one LangSmith
// creates and manages (nil customer), or one the user owns.
type slackApp struct {
	customer *hostapi.SlackCustomerApp
}

type slackAppSource int

const (
	slackAppManaged slackAppSource = iota
	slackAppManifest
	slackAppExisting
)

// slackAppOption is one answer to "which Slack app?".
type slackAppOption struct {
	source slackAppSource
	label  string
	hint   string
}

// slackChoiceEnv is what choosing an app needs from LangSmith and the
// terminal.
type slackChoiceEnv interface {
	interactive() bool
	selfHosted() bool
	credentialsFromEnv() (*hostapi.SlackCustomerApp, error)
	// manager is whether LangSmith can create the app, and where; nil when
	// unknown.
	manager(ctx context.Context) *hostapi.SlackManager
	choose(ctx context.Context, options []slackAppOption) (slackAppSource, error)
	info(message string) error
	manifest(ctx context.Context) (map[string]any, error)
	showCreatePage(link *url.URL) error
	showExistingSettings(settings string) error
	askCredentials(ctx context.Context) (hostapi.SlackCustomerApp, error)
}

// chooseSlackApp is which Slack app to register the channel with.
func chooseSlackApp(ctx context.Context, env slackChoiceEnv) (slackApp, error) {
	credentials, err := env.credentialsFromEnv()
	if err != nil {
		return slackApp{}, err
	}
	if credentials != nil {
		return slackApp{customer: credentials}, nil
	}
	selfHosted := env.selfHosted()
	var manager *hostapi.SlackManager
	if !selfHosted {
		manager = env.manager(ctx)
	}
	if !env.interactive() {
		if problem, ok := slackUnattendedProblem(selfHosted, manager); ok {
			return slackApp{}, errors.New(problem)
		}
		workspace := ""
		if manager != nil && manager.Kind == hostapi.SlackManagerAuthorized {
			workspace = " in " + manager.TeamName
		}
		if err := env.info(fmt.Sprintf("Without a terminal, LangSmith creates and manages this agent's Slack app%s. To use your own app instead, set %s and %s.",
			workspace, SlackBotTokenEnv, SlackSigningSecretEnv)); err != nil {
			return slackApp{}, err
		}
		return slackApp{}, nil
	}
	source, err := env.choose(ctx, slackAppOptions(selfHosted, manager))
	if err != nil {
		return slackApp{}, err
	}
	switch source {
	case slackAppManifest:
		manifest, err := env.manifest(ctx)
		if err != nil {
			return slackApp{}, err
		}
		link, err := slackManifestLink(manifest)
		if err != nil {
			return slackApp{}, err
		}
		if err := env.showCreatePage(link); err != nil {
			return slackApp{}, err
		}
	case slackAppExisting:
		manifest, err := env.manifest(ctx)
		if err != nil {
			return slackApp{}, err
		}
		settings, err := slackExistingAppSettings(manifest)
		if err != nil {
			return slackApp{}, err
		}
		if err := env.showExistingSettings(settings); err != nil {
			return slackApp{}, err
		}
	default:
		return slackApp{}, nil
	}
	customer, err := env.askCredentials(ctx)
	if err != nil {
		return slackApp{}, err
	}
	return slackApp{customer: &customer}, nil
}

// slackAppOptions are the answers deploy offers: LangSmith only creates apps
// on hosted LangSmith, and names the workspace when the user already
// authorized it in one.
func slackAppOptions(selfHosted bool, manager *hostapi.SlackManager) []slackAppOption {
	existing := slackAppOption{slackAppExisting, "Use an existing Slack app", "enter its bot token and signing secret"}
	if selfHosted || (manager != nil && manager.Kind == hostapi.SlackManagerUnavailable) {
		return []slackAppOption{
			{slackAppManifest, "Create a new Slack app", "recommended: from a manifest LangSmith prepares"},
			existing,
		}
	}
	if manager != nil && manager.Kind == hostapi.SlackManagerAuthorized {
		return []slackAppOption{
			{slackAppManaged, "Create a new Slack app in " + manager.TeamName, "recommended: LangSmith creates and manages it"},
			existing,
		}
	}
	return []slackAppOption{
		{slackAppManaged, "Let LangSmith create and manage a new Slack app", "recommended: authorize LangSmith in Slack first"},
		{slackAppManifest, "Create a new Slack app yourself", "from a manifest LangSmith prepares"},
		existing,
	}
}

// slackCredentialsEnv is the bot token and signing secret from the CLI-only
// env vars: both, or neither (nil).
func slackCredentialsEnv(envVars map[string]string) (*hostapi.SlackCustomerApp, error) {
	botToken, hasToken := overrideValue(SlackBotTokenEnv, envVars)
	signingSecret, hasSecret := overrideValue(SlackSigningSecretEnv, envVars)
	switch {
	case hasToken && hasSecret:
		botToken = strings.TrimSpace(botToken)
		signingSecret = strings.TrimSpace(signingSecret)
		if err := validateSlackBotToken(botToken); err != nil {
			return nil, fmt.Errorf("%s is not valid: %w", SlackBotTokenEnv, err)
		}
		if err := validateSlackSigningSecret(signingSecret); err != nil {
			return nil, fmt.Errorf("%s is not valid: %w", SlackSigningSecretEnv, err)
		}
		return &hostapi.SlackCustomerApp{BotToken: botToken, SigningSecret: signingSecret}, nil
	case !hasToken && !hasSecret:
		return nil, nil
	default:
		return nil, fmt.Errorf("Set both %s and %s, or neither.", SlackBotTokenEnv, SlackSigningSecretEnv)
	}
}

// slackAppsURL is Slack's link that creates an app from a manifest, and where
// the user changes an existing Slack app's settings.
const slackAppsURL = "https://api.slack.com/apps"

// slackManifestLink is Slack's link that creates an app from manifest.
func slackManifestLink(manifest map[string]any) (*url.URL, error) {
	link, err := url.Parse(slackAppsURL)
	if err != nil {
		return nil, err
	}
	encoded, err := marshalUnescaped(manifest)
	if err != nil {
		return nil, fmt.Errorf("Failed to serialize the Slack manifest.: %w", err)
	}
	link.RawQuery = "new_app=1&manifest_json=" + url.QueryEscape(encoded)
	return link, nil
}

const (
	// settingsListWidth is the width a wrapped list of events or scopes fits
	// in, after its indent.
	settingsListWidth = 72
	// settingsPad is the indent of a step's detail lines, under the step's
	// text.
	settingsPad = "     "
	// settingsIndent is the indent of a URL or list inside a step.
	settingsIndent = "       "
)

// slackExistingAppSettings is what an existing app must be set to for
// Trigger Server to receive its events, from the manifest Trigger Server
// would create the app with, as numbered steps in the order of the app's
// settings sidebar. Trigger Server cannot change an app it doesn't manage, so
// the user sets these in Slack.
func slackExistingAppSettings(manifest map[string]any) (string, error) {
	text := func(pointer ...string) (string, bool) {
		value, ok := jsonPointer(manifest, pointer...).(string)
		return value, ok && !hasControl(value)
	}
	list := func(pointer ...string) ([]string, bool) {
		items, _ := jsonPointer(manifest, pointer...).([]any)
		var listed []string
		for _, item := range items {
			if value, ok := item.(string); ok && !hasControl(value) {
				listed = append(listed, value)
			}
		}
		return listed, len(listed) > 0
	}
	incomplete := errors.New("Trigger Server returned an incomplete Slack app manifest.")
	eventsURL, ok := text("settings", "event_subscriptions", "request_url")
	if !ok {
		return "", incomplete
	}
	events, ok := list("settings", "event_subscriptions", "bot_events")
	if !ok {
		return "", incomplete
	}
	interactivityURL, ok := text("settings", "interactivity", "request_url")
	if !ok {
		return "", incomplete
	}
	scopes, ok := list("oauth_config", "scopes", "bot")
	if !ok {
		return "", incomplete
	}
	enabled := func(pointer ...string) bool {
		value, ok := jsonPointer(manifest, pointer...).(bool)
		return ok && value
	}

	steps := []string{
		"Socket Mode: turn it off if it is on. Slack sends events to a URL only without it.",
		fmt.Sprintf("Event Subscriptions: turn on Enable Events and set the Request URL to\n%s%s\n%sUnder Subscribe to bot events, add\n%s",
			settingsIndent, eventsURL, settingsPad, wrapList(events, settingsIndent)),
		fmt.Sprintf("Interactivity & Shortcuts: turn on Interactivity and set the Request URL to\n%s%s",
			settingsIndent, interactivityURL),
	}
	if enabled("features", "app_home", "messages_tab_enabled") {
		steps = append(steps, "App Home: turn on the Messages Tab and allow users to send messages from it, so people can DM the agent.")
	}
	if jsonPointer(manifest, "features", "agent_view") != nil {
		steps = append(steps, "Agents & AI Apps: turn it on, so Slack shows the agent's progress and a Stop button.")
	}
	steps = append(steps, fmt.Sprintf("OAuth & Permissions: under Bot Token Scopes, add\n%s\n%sThen click Reinstall to Workspace at the top of the page.",
		wrapList(scopes, settingsIndent), settingsPad))

	var message strings.Builder
	fmt.Fprintf(&message, "Set up your Slack app at %s: pick the app, then open each page below from its sidebar.\n", slackAppsURL)
	for number, step := range steps {
		fmt.Fprintf(&message, "\n  %d. %s", number+1, step)
	}
	message.WriteString("\n\nThen copy the app's Bot User OAuth Token (OAuth & Permissions, starts with xoxb-) " +
		"and its Signing Secret (Basic Information, under App Credentials) into the prompts " +
		"below. Slack sends an app's events to one URL, so anything else that receives this " +
		"app's events now stops receiving them.")
	return message.String(), nil
}

// jsonPointer is the value at the object keys path of a JSON value, nil when
// there is none.
func jsonPointer(value any, path ...string) any {
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = object[key]
	}
	return value
}

func hasControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}

// wrapList is items joined by commas, wrapped to lines that start with
// indent.
func wrapList(items []string, indent string) string {
	lines := []string{""}
	for index, item := range items {
		if index+1 < len(items) {
			item += ","
		}
		line := &lines[len(lines)-1]
		if *line != "" && len(*line)+1+len(item) > settingsListWidth {
			lines = append(lines, item)
			continue
		}
		if *line != "" {
			*line += " "
		}
		*line += item
	}
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func validateSlackBotToken(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("enter the app's Bot User OAuth Token, under OAuth & Permissions")
	}
	if !strings.HasPrefix(value, slackBotTokenPrefix) || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return errors.New("a bot token starts with xoxb-")
	}
	return nil
}

func validateSlackSigningSecret(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return errors.New("enter the app's Signing Secret, under Basic Information")
	}
	return nil
}

// slackLiveChoice is the choice as `langsmith deploy` asks it.
type slackLiveChoice struct {
	client         *hostapi.Client
	slack          *hostapi.SlackChannelSetup
	envVars        map[string]string
	isInteractive  bool
	openBrowserURL func(string)
}

func newSlackLiveChoice(client *hostapi.Client, slack *hostapi.SlackChannelSetup, envVars map[string]string, interactive bool) *slackLiveChoice {
	return &slackLiveChoice{
		client:         client,
		slack:          slack,
		envVars:        envVars,
		isInteractive:  interactive,
		openBrowserURL: hostapi.OpenBrowser,
	}
}

func (c *slackLiveChoice) interactive() bool {
	return c.isInteractive
}

func (c *slackLiveChoice) selfHosted() bool {
	return c.client.IsSelfHosted()
}

func (c *slackLiveChoice) credentialsFromEnv() (*hostapi.SlackCustomerApp, error) {
	credentials, err := slackCredentialsEnv(c.envVars)
	if err != nil {
		return nil, err
	}
	if credentials != nil {
		if err := ui.LogInfo(envAppNotice("Slack app", "LangSmith", []string{SlackBotTokenEnv, SlackSigningSecretEnv})); err != nil {
			return nil, err
		}
	}
	return credentials, nil
}

func (c *slackLiveChoice) manager(ctx context.Context) *hostapi.SlackManager {
	// Not knowing only drops the workspace's name from the choice.
	manager, err := c.client.SlackManager(ctx)
	if err != nil {
		warnBestEffort(fmt.Sprintf("Could not check whether LangSmith can create Slack apps for you: %v", err))
		return nil
	}
	return manager
}

func (c *slackLiveChoice) choose(ctx context.Context, options []slackAppOption) (slackAppSource, error) {
	prompt := ui.Select[slackAppSource]("This agent's Slack channel needs a Slack app")
	for _, option := range options {
		prompt = prompt.Item(option.source, option.label, option.hint)
	}
	source, err := prompt.Interact(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to choose a Slack app: %w", err)
	}
	return source, nil
}

func (c *slackLiveChoice) info(message string) error {
	return ui.LogInfo(message)
}

func (c *slackLiveChoice) manifest(ctx context.Context) (map[string]any, error) {
	return c.client.SlackAppManifest(ctx, c.slack.App, c.slack.Options)
}

func (c *slackLiveChoice) showCreatePage(link *url.URL) error {
	if err := ui.LogStep(fmt.Sprintf("Create the Slack app for this agent in your browser: Slack opens with the app filled in. If your browser does not open, visit:\n%s\nInstall the app to your workspace, then copy its Bot User OAuth Token (OAuth & Permissions) and Signing Secret (Basic Information) into the prompts below.", link)); err != nil {
		return err
	}
	c.openBrowserURL(link.String())
	return nil
}

func (c *slackLiveChoice) showExistingSettings(settings string) error {
	if err := ui.LogStep(settings); err != nil {
		return err
	}
	c.openBrowserURL(slackAppsURL)
	return nil
}

func (c *slackLiveChoice) askCredentials(ctx context.Context) (hostapi.SlackCustomerApp, error) {
	botToken, err := ui.Password("Bot User OAuth Token (xoxb-…)").Mask('•').Validate(validateSlackBotToken).Interact(ctx)
	if err != nil {
		return hostapi.SlackCustomerApp{}, fmt.Errorf("failed to read the Slack bot token: %w", err)
	}
	signingSecret, err := ui.Password("Signing Secret").Mask('•').Validate(validateSlackSigningSecret).Interact(ctx)
	if err != nil {
		return hostapi.SlackCustomerApp{}, fmt.Errorf("failed to read the Slack signing secret: %w", err)
	}
	return hostapi.SlackCustomerApp{
		BotToken:      strings.TrimSpace(botToken),
		SigningSecret: strings.TrimSpace(signingSecret),
	}, nil
}
