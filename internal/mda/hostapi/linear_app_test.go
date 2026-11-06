package hostapi

import (
	"strings"
	"testing"
)

func linearApp(scopes []string) ExistingLinearApp {
	return ExistingLinearApp{
		ClientID:      "client-1",
		ClientSecret:  "linear-client-secret",
		WebhookSecret: "lin_wh_signing",
		Scopes:        scopes,
		AllowedScopes: scopes,
	}
}

func TestAnExistingLinearAppIsBroughtInWithItsCredentialsAndScopes(t *testing.T) {
	server := serveOnce(t, 201, `{"app":{"id":"app-1","provider":"linear","source":"existing","slug":"mda/linear-app/dep-1","connection_id":"conn-1"}}`)

	created, err := appTestClient(t, server.URL).BringLinearApp(t.Context(), "dep-1", "mda/linear-app/dep-1", linearApp([]string{"read", "write"}))

	if err != nil || created.ID != "app-1" || created.ConnectionID != "conn-1" {
		t.Fatalf("created = %+v, %v", created, err)
	}
	request := server.Only(t)
	want := `{"provider":"linear","source":"existing","owner_id":"dep-1","slug":"mda/linear-app/dep-1",
		"existing":{"client_id":"client-1","client_secret":"linear-client-secret","webhook_secret":"lin_wh_signing","scopes":["read","write"],"allowed_scopes":["read","write"]}}`
	if request.Line() != "POST /v1/agent-auth/apps" || !jsonEqual(t, request.JSON(t), want) {
		t.Fatalf("request = %s %s", request.Line(), request.Body)
	}
	// Agent Auth builds the OAuth provider itself.
	for _, field := range []string{"authorize_url", "token_url", "authorization_params"} {
		if strings.Contains(string(request.Body), field) {
			t.Errorf("%s sent: %s", field, request.Body)
		}
	}
}

func TestLinearErrorsNeverEchoTheSecrets(t *testing.T) {
	server := serveOnce(t, 422, `{"detail":"Linear refused linear-client-secret and lin_wh_signing"}`)
	_, err := appTestClient(t, server.URL).BringLinearApp(t.Context(), "dep-1", "mda/linear-app/dep-1", linearApp(nil))
	if err == nil || strings.Contains(err.Error(), "linear-client-secret") || strings.Contains(err.Error(), "lin_wh_signing") {
		t.Fatalf("err = %v", err)
	}
	if !jsonEqual(t, field(field(server.Only(t).JSON(t), "existing"), "scopes"), `[]`) {
		t.Fatalf("scopes = %s", server.Only(t).Body)
	}
}
