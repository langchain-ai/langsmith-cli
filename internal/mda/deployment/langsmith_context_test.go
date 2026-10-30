package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// isolateEnv keeps the developer's LangSmith config and environment out of
// resolution and of the SDK, which reads both.
func isolateEnv(t *testing.T) {
	t.Helper()
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))
	for _, key := range []string{
		"LANGSMITH_API_KEY", "LANGCHAIN_API_KEY", "LANGSMITH_ENDPOINT",
		"LANGSMITH_WORKSPACE_ID", "LANGCHAIN_WORKSPACE_ID", "LANGSMITH_TENANT_ID",
		"LANGCHAIN_TENANT_ID", "LANGSMITH_PROFILE", "LANGCHAIN_CUSTOM_HEADERS",
	} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
}

// useTerminal makes the default terminal interactive, or not, for the rest of
// the test.
func useTerminal(t *testing.T, interactive bool) {
	t.Helper()
	t.Cleanup(ui.SetDefault(ui.NewTerminal(strings.NewReader(""), &bytes.Buffer{}, ui.WithInteractive(interactive))))
}

// oauthProfileCLI is a CLI signed in with the OAuth profile "oauth-profile"
// holding accessToken, whose default workspace is workspace.
func oauthProfileCLI(t *testing.T, accessToken, workspace string) CLI {
	t.Helper()
	config := map[string]any{"profiles": map[string]any{
		"oauth-profile": map[string]any{"oauth": map[string]any{"access_token": accessToken}},
	}}
	data, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(os.Getenv("LANGSMITH_CONFIG_FILE"), data, 0o600))
	return cliWith(client.Options{
		ProfileName: "oauth-profile",
		APIURL:      "https://api.smith.langchain.com",
		WorkspaceID: workspace,
	})
}

// cliWith is a CLI whose own options are options.
func cliWith(options client.Options) CLI {
	return CLI{Options: func() (client.Options, error) { return options, nil }}
}

func TestAHostOverrideBeatsLangsmithEndpoint(t *testing.T) {
	isolateEnv(t)
	env := map[string]string{"LANGSMITH_ENDPOINT": "https://eu.api.smith.langchain.com"}

	endpoints := ResolveLangSmithEndpoints(CLI{}, env, " https://api.host.langchain.com ")

	assert.Equal(t, "https://api.host.langchain.com", endpoints.HostURL)
}

func TestABlankHostOverrideFallsBackToLangsmithEndpoint(t *testing.T) {
	isolateEnv(t)
	env := map[string]string{"LANGSMITH_ENDPOINT": "https://eu.api.smith.langchain.com"}

	endpoints := ResolveLangSmithEndpoints(CLI{}, env, "  ")

	assert.Equal(t, LangSmithEndpoints{
		HostURL:        "https://eu.api.host.langchain.com",
		PlatformAPIURL: "https://eu.api.smith.langchain.com",
	}, endpoints)
}

func TestTheProjectEndpointBeatsTheCLIAPIURL(t *testing.T) {
	isolateEnv(t)
	cli := cliWith(client.Options{APIURL: "https://langsmith.example.com"})
	env := map[string]string{"LANGSMITH_ENDPOINT": "https://eu.api.smith.langchain.com"}

	assert.Equal(t, LangSmithEndpoints{
		HostURL:        "https://eu.api.host.langchain.com",
		PlatformAPIURL: "https://eu.api.smith.langchain.com",
	}, ResolveLangSmithEndpoints(cli, env, ""))
}

func TestTheCLIAPIURLPicksTheInstallationWithoutAProjectEndpoint(t *testing.T) {
	isolateEnv(t)
	cli := cliWith(client.Options{APIURL: "https://langsmith.example.com"})

	assert.Equal(t, LangSmithEndpoints{
		HostURL:        "https://langsmith.example.com/api-host",
		PlatformAPIURL: "https://langsmith.example.com",
	}, ResolveLangSmithEndpoints(cli, map[string]string{}, ""))
}

