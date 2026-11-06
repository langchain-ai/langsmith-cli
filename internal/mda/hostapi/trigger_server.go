package hostapi

// Trigger Server endpoints on the public platform API: channel setup, the
// channels a deployment has, and the Slack app LangSmith can create.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// TransientSetupError is a channel setup request that may succeed when made
// again: it never reached Trigger Server, or Trigger Server or Slack was busy
// (a 5xx or 429).
type TransientSetupError struct {
	Message string
	// Err is why the request never got an answer, nil for a busy answer.
	Err error
}

func (e *TransientSetupError) Error() string { return e.Message }

func (e *TransientSetupError) Unwrap() error { return e.Err }

// IsTransientSetupError reports whether a failed channel setup may succeed
// when asked again.
func IsTransientSetupError(err error) bool {
	var transient *TransientSetupError
	return errors.As(err, &transient)
}

const setupErrorFieldMaxChars = 512

// ChannelSetupRequest is the body of a channel setup.
type ChannelSetupRequest struct {
	Provider string             `json:"provider"`
	Target   ChannelSetupTarget `json:"target"`
	Slack    SlackChannelSetup  `json:"slack"`
}

// ChannelSetupTarget is the deployment endpoint a channel delivers to.
type ChannelSetupTarget struct {
	DeploymentID string `json:"deployment_id"`
	EndpointURL  string `json:"endpoint_url"`
	ChannelName  string `json:"channel_name"`
	AgentID      string `json:"agent_id"`
}

// SlackChannelSetup is the Slack part of a channel setup.
type SlackChannelSetup struct {
	App     SlackChannelApp     `json:"app"`
	Options SlackChannelOptions `json:"options"`
	// CustomerApp is a Slack app the user owns, instead of one LangSmith
	// creates. Trigger Server keeps its credentials, so a redeploy leaves this
	// out.
	CustomerApp *SlackCustomerApp `json:"customer_app,omitempty"`
}

// SlackCustomerApp holds the credentials of a Slack app the user owns.
// Formatting it never prints them.
type SlackCustomerApp struct {
	BotToken      string `json:"bot_token"`
	SigningSecret string `json:"signing_secret"`
	// MoveFromDeployment is the deployment this app may be moved away from,
	// once the user agreed. Trigger Server refuses to take an app another
	// deployment uses otherwise.
	MoveFromDeployment string `json:"move_from_deployment,omitempty"`
}

// String redacts the credentials.
func (a SlackCustomerApp) String() string {
	return fmt.Sprintf("SlackCustomerApp { bot_token: \"[REDACTED]\", signing_secret: \"[REDACTED]\", move_from_deployment: %q }", a.MoveFromDeployment)
}

// GoString redacts the credentials.
func (a SlackCustomerApp) GoString() string {
	return a.String()
}

// SlackAppInUse is channel setup refusing the user's Slack app because
// another deployment in the workspace uses it. Slack sends an app's events to
// one place, so it can serve one deployment at a time.
type SlackAppInUse struct {
	DeploymentID string
}

func (e *SlackAppInUse) Error() string {
	return fmt.Sprintf("Slack app is connected to deployment %s.", e.DeploymentID)
}

// slackAppInUse is the holder a 409 names when it refuses a Slack app another
// deployment uses. Only a well-formed deployment ID is taken from the
// untrusted body.
func slackAppInUse(status int, text []byte) *SlackAppInUse {
	if status != http.StatusConflict {
		return nil
	}
	var body any
	if err := json.Unmarshal(text, &body); err != nil {
		return nil
	}
	detail := field(body, "detail")
	if code, ok := field(detail, "code").(string); !ok || code != "slack_app_in_use" {
		return nil
	}
	holder, ok := field(detail, "deployment_id").(string)
	if !ok {
		return nil
	}
	deploymentID, err := uuid.Parse(holder)
	if err != nil {
		return nil
	}
	return &SlackAppInUse{DeploymentID: deploymentID.String()}
}

// SlackManagerKind is whether LangSmith can create a Slack app for the user.
type SlackManagerKind int

// The answers.
const (
	// SlackManagerAuthorized: the user let LangSmith create apps in the
	// workspace [SlackManager.TeamName].
	SlackManagerAuthorized SlackManagerKind = iota
	// SlackManagerNotAuthorized: creating one first asks the user to authorize
	// LangSmith in Slack.
	SlackManagerNotAuthorized
	// SlackManagerUnavailable: this LangSmith cannot create Slack apps.
	SlackManagerUnavailable
)

