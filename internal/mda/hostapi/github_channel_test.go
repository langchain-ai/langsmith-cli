package hostapi

import (
	"slices"
	"testing"
)

const testAppID = "11111111-2222-3333-4444-555555555555"

func TestTheReportReadsTheAppFromAgentAuth(t *testing.T) {
	server := serveOnce(t, 200, `{
		"id":"`+testAppID+`","source":"manifest","slug":"mda/github-app/dep",
		"app_metadata":{"slug":"acme-triage","html_url":"https://github.com/apps/acme-triage","bot_id":"4242"},
		"github":{
			"app":{"id":42,"slug":"acme-triage","name":"acme-triage","owner":{"login":"acme","type":"Organization"},
				"permissions":{"issues":"write","metadata":"read"},"events":["issues"],
				"webhook_url":"https://agent.example/channels/github/events"},
			"installations":[{"id":7,"account":{"login":"acme","type":"Organization"},
				"repository_selection":"selected","repositories":["acme/api"],"repositories_truncated":true,
				"permissions":{"issues":"write","metadata":"read"},
				"settings_url":"https://github.com/organizations/acme/settings/installations/7"}],
			"installation_requests":[{"account":{"login":"globex","type":"Organization"}}]}}`)
	c := newClient(t, apiKeyCLIClient(t, "lsv2_test_key", ""), server.URL, server.URL)

	outcome := c.GitHubAppReport(t.Context(), testAppID)

	request := server.Only(t)
	if request.Line() != "GET /v1/agent-auth/apps/"+testAppID+"?include=repositories" || request.Header.Get("X-Api-Key") != "lsv2_test_key" {
		t.Errorf("request = %s, key %q", request.Line(), request.Header.Get("X-Api-Key"))
	}
	if outcome.Kind != GitHubSyncOutcomeSynced {
		t.Fatalf("outcome = %+v, want a report", outcome)
	}
	status := outcome.Status
	if status.App.Slug != "acme-triage" || status.App.BotID != 4242 ||
		status.App.WebhookURL != "https://agent.example/channels/github/events" || !status.App.Owner.IsOrganization() {
		t.Errorf("app = %+v", status.App)
	}
	if !slices.Equal(status.Installations[0].Repositories, []string{"acme/api"}) || !status.Installations[0].RepositoriesTruncated {
		t.Errorf("installation = %+v", status.Installations[0])
	}
	if status.InstallationRequests[0].Account.Login != "globex" {
		t.Errorf("requests = %+v", status.InstallationRequests)
	}
}

func TestAPendingAppHasNoReport(t *testing.T) {
	outcome := appTestClient(t, serveOnce(t, 200, `{"id":"`+testAppID+`","slug":"x"}`).URL).GitHubAppReport(t.Context(), testAppID)
	if outcome.Kind != GitHubSyncOutcomeNotCreated {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestTheWebhookIsPointedWithAPatch(t *testing.T) {
	server := serveOnce(t, 200, "{}")
	if err := appTestClient(t, server.URL).PointGitHubAppWebhook(t.Context(), testAppID, "https://agent.example/channels/github/events"); err != nil {
		t.Fatal(err)
	}
	request := server.Only(t)
	if request.Line() != "PATCH /v1/agent-auth/apps/"+testAppID ||
		!jsonEqual(t, request.JSON(t), `{"manifest":{"webhook_url":"https://agent.example/channels/github/events"},"overwrite_external":true}`) {
		t.Fatalf("request = %s %s", request.Line(), request.Body)
	}
}

func TestTheReportMapsErrorStatuses(t *testing.T) {
	for status, want := range map[int]GitHubSyncOutcome{
		404: {Kind: GitHubSyncOutcomeMissingCredentials},
		403: {Kind: GitHubSyncOutcomeMissingCredentials},
		422: {Kind: GitHubSyncOutcomeInvalidCredentials},
		502: {Kind: GitHubSyncOutcomeUnavailable, Reason: "HTTP 502"},
	} {
		got := appTestClient(t, serveOnce(t, status, `{"error":"x","message":"y"}`).URL).GitHubAppReport(t.Context(), testAppID)
		if got != want {
			t.Errorf("%d: outcome = %+v, want %+v", status, got, want)
		}
	}
}

func TestTheReportRejectsAMalformedAnswer(t *testing.T) {
	got := appTestClient(t, serveOnce(t, 200, `{"github":{"unexpected":true}}`).URL).GitHubAppReport(t.Context(), testAppID)
	if got != (GitHubSyncOutcome{Kind: GitHubSyncOutcomeUnavailable, Reason: "Agent Auth returned an unexpected GitHub App status"}) {
		t.Fatalf("outcome = %+v", got)
	}
}

func TestTheReportNamesAnUnreachableAgentAuth(t *testing.T) {
	got := appTestClient(t, "http://127.0.0.1:1").GitHubAppReport(t.Context(), testAppID)
	if got != (GitHubSyncOutcome{Kind: GitHubSyncOutcomeUnavailable, Reason: "Agent Auth could not be reached"}) {
		t.Fatalf("outcome = %+v", got)
	}
}