func TestTheDefaultCLIAPIURLDoesNotPinAHostOverridesPlatform(t *testing.T) {
	isolateEnv(t)
	cli := cliWith(client.Options{APIURL: "https://api.smith.langchain.com"})

	assert.Equal(t, LangSmithEndpoints{
		HostURL:        "https://eu.api.host.langchain.com",
		PlatformAPIURL: "https://eu.api.smith.langchain.com",
	}, ResolveLangSmithEndpoints(cli, map[string]string{}, "https://eu.api.host.langchain.com"))
}

func TestTheAPIURLFlagBeatsTheProjectEndpoint(t *testing.T) {
	isolateEnv(t)
	cli := cliWith(client.Options{APIURL: "https://langsmith.example.com"})
	cli.APIURL = "https://langsmith.example.com/"
	env := map[string]string{"LANGSMITH_ENDPOINT": "https://eu.api.smith.langchain.com"}

	assert.Equal(t, LangSmithEndpoints{
		HostURL:        "https://langsmith.example.com/api-host",
		PlatformAPIURL: "https://langsmith.example.com",
	}, ResolveLangSmithEndpoints(cli, env, ""))
}

func TestAProjectAPIKeyBeatsTheCLIProfileWithoutItsWorkspace(t *testing.T) {
	isolateEnv(t)
	cli := oauthProfileCLI(t, "oauth-token", "profile-workspace")

	lsctx, err := ResolveLangSmithContext(t.Context(), cli, "/project/.env",
		map[string]string{"LANGSMITH_API_KEY": "project-key"}, "", "", WorkspaceFlagHint)

	require.NoError(t, err)
	assert.Equal(t, client.Options{
		APIKey: "project-key",
		APIURL: "https://api.smith.langchain.com",
	}, lsctx.Options)
}

func TestTheAPIKeyFlagBeatsAProjectAPIKey(t *testing.T) {
	isolateEnv(t)
	cli := cliWith(client.Options{APIKey: "flag-key", APIURL: "https://api.smith.langchain.com", WorkspaceID: "cli-workspace"})
	cli.APIKey = "flag-key"

	lsctx, err := ResolveLangSmithContext(t.Context(), cli, "/project/.env",
		map[string]string{"LANGSMITH_API_KEY": "project-key"}, "", "", WorkspaceFlagHint)

	require.NoError(t, err)
	assert.Equal(t, client.Options{
		APIKey:      "flag-key",
		APIURL:      "https://api.smith.langchain.com",
		WorkspaceID: "cli-workspace",
	}, lsctx.Options)
}

func TestTheAPIKeyFlagIsNotSentToAProjectEndpointElsewhere(t *testing.T) {
	isolateEnv(t)
	cli := cliWith(client.Options{APIKey: "flag-key", APIURL: "https://api.smith.langchain.com"})
	cli.APIKey = "flag-key"

	lsctx, err := ResolveLangSmithContext(t.Context(), cli, "/project/.env", map[string]string{
		"LANGSMITH_ENDPOINT": "https://langsmith.attacker.example",
		"LANGSMITH_API_KEY":  "project-key",
	}, "", "", WorkspaceFlagHint)

	require.NoError(t, err)
	assert.Equal(t, client.Options{
		APIKey: "project-key",
		APIURL: "https://langsmith.attacker.example",
	}, lsctx.Options)
}

func TestTheCLICredentialIsUsedWithoutAProjectAPIKey(t *testing.T) {
	isolateEnv(t)
	cli := oauthProfileCLI(t, "oauth-token", "profile-workspace")

	lsctx, err := ResolveLangSmithContext(t.Context(), cli, "/project/.env", map[string]string{}, "", "", WorkspaceFlagHint)

	require.NoError(t, err)
	assert.Equal(t, client.Options{
		ProfileName: "oauth-profile",
		APIURL:      "https://api.smith.langchain.com",
		WorkspaceID: "profile-workspace",
	}, lsctx.Options)
}

func TestTheCLICredentialIsNotSentToAProjectEndpointElsewhere(t *testing.T) {
	isolateEnv(t)
	useTerminal(t, false)
	cli := oauthProfileCLI(t, "oauth-token", "profile-workspace")
	cli.Login = func(context.Context, string) (client.Options, error) {
		t.Fatal("signed in without a terminal")
		return client.Options{}, nil
	}

	_, err := ResolveLangSmithContext(t.Context(), cli, "/project/.env",
		map[string]string{"LANGSMITH_ENDPOINT": "https://eu.api.smith.langchain.com"}, "", "", WorkspaceFlagHint)

	assert.EqualError(t, err, "The LangSmith CLI's credentials are for https://api.smith.langchain.com, not "+
		"https://eu.api.smith.langchain.com, so they are not sent there. Run `langsmith auth login --api-url "+
		"https://eu.api.smith.langchain.com`, or set LANGSMITH_API_KEY in the environment or in /project/.env")
	assert.Equal(t, errs.Authentication, errs.CategoryOf(err))
}