// SlackManager is whether LangSmith can create a Slack app for the user, and
// where.
type SlackManager struct {
	Kind SlackManagerKind
	// TeamName names the workspace of [SlackManagerAuthorized].
	TeamName string
}

type slackManagerResponse struct {
	Authorized bool   `json:"authorized"`
	TeamID     string `json:"team_id"`
	TeamName   string `json:"team_name"`
}

// UnmarshalJSON requires authorized.
func (r *slackManagerResponse) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "authorized"); err != nil {
		return err
	}
	type plain slackManagerResponse
	return json.Unmarshal(data, (*plain)(r))
}

// TriggerChannel is a channel Trigger Server has registered for a deployment.
type TriggerChannel struct {
	Provider    string `json:"provider"`
	ChannelName string `json:"channel_name"`
}

// UnmarshalJSON requires the provider and channel name.
func (c *TriggerChannel) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "provider", "channel_name"); err != nil {
		return err
	}
	type plain TriggerChannel
	return json.Unmarshal(data, (*plain)(c))
}

type triggerChannelList struct {
	Channels []TriggerChannel `json:"channels"`
}

// UnmarshalJSON requires the channels.
func (l *triggerChannelList) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "channels"); err != nil {
		return err
	}
	type plain triggerChannelList
	return json.Unmarshal(data, (*plain)(l))
}

// SlackChannelApp is the Slack app a channel setup creates or updates.
type SlackChannelApp struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	Image           string `json:"image"`
	BackgroundColor string `json:"background_color"`
}

// SlackChannelOptions are the Slack channel's options.
type SlackChannelOptions struct {
	// TriggerOnAllMessages is deprecated in favor of TriggerMode; still sent
	// for Trigger Servers that only read this flag.
	TriggerOnAllMessages bool `json:"triggerOnAllMessages"`
	// TriggerMode is only sent when the channel sets it, so default payloads
	// stay identical for consumers that predate the option. Unset lets Trigger
	// Server keep an existing app's mode or default a new one to `thread`.
	TriggerMode      *SlackTriggerMode `json:"triggerMode,omitempty"`
	AllowBotTriggers bool              `json:"allowBotTriggers"`
}

// SlackTriggerMode is which channel messages start a run; DMs always do.
type SlackTriggerMode int

// The modes.
const (
	// SlackTriggerModeMention: only messages that mention the agent.
	SlackTriggerModeMention SlackTriggerMode = iota
	// SlackTriggerModeThread: mentions, and later replies in threads the agent
	// was mentioned in.
	SlackTriggerModeThread
	// SlackTriggerModeAll: every message in the agent's channels. Sent to
	// Trigger Server as the legacy `triggerOnAllMessages` flag, which every
	// version understands.
	SlackTriggerModeAll
)

// String is the mode as Trigger Server spells it.
func (m SlackTriggerMode) String() string {
	switch m {
	case SlackTriggerModeThread:
		return "thread"
	case SlackTriggerModeAll:
		return "all"
	default:
		return "mention"
	}
}

// MarshalText serializes the mode as Trigger Server spells it.
func (m SlackTriggerMode) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}

// UnmarshalText reads a mode as Trigger Server spells it.
func (m *SlackTriggerMode) UnmarshalText(text []byte) error {
	for _, mode := range []SlackTriggerMode{SlackTriggerModeMention, SlackTriggerModeThread, SlackTriggerModeAll} {
		if mode.String() == string(text) {
			*m = mode
			return nil
		}
	}
	return fmt.Errorf("unknown variant `%s`, expected one of `mention`, `thread`, `all`", text)
}

// ChannelSetupResponse is Trigger Server's answer to a channel setup.
type ChannelSetupResponse struct {
	Status               ChannelSetupStatus `json:"status"`
	ChannelID            string             `json:"channel_id"`
	SourceRegistrationID string             `json:"source_registration_id"`
	AuthorizationURL     string             `json:"authorization_url"`
	Message              string             `json:"message"`
}

// ChannelSetupStatus is the state a channel setup left the channel in.
type ChannelSetupStatus int

// The statuses.
const (
	ChannelSetupStatusActive ChannelSetupStatus = iota
	ChannelSetupStatusAuthorizationRequired
	ChannelSetupStatusPendingApproval
)

var channelSetupStatusNames = [...]string{
	ChannelSetupStatusActive:                "active",
	ChannelSetupStatusAuthorizationRequired: "authorization_required",
	ChannelSetupStatusPendingApproval:       "pending_approval",
}

