package hostapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// triggerClient is a client for a stub platform API at platform, with the
// API key "resolved-login-key".
func triggerClient(t *testing.T, tenant, platform string) *Client {
	t.Helper()
	return newClient(t, apiKeyCLIClient(t, "resolved-login-key", tenant), "https://host.invalid", platform)
}

// selfHostedTriggerClient is a client for a self-hosted installation at
// platform under basePath.
func selfHostedTriggerClient(t *testing.T, platform, basePath string) *Client {
	t.Helper()
	platform += basePath
	return newClient(t, apiKeyCLIClient(t, "resolved-login-key", ""), platform+"/api-host", platform)
}

func setupRequest(t *testing.T) *ChannelSetupRequest {
	t.Helper()
	var request ChannelSetupRequest
	err := json.Unmarshal([]byte(`{
		"provider":"slack",
		"target":{"deployment_id":"deployment-id","endpoint_url":"https://agent.example/v1/channels/slack/events","channel_name":"slack","agent_id":"assistant-id"},
		"slack":{
			"app":{"name":"Managed Deep Agent","description":"Managed Deep Agent channel","image":"data:image/png;base64,aW1hZ2U=","background_color":"#09090f"},
			"options":{"triggerOnAllMessages":false,"allowBotTriggers":false}}}`), &request)
	if err != nil {
		t.Fatal(err)
	}
	return &request
}

const activeSetup = `{"status":"active","channel_id":"a6190f67-865d-4705-8171-35c90e64c730","source_registration_id":"f4cde312-d221-44a7-8417-00f13d3b868a","authorization_url":null,"message":null}`

func TestSetupChannelPostsExactPathBodyAndAuthHeaders(t *testing.T) {
	server := serveOnce(t, 200, activeSetup)

	response, err := triggerClient(t, "tenant-id", server.URL).SetupTriggerChannel(t.Context(), setupRequest(t))

	if err != nil || response.Status != ChannelSetupStatusActive || response.ChannelID != "a6190f67-865d-4705-8171-35c90e64c730" {
		t.Fatalf("response = %+v, %v", response, err)
	}
	request := server.Only(t)
	if request.Line() != "POST /v1/channels" || request.Header.Get("X-Api-Key") != "resolved-login-key" || request.Header.Get("X-Tenant-Id") != "tenant-id" {
		t.Errorf("request = %s, headers %v", request.Line(), request.Header)
	}
	if !jsonEqual(t, request.JSON(t), mustJSON(t, setupRequest(t))) {
		t.Errorf("body = %s", request.Body)
	}
}