func TestAProjectEndpointElsewhereSignsInThereOnATerminal(t *testing.T) {
	isolateEnv(t)
	useTerminal(t, true)
	cli := oauthProfileCLI(t, "oauth-token", "profile-workspace")
	var signedInAt string
	cli.Login = func(_ context.Context, platformAPIURL string) (client.Options, error) {
		signedInAt = platformAPIURL
		return client.Options{ProfileName: "oauth-profile"}, nil
	}

	lsctx, err := ResolveLangSmithContext(t.Context(), cli, "/project/.env",
		map[string]string{"LANGSMITH_ENDPOINT": "https://eu.api.smith.langchain.com"}, "", "", WorkspaceFlagHint)

	require.NoError(t, err)
	assert.Equal(t, "https://eu.api.smith.langchain.com", signedInAt)
	assert.Equal(t, client.Options{ProfileName: "oauth-profile", APIURL: "https://eu.api.smith.langchain.com"}, lsctx.Options)
}

func TestABrokenCLIConfigOnlyFailsWhenItsCredentialIsNeeded(t *testing.T) {
	isolateEnv(t)
	broken := CLI{Options: func() (client.Options, error) {
		return client.Options{}, errors.New("config is not valid JSON")
	}}

	lsctx, err := ResolveLangSmithContext(t.Context(), broken, "/project/.env",
		map[string]string{"LANGSMITH_API_KEY": "project-key"}, "", "", WorkspaceFlagHint)
	require.NoError(t, err)
	assert.Equal(t, client.Options{APIKey: "project-key", APIURL: "https://api.smith.langchain.com"}, lsctx.Options)

	_, err = ResolveLangSmithContext(t.Context(), broken, "/project/.env", map[string]string{}, "", "", WorkspaceFlagHint)
	assert.EqualError(t, err, "config is not valid JSON")
}

func TestARemovedAPIKeyNameIsAnError(t *testing.T) {
	isolateEnv(t)
	cli := oauthProfileCLI(t, "oauth-token", "")

	_, err := ResolveCredential(t.Context(), cli, "/project/.env", map[string]string{"LANGCHAIN_API_KEY": "old"}, DefaultHostURL)

	assert.EqualError(t, err, "LANGCHAIN_API_KEY is no longer supported; set LANGSMITH_API_KEY instead (same value, renamed)")
}

func TestNoCredentialWithoutATerminalIsAnAuthenticationError(t *testing.T) {
	isolateEnv(t)
	useTerminal(t, false)
	login := func(context.Context, string) (client.Options, error) {
		t.Fatal("signed in without a terminal")
		return client.Options{}, nil
	}

	_, err := ResolveCredential(t.Context(), CLI{Login: login}, "/project/.env", map[string]string{}, DefaultHostURL)

	assert.EqualError(t, err, "No LangSmith credentials found. Run `langsmith auth login`, or set LANGSMITH_API_KEY in the environment or in /project/.env")
	assert.Equal(t, errs.Authentication, errs.CategoryOf(err))
}

func TestNoCredentialOnATerminalSignsInAtThePlatformAPI(t *testing.T) {
	isolateEnv(t)
	useTerminal(t, true)
	var signedInAt string
	login := func(_ context.Context, platformAPIURL string) (client.Options, error) {
		signedInAt = platformAPIURL
		return client.Options{ProfileName: "default", WorkspaceID: "login-workspace"}, nil
	}

	lsctx, err := ResolveLangSmithContext(t.Context(), CLI{Login: login}, "/project/.env",
		map[string]string{"LANGSMITH_ENDPOINT": "https://eu.api.smith.langchain.com"}, "", "", WorkspaceFlagHint)

	require.NoError(t, err)
	assert.Equal(t, "https://eu.api.smith.langchain.com", signedInAt)
	assert.Equal(t, client.Options{
		ProfileName: "default",
		APIURL:      "https://eu.api.smith.langchain.com",
		WorkspaceID: "login-workspace",
	}, lsctx.Options)
}