// String is the status's wire name.
func (s ChannelSetupStatus) String() string {
	if s < 0 || int(s) >= len(channelSetupStatusNames) {
		return fmt.Sprintf("ChannelSetupStatus(%d)", int(s))
	}
	return channelSetupStatusNames[s]
}

// UnmarshalText reads a status by its wire name.
func (s *ChannelSetupStatus) UnmarshalText(text []byte) error {
	for status, name := range channelSetupStatusNames {
		if name == string(text) {
			*s = ChannelSetupStatus(status)
			return nil
		}
	}
	return fmt.Errorf("unknown variant `%s`", text)
}

// decodeChannelSetupResponse reads a setup response strictly: the status is
// required and unknown fields are refused.
func decodeChannelSetupResponse(text []byte) (ChannelSetupResponse, error) {
	if err := requireFields(text, "status"); err != nil {
		return ChannelSetupResponse{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(text))
	decoder.DisallowUnknownFields()
	var response ChannelSetupResponse
	if err := decoder.Decode(&response); err != nil {
		return ChannelSetupResponse{}, err
	}
	return response, nil
}

// triggerURL is a Trigger Server URL under the public platform API.
func (c *Client) triggerURL(segments []string, what string) (string, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return "", err
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" {
		return "", errors.New("LangSmith platform API URL is invalid")
	}
	if u.Opaque != "" {
		return "", fmt.Errorf("%s URL is invalid.", what)
	}
	return u.JoinPath(segments...).String(), nil
}

// triggerGet GETs a Trigger Server path under the platform API, with the
// CLI's credential.
func (c *Client) triggerGet(ctx context.Context, segments []string, query []queryParam, what string) (int, []byte, error) {
	target, err := c.triggerURL(segments, what)
	if err != nil {
		return 0, nil, err
	}
	status, text, err := c.send(ctx, target, http.MethodGet, query, nil, requestOptions{noRedirects: true})
	if err != nil {
		return 0, nil, fmt.Errorf("%s request failed: %w", what, err)
	}
	return status, text, nil
}

// SlackAppManifest is the manifest of a Slack app Trigger Server can receive
// events from, for the user to create or match their own app with.
func (c *Client) SlackAppManifest(ctx context.Context, app SlackChannelApp, options SlackChannelOptions) (map[string]any, error) {
	trigger := "false"
	if options.TriggerOnAllMessages {
		trigger = "true"
	}
	query := []queryParam{
		{"name", app.Name},
		{"description", app.Description},
		{"background_color", app.BackgroundColor},
		{"trigger_on_all_messages", trigger},
	}
	if options.TriggerMode != nil {
		query = append(query, queryParam{"trigger_mode", options.TriggerMode.String()})
	}
	status, text, err := c.triggerGet(ctx, []string{"v1", "triggers", "slack-custom-app", "manifest"}, query, "Slack manifest")
	if err != nil {
		return nil, err
	}
	if !isSuccess(status) {
		return nil, fmt.Errorf("Slack manifest request failed (HTTP %d).", status)
	}
	var manifest map[string]any
	if err := json.Unmarshal(text, &manifest); err != nil {
		return nil, errors.New("Slack manifest request returned invalid JSON.")
	}
	return manifest, nil
}

// SlackManager reports whether LangSmith can create a Slack app for the user,
// and in which workspace. nil when this Trigger Server cannot say.
func (c *Client) SlackManager(ctx context.Context) (*SlackManager, error) {
	status, text, err := c.triggerGet(ctx, []string{"v1", "triggers", "slack-custom-app", "manager"}, nil, "Slack app management")
	if err != nil {
		return nil, err
	}
	switch {
	case status == http.StatusNotFound:
		return nil, nil
	case status == http.StatusServiceUnavailable:
		return &SlackManager{Kind: SlackManagerUnavailable}, nil
	case !isSuccess(status):
		return nil, fmt.Errorf("Slack app management lookup failed (HTTP %d).", status)
	}
	var body slackManagerResponse
	if err := json.Unmarshal(text, &body); err != nil {
		return nil, errors.New("Slack app management lookup returned invalid JSON.")
	}
	if !body.Authorized {
		return &SlackManager{Kind: SlackManagerNotAuthorized}, nil
	}
	printable := func(value string) (string, bool) {
		return value, strings.TrimSpace(value) != "" && strings.IndexFunc(value, unicode.IsControl) < 0
	}
	teamName, ok := printable(body.TeamName)
	if !ok {
		teamName, ok = printable(body.TeamID)
	}
	if !ok {
		teamName = "your Slack workspace"
	}
	return &SlackManager{Kind: SlackManagerAuthorized, TeamName: teamName}, nil
}

