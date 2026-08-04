package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// selfHostedDiscoveryServer serves the RFC 8414 metadata document under /api
// (as self-hosted LangSmith does) and returns an HTML 200 at the bare-root
// .well-known path, mimicking the SPA catch-all that must not be mistaken for
// real metadata.
func selfHostedDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/.well-known/oauth-authorization-server":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                        srv.URL + "/api",
				"device_authorization_endpoint": srv.URL + "/api/oauth/device/code",
				"token_endpoint":                srv.URL + "/api/oauth/token",
				"registration_endpoint":         srv.URL + "/api/oauth/register",
				"protected_resources_supported": []string{srv.URL + "/api", srv.URL + "/api/mcp"},
			})
		case "/.well-known/oauth-authorization-server":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<!doctype html><html><body>app</body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDiscoverOAuth_SelfHostedUnderApi(t *testing.T) {
	srv := selfHostedDiscoveryServer(t)

	meta, err := DiscoverOAuth(context.Background(), srv.URL+"/api")
	if err != nil {
		t.Fatalf("DiscoverOAuth: %v", err)
	}
	if got, want := meta.TokenEndpoint, srv.URL+"/api/oauth/token"; got != want {
		t.Errorf("TokenEndpoint = %q, want %q", got, want)
	}
	if got, want := meta.DeviceAuthorizationEndpoint, srv.URL+"/api/oauth/device/code"; got != want {
		t.Errorf("DeviceAuthorizationEndpoint = %q, want %q", got, want)
	}
	if got, want := meta.RegistrationEndpoint, srv.URL+"/api/oauth/register"; got != want {
		t.Errorf("RegistrationEndpoint = %q, want %q", got, want)
	}
	if got, want := meta.Resource, srv.URL+"/api"; got != want {
		t.Errorf("Resource = %q, want %q (issuer)", got, want)
	}
}

// A self-hosted user who passes the bare origin must still resolve: the bare
// .well-known returns HTML (rejected), and discovery falls back to /api.
func TestDiscoverOAuth_BareOriginFallsBackToApi(t *testing.T) {
	srv := selfHostedDiscoveryServer(t)

	meta, err := DiscoverOAuth(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("DiscoverOAuth: %v", err)
	}
	if got, want := meta.TokenEndpoint, srv.URL+"/api/oauth/token"; got != want {
		t.Errorf("TokenEndpoint = %q, want %q", got, want)
	}
	if got, want := meta.Resource, srv.URL+"/api"; got != want {
		t.Errorf("Resource = %q, want %q", got, want)
	}
}

func TestDiscoverOAuth_SaaSAtRoot(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                        srv.URL,
				"device_authorization_endpoint": srv.URL + "/oauth/device/code",
				"token_endpoint":                srv.URL + "/oauth/token",
				"registration_endpoint":         srv.URL + "/oauth/register",
				"protected_resources_supported": []string{srv.URL, srv.URL + "/mcp"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	meta, err := DiscoverOAuth(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("DiscoverOAuth: %v", err)
	}
	if got, want := meta.TokenEndpoint, srv.URL+"/oauth/token"; got != want {
		t.Errorf("TokenEndpoint = %q, want %q", got, want)
	}
	if got, want := meta.Resource, srv.URL; got != want {
		t.Errorf("Resource = %q, want %q", got, want)
	}
}

// When no metadata document exists (older backend), discovery returns an error
// so callers can fall back to legacy path construction.
func TestDiscoverOAuth_NoMetadataReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	if _, err := DiscoverOAuth(context.Background(), srv.URL+"/api"); err == nil {
		t.Fatal("expected error when no metadata document is served")
	}
}

// ResolveOAuth falls back to legacy <base>/oauth/* construction when the backend
// serves no metadata document, so older self-hosted backends keep working when
// apiURL points at the API root.
func TestResolveOAuth_FallsBackToLegacyPaths(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	meta := ResolveOAuth(context.Background(), srv.URL+"/api")
	if got, want := meta.TokenEndpoint, srv.URL+"/api/oauth/token"; got != want {
		t.Errorf("TokenEndpoint = %q, want legacy %q", got, want)
	}
	if got, want := meta.DeviceAuthorizationEndpoint, srv.URL+"/api/oauth/device/code"; got != want {
		t.Errorf("DeviceAuthorizationEndpoint = %q, want legacy %q", got, want)
	}
	if got, want := meta.Resource, srv.URL+"/api"; got != want {
		t.Errorf("Resource = %q, want %q", got, want)
	}
}

// ResolveOAuth prefers discovery: a bare origin resolves to the discovered /api
// endpoints even though the legacy fallback would have produced bare-root paths.
func TestResolveOAuth_PrefersDiscovery(t *testing.T) {
	srv := selfHostedDiscoveryServer(t)

	meta := ResolveOAuth(context.Background(), srv.URL)
	if got, want := meta.TokenEndpoint, srv.URL+"/api/oauth/token"; got != want {
		t.Errorf("TokenEndpoint = %q, want discovered %q", got, want)
	}
}