func TestAFailedSignInIsAnAuthenticationError(t *testing.T) {
	isolateEnv(t)
	useTerminal(t, true)
	login := func(context.Context, string) (client.Options, error) {
		return client.Options{}, errors.New("authorization denied")
	}

	_, err := ResolveCredential(t.Context(), CLI{Login: login}, "/project/.env", map[string]string{}, DefaultHostURL)

	assert.EqualError(t, err, "authorization denied")
	assert.Equal(t, errs.Authentication, errs.CategoryOf(err))
}

func TestTheWorkspaceFlagBeatsTheEnvAndTheCLI(t *testing.T) {
	isolateEnv(t)
	cli := oauthProfileCLI(t, "oauth-token", "profile-workspace")
	env := map[string]string{"LANGSMITH_WORKSPACE_ID": "env-workspace"}

	fromFlag, err := ResolveLangSmithContext(t.Context(), cli, "/project/.env", env, "", " flag-workspace ", WorkspaceFlagHint)
	require.NoError(t, err)
	fromEnv, err := ResolveLangSmithContext(t.Context(), cli, "/project/.env", env, "", "", WorkspaceFlagHint)
	require.NoError(t, err)

	assert.Equal(t, "flag-workspace", fromFlag.WorkspaceID())
	assert.Equal(t, "env-workspace", fromEnv.WorkspaceID())
}

func TestARemovedWorkspaceNameIsAnError(t *testing.T) {
	isolateEnv(t)
	cli := oauthProfileCLI(t, "oauth-token", "profile-workspace")

	_, err := ResolveLangSmithContext(t.Context(), cli, "/project/.env",
		map[string]string{"LANGSMITH_TENANT_ID": "old"}, "", "", WorkspaceFlagHint)

	assert.EqualError(t, err, "LANGSMITH_TENANT_ID is no longer supported; set LANGSMITH_WORKSPACE_ID or pass --workspace-id instead")
}

// recorded is a request a stub received.
type recorded struct {
	URI    string
	Header http.Header
}

// stub serves replies in order, one per request, recording each request.
type stub struct {
	URL string

	mu       sync.Mutex
	replies  []string
	received []recorded
}

func serve(t *testing.T, replies ...string) *stub {
	t.Helper()
	s := &stub{replies: replies}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.received = append(s.received, recorded{URI: r.URL.RequestURI(), Header: r.Header.Clone()})
		index := len(s.received) - 1
		s.mu.Unlock()
		if index >= len(s.replies) {
			t.Errorf("unexpected request %d: %s %s", index+1, r.Method, r.URL)
			w.WriteHeader(http.StatusTeapot)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(s.replies[index]))
	}))
	t.Cleanup(server.Close)
	s.URL = server.URL
	return s
}

func (s *stub) Requests() []recorded {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recorded(nil), s.received...)
}

func TestTheContextClientAuthenticatesWithTheCLIOAuthProfile(t *testing.T) {
	isolateEnv(t)
	server := serve(t, `{"resources":[{"name":"agent","id":"deployment"}]}`)
	signedIn, err := oauthProfileCLI(t, "oauth-token", "profile-workspace").Options()
	require.NoError(t, err)
	signedIn.APIURL = server.URL
	cli := cliWith(signedIn)
	lsctx, err := ResolveLangSmithContext(t.Context(), cli, "/project/.env", map[string]string{}, server.URL, "", WorkspaceFlagHint)
	require.NoError(t, err)
	c, err := lsctx.Client()
	require.NoError(t, err)

	deployment, err := FindTargetDeployment(t.Context(), c, "agent", nil)

	require.NoError(t, err)
	assert.Equal(t, "deployment", deployment["id"])
	request := server.Requests()[0]
	assert.Equal(t, "Bearer oauth-token", request.Header.Get("Authorization"))
	assert.Equal(t, "", request.Header.Get("X-Api-Key"))
	assert.Equal(t, "profile-workspace", request.Header.Get("X-Tenant-Id"))
}