// TriggerChannels lists the channels Trigger Server has registered for the
// deployment.
func (c *Client) TriggerChannels(ctx context.Context, deploymentID string) ([]TriggerChannel, error) {
	status, text, err := c.triggerGet(ctx, []string{"v1", "channels"}, []queryParam{{"deployment_id", deploymentID}}, "Channel listing")
	if err != nil {
		return nil, err
	}
	if !isSuccess(status) {
		return nil, fmt.Errorf("Channel listing failed (HTTP %d).", status)
	}
	var list triggerChannelList
	if err := json.Unmarshal(text, &list); err != nil {
		return nil, errors.New("Channel listing returned an invalid response.")
	}
	return list.Channels, nil
}

// SetupTriggerChannel registers a channel with Trigger Server.
//
// Whatever credential the request carried (an API key, or an OAuth token) is
// redacted from Trigger Server's answer before it reaches an error or the
// response.
func (c *Client) SetupTriggerChannel(ctx context.Context, payload *ChannelSetupRequest) (ChannelSetupResponse, error) {
	target, err := c.triggerURL([]string{"v1", "channels"}, "Channel setup")
	if err != nil {
		return ChannelSetupResponse{}, err
	}
	var credential string
	status, text, err := c.send(ctx, target, http.MethodPost, nil, payload, requestOptions{
		noRedirects: true,
		observe:     func(req *http.Request) { credential = sentCredential(req) },
	})
	if err != nil {
		return ChannelSetupResponse{}, &TransientSetupError{Message: "Channel setup request failed.", Err: err}
	}
	if !isSuccess(status) {
		if inUse := slackAppInUse(status, text); inUse != nil {
			return ChannelSetupResponse{}, inUse
		}
		message := setupFailureMessage(status, text, credential)
		if customerApp := payload.Slack.CustomerApp; customerApp != nil {
			// The server never echoes them, but its detail is untrusted.
			for _, secret := range []string{customerApp.BotToken, customerApp.SigningSecret} {
				if secret != "" {
					message = strings.ReplaceAll(message, secret, "[REDACTED]")
				}
			}
		}
		if status >= 500 || status == http.StatusTooManyRequests {
			return ChannelSetupResponse{}, &TransientSetupError{Message: message}
		}
		return ChannelSetupResponse{}, errors.New(message)
	}
	response, err := decodeChannelSetupResponse(text)
	if err != nil {
		return ChannelSetupResponse{}, errors.New("Channel setup returned an invalid response.")
	}
	if credential != "" {
		response.Message = strings.ReplaceAll(response.Message, credential, "[REDACTED]")
		if strings.Contains(response.AuthorizationURL, credential) {
			response.AuthorizationURL = ""
		}
	}
	return response, nil
}

// sentCredential is the secret part of the credential req carries: its API
// key, or its bearer token.
func sentCredential(req *http.Request) string {
	if key := req.Header.Get("X-Api-Key"); key != "" {
		return key
	}
	authorization := req.Header.Get("Authorization")
	if token, ok := strings.CutPrefix(authorization, "Bearer "); ok {
		return token
	}
	return authorization
}

// setupFailureMessage is the error for a failed channel setup: the status,
// plus the Trigger Server's `detail` and `remedy` when its body is JSON. A
// body that is not JSON is never echoed, and each field is dropped unless it
// is short, printable text. credential is redacted from both.
func setupFailureMessage(status int, text []byte, credential string) string {
	safe := func(value string) (string, bool) {
		if credential != "" {
			value = strings.ReplaceAll(value, credential, "[REDACTED]")
		}
		value = strings.TrimSpace(value)
		ok := value != "" &&
			utf8.RuneCountInString(value) <= setupErrorFieldMaxChars &&
			strings.IndexFunc(value, unicode.IsControl) < 0
		return value, ok
	}
	humanized, ok := humanizeErrorBody(text)
	if !ok {
		return fmt.Sprintf("Channel setup failed (HTTP %d).", status)
	}
	detail, ok := safe(humanized)
	if !ok {
		return fmt.Sprintf("Channel setup failed (HTTP %d).", status)
	}
	message := fmt.Sprintf("Channel setup failed (HTTP %d): %s", status, detail)
	if !strings.HasSuffix(detail, ".") && !strings.HasSuffix(detail, "!") && !strings.HasSuffix(detail, "?") {
		message += "."
	}
	var body any
	if json.Unmarshal(text, &body) == nil {
		if remedy, ok := field(body, "remedy").(string); ok {
			if remedy, ok := safe(remedy); ok {
				message += " " + remedy
			}
		}
	}
	return message
}