func TestSetupChannelSendsTheOAuthToken(t *testing.T) {
	server := serveOnce(t, 200, `{"status":"authorization_required","authorization_url":"https://trigger.example/authorize?t=oauth-token-value","message":"echoed oauth-token-value"}`)
	c := newClient(t, oauthCLIClient(t, "oauth-token-value", ""), "https://host.invalid", server.URL)

	response, err := c.SetupTriggerChannel(t.Context(), setupRequest(t))

	if err != nil || response.Message != "echoed [REDACTED]" || response.AuthorizationURL != "" {
		t.Fatalf("response = %+v, %v", response, err)
	}
	if got := server.Only(t).Header.Get("Authorization"); got != "Bearer oauth-token-value" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestSelfHostedSetupChannelUsesThePublicAPIPrefix(t *testing.T) {
	for basePath, want := range map[string]string{"": "POST /api/v1/channels", "/langsmith": "POST /langsmith/api/v1/channels"} {
		server := serveOnce(t, 200, activeSetup)
		if _, err := selfHostedTriggerClient(t, server.URL, basePath).SetupTriggerChannel(t.Context(), setupRequest(t)); err != nil {
			t.Fatal(err)
		}
		if line := server.Only(t).Line(); line != want {
			t.Errorf("%q: request = %s, want %s", basePath, line, want)
		}
	}
}

func TestSetupChannelRedactsTheSentAPIKeyFromResponseFields(t *testing.T) {
	server := serveOnce(t, 200, `{"status":"authorization_required","channel_id":null,"source_registration_id":null,
		"authorization_url":"https://trigger.example/authorize?state=resolved-login-key","message":"Trigger echoed resolved-login-key in its response"}`)

	response, err := triggerClient(t, "", server.URL).SetupTriggerChannel(t.Context(), setupRequest(t))

	if err != nil || response.Message != "Trigger echoed [REDACTED] in its response" || response.AuthorizationURL != "" {
		t.Fatalf("response = %+v, %v", response, err)
	}
}

func TestSetupChannelRejectsAnUnknownResponseShape(t *testing.T) {
	for _, body := range []string{`{"status":"active","extra":1}`, `{"status":"unknown"}`, `{}`, `not json`} {
		_, err := triggerClient(t, "", serveOnce(t, 200, body).URL).SetupTriggerChannel(t.Context(), setupRequest(t))
		if err == nil || err.Error() != "Channel setup returned an invalid response." {
			t.Errorf("%s: err = %v", body, err)
		}
	}
}

func TestSetupChannelDoesNotFollowRedirects(t *testing.T) {
	destination, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	contacted := make(chan struct{}, 1)
	go func() {
		if connection, err := destination.Accept(); err == nil {
			contacted <- struct{}{}
			connection.Close()
		}
	}()
	server := serveSequence(t, reply{status: http.StatusFound, body: "redirect-body-secret", header: map[string]string{"Location": "http://" + destination.Addr().String() + "/captured"}})

	_, err = triggerClient(t, "", server.URL).SetupTriggerChannel(t.Context(), setupRequest(t))

	if err == nil || err.Error() != "Channel setup failed (HTTP 302)." {
		t.Fatalf("err = %v", err)
	}
	select {
	case <-contacted:
		t.Fatal("redirect destination was contacted")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSetupChannelNonSuccessErrorIsolatedFromCredentialsAndResponse(t *testing.T) {
	server := serveSequence(t, reply{status: 401, body: "remote-response-secret", header: map[string]string{"Location": "https://evil.example/leak"}})
	_, err := triggerClient(t, "", server.URL).SetupTriggerChannel(t.Context(), setupRequest(t))
	if err == nil || err.Error() != "Channel setup failed (HTTP 401)." || IsTransientSetupError(err) {
		t.Fatalf("err = %v", err)
	}
}

func setupError(t *testing.T, status int, body string) error {
	t.Helper()
	_, err := triggerClient(t, "", serveOnce(t, status, body).URL).SetupTriggerChannel(t.Context(), setupRequest(t))
	if err == nil {
		t.Fatal("setup succeeded")
	}
	return err
}

func TestSetupChannelErrorIncludesTheTriggerServerDetail(t *testing.T) {
	err := setupError(t, 502, `{"detail":"Unable to update the existing Slack app"}`)
	if err.Error() != "Channel setup failed (HTTP 502): Unable to update the existing Slack app." || !IsTransientSetupError(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestSetupChannelErrorAppendsTheRemedy(t *testing.T) {
	err := setupError(t, 403, `{"type":"https://docs.langchain.com/errors/channel-setup","title":"Forbidden","status":403,
		"detail":"Slack did not allow this user to update the app.","remedy":"Ask a Slack admin to add you as an app collaborator."}`)
	if err.Error() != "Channel setup failed (HTTP 403): Slack did not allow this user to update the app. Ask a Slack admin to add you as an app collaborator." {
		t.Fatalf("err = %v", err)
	}
}

func TestSetupChannelErrorSummarizesValidationDetails(t *testing.T) {
	err := setupError(t, 422, `{"detail":[{"type":"string_too_long","loc":["body","slack","app","description"],"msg":"String should have at most 139 characters","input":"resolved-login-key"}]}`)
	if err.Error() != "Channel setup failed (HTTP 422): Invalid value for 'slack.app.description': String should have at most 139 characters." {
		t.Fatalf("err = %v", err)
	}
}

func TestSetupChannelErrorRedactsTheAPIKeyFromDetailAndRemedy(t *testing.T) {
	err := setupError(t, 409, `{"detail":"Key resolved-login-key cannot own this channel.","remedy":"Rotate resolved-login-key."}`)
	if err.Error() != "Channel setup failed (HTTP 409): Key [REDACTED] cannot own this channel. Rotate [REDACTED]." {
		t.Fatalf("err = %v", err)
	}
}

func TestSetupChannelErrorDropsUnsafeDetailAndRemedy(t *testing.T) {
	if err := setupError(t, 502, `{"detail":"Slack failed\u001b[2J","remedy":"Retry."}`); err.Error() != "Channel setup failed (HTTP 502)." {
		t.Errorf("err = %v", err)
	}
	if err := setupError(t, 502, `{"detail":"Slack failed.","remedy":"`+strings.Repeat("x", 513)+`"}`); err.Error() != "Channel setup failed (HTTP 502): Slack failed." {
		t.Errorf("err = %v", err)
	}
}

func TestATransportFailureIsTransient(t *testing.T) {
	_, err := triggerClient(t, "", "http://127.0.0.1:1").SetupTriggerChannel(t.Context(), setupRequest(t))
	if err == nil || err.Error() != "Channel setup request failed." || !IsTransientSetupError(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestCustomerAppCredentialsAreSentButNeverPrinted(t *testing.T) {
	payload := setupRequest(t)
	payload.Slack.CustomerApp = &SlackCustomerApp{BotToken: "xoxb-secret", SigningSecret: "signing-secret"}

	if !jsonEqual(t, field(field(decodeJSONText(t, mustJSON(t, payload)), "slack"), "customer_app"), `{"bot_token":"xoxb-secret","signing_secret":"signing-secret"}`) {
		t.Errorf("serialized = %s", mustJSON(t, payload))
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		if printed := fmt.Sprintf(format, payload); strings.Contains(printed, "xoxb-secret") || strings.Contains(printed, "signing-secret") {
			t.Errorf("%s printed the credentials: %s", format, printed)
		}
	}
	if strings.Contains(mustJSON(t, setupRequest(t)), "customer_app") {
		t.Error("a request without a customer app sends one")
	}
}

func decodeJSONText(t *testing.T, text string) any {
	t.Helper()
	value, err := decodeAny([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestFailedCustomerSetupNeverEchoesTheCredentials(t *testing.T) {
	payload := setupRequest(t)
	payload.Slack.CustomerApp = &SlackCustomerApp{BotToken: "xoxb-secret", SigningSecret: "signing-secret"}
	server := serveOnce(t, 400, `{"detail":"Token xoxb-secret is invalid"}`)

	_, err := triggerClient(t, "", server.URL).SetupTriggerChannel(t.Context(), payload)

	if err == nil || err.Error() != "Channel setup failed (HTTP 400): Token [REDACTED] is invalid." {
		t.Fatalf("err = %v", err)
	}
}

func TestASlackAppAnotherDeploymentUsesIsATypedError(t *testing.T) {
	holder := "b809e66e-0000-4000-8000-000000000002"
	payload := setupRequest(t)
	payload.Slack.CustomerApp = &SlackCustomerApp{BotToken: "xoxb-secret", SigningSecret: "signing-secret"}
	server := serveOnce(t, 409, `{"detail":{"message":"This Slack app is connected to deployment `+holder+`","code":"slack_app_in_use","deployment_id":"`+holder+`","channel_name":"slack"}}`)

	_, err := triggerClient(t, "", server.URL).SetupTriggerChannel(t.Context(), payload)

	var inUse *SlackAppInUse
	if !errors.As(err, &inUse) || inUse.DeploymentID != holder || err.Error() != "Slack app is connected to deployment "+holder+"." {
		t.Fatalf("err = %v", err)
	}
}

func TestSlackAppInUseNeedsAConflictWithADeploymentID(t *testing.T) {
	holder := "b809e66e-0000-4000-8000-000000000002"
	body := func(deploymentID string) []byte {
		return []byte(`{"detail":{"code":"slack_app_in_use","deployment_id":"` + deploymentID + `"}}`)
	}
	if slackAppInUse(409, body(holder)) == nil {
		t.Error("a conflict naming a deployment is not in use")
	}
	if slackAppInUse(400, body(holder)) != nil {
		t.Error("a bad request is in use")
	}
	if slackAppInUse(409, body(`\u001b[2J`)) != nil {
		t.Error("a malformed deployment id is in use")
	}
	if slackAppInUse(409, []byte(`{"detail":"This Slack app is already in use"}`)) != nil {
		t.Error("a bare message is in use")
	}
}

func TestAMoveNamesTheDeploymentItReleases(t *testing.T) {
	app := SlackCustomerApp{BotToken: "xoxb-secret", SigningSecret: "signing-secret", MoveFromDeployment: "holder-id"}
	if field(decodeJSONText(t, mustJSON(t, app)), "move_from_deployment") != "holder-id" {
		t.Errorf("serialized = %s", mustJSON(t, app))
	}
	if strings.Contains(fmt.Sprintf("%v", app), "xoxb-secret") {
		t.Error("printed the token")
	}
}

func TestSlackManagerNamesTheAuthorizedWorkspace(t *testing.T) {
	server := serveOnce(t, 200, `{"authorized":true,"team_id":"T1","team_name":"Acme"}`)

	manager, err := triggerClient(t, "tenant-id", server.URL).SlackManager(t.Context())

	if err != nil || manager == nil || *manager != (SlackManager{Kind: SlackManagerAuthorized, TeamName: "Acme"}) {
		t.Fatalf("manager = %+v, %v", manager, err)
	}
	request := server.Only(t)
	if request.Line() != "GET /v1/triggers/slack-custom-app/manager" || request.Header.Get("X-Tenant-Id") != "tenant-id" {
		t.Fatalf("request = %s, tenant %q", request.Line(), request.Header.Get("X-Tenant-Id"))
	}
}

func TestSlackManagerReadsEveryAnswer(t *testing.T) {
	for _, tt := range []struct {
		status int
		body   string
		want   *SlackManager
	}{
		{200, `{"authorized":false}`, &SlackManager{Kind: SlackManagerNotAuthorized}},
		{200, `{"authorized":true,"team_id":"T1","team_name":"\u001b[31m"}`, &SlackManager{Kind: SlackManagerAuthorized, TeamName: "T1"}},
		{200, `{"authorized":true}`, &SlackManager{Kind: SlackManagerAuthorized, TeamName: "your Slack workspace"}},
		{503, `{"detail":"not configured"}`, &SlackManager{Kind: SlackManagerUnavailable}},
		{404, `{}`, nil},
	} {
		got, err := triggerClient(t, "", serveOnce(t, tt.status, tt.body).URL).SlackManager(t.Context())
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%d %s: manager = %+v, %v; want %+v", tt.status, tt.body, got, err, tt.want)
		}
	}
	_, err := triggerClient(t, "", serveOnce(t, 500, `{}`).URL).SlackManager(t.Context())
	if err == nil || err.Error() != "Slack app management lookup failed (HTTP 500)." {
		t.Errorf("err = %v", err)
	}
}

func TestTriggerChannelsListsTheDeploymentsChannels(t *testing.T) {
	server := serveOnce(t, 200, `{"channels":[{"channel_id":"a6190f67-865d-4705-8171-35c90e64c730","deployment_id":"deployment-id",
		"provider":"slack","channel_name":"support","assistant_id":"agent","endpoint_url":"https://agent.example/v1/channels/support/events",
		"status":"active","created_at":"2026-10-21T00:00:00Z","updated_at":"2026-10-21T00:00:00Z"}]}`)

	channels, err := triggerClient(t, "", server.URL).TriggerChannels(t.Context(), "deployment-id")

	if err != nil || len(channels) != 1 || channels[0] != (TriggerChannel{Provider: "slack", ChannelName: "support"}) {
		t.Fatalf("channels = %+v, %v", channels, err)
	}
	if line := server.Only(t).Line(); line != "GET /v1/channels?deployment_id=deployment-id" {
		t.Fatalf("request = %s", line)
	}
}

func TestSlackAppManifestAsksForTheChannelsApp(t *testing.T) {
	server := serveOnce(t, 200, `{"display_information":{"name":"Support"}}`)
	payload := setupRequest(t)
	mode := SlackTriggerModeThread
	payload.Slack.Options.TriggerMode = &mode

	manifest, err := triggerClient(t, "", server.URL).SlackAppManifest(t.Context(), payload.Slack.App, payload.Slack.Options)

	if err != nil || !jsonEqual(t, manifest, `{"display_information":{"name":"Support"}}`) {
		t.Fatalf("manifest = %v, %v", manifest, err)
	}
	line := server.Only(t).Line()
	for _, part := range []string{"GET /v1/triggers/slack-custom-app/manifest?", "name=Managed+Deep+Agent", "trigger_on_all_messages=false", "trigger_mode=thread"} {
		if !strings.Contains(line, part) {
			t.Errorf("request %s lacks %s", line, part)
		}
	}
	if !strings.Contains(mustJSON(t, payload.Slack.Options), `"triggerMode":"thread"`) {
		t.Errorf("options = %s", mustJSON(t, payload.Slack.Options))
	}
}
