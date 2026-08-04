package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// selfHostedDiscoveryServer serves metadata under /api and, like the real SPA
// catch-all, an HTML 200 at the bare-root .well-known path.
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

// A bare origin must still resolve: the root .well-known returns HTML, so
// discovery has to reject it and fall back to /api.
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

// No metadata document must surface an error so callers can fall back.
func TestDiscoverOAuth_NoMetadataReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	if _, err := DiscoverOAuth(context.Background(), srv.URL+"/api"); err == nil {
		t.Fatal("expected error when no metadata document is served")
	}
}

// Without a metadata document, ResolveOAuth keeps the legacy <base>/oauth/*
// behavior.
func TestResolveOAuth_FallsBackToLegacyPaths(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	meta, err := ResolveOAuth(context.Background(), srv.URL+"/api")
	if err != nil {
		t.Fatalf("ResolveOAuth: %v", err)
	}
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

// Discovery wins over the fallback, which would have produced root paths.
func TestResolveOAuth_PrefersDiscovery(t *testing.T) {
	srv := selfHostedDiscoveryServer(t)

	meta, err := ResolveOAuth(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ResolveOAuth: %v", err)
	}
	if got, want := meta.TokenEndpoint, srv.URL+"/api/oauth/token"; got != want {
		t.Errorf("TokenEndpoint = %q, want discovered %q", got, want)
	}
}

// metadataServer serves one metadata document at the root .well-known path.
func metadataServer(t *testing.T, doc func(base string) map[string]any) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/oauth-authorization-server" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc(srv.URL))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Credentials are POSTed to these endpoints, so a document claiming an issuer
// other than the host we probed must not be trusted.
func TestDiscoverOAuth_RejectsIssuerMismatch(t *testing.T) {
	srv := metadataServer(t, func(base string) map[string]any {
		return map[string]any{
			"issuer":                        "https://evil.example.com",
			"device_authorization_endpoint": base + "/oauth/device/code",
			"token_endpoint":                base + "/oauth/token",
		}
	})

	if _, err := DiscoverOAuth(context.Background(), srv.URL); err == nil {
		t.Fatal("expected issuer mismatch to be rejected")
	}
}

// An endpoint pointing off-host would exfiltrate the refresh token.
func TestDiscoverOAuth_RejectsForeignEndpointHost(t *testing.T) {
	srv := metadataServer(t, func(base string) map[string]any {
		return map[string]any{
			"issuer":                        base,
			"device_authorization_endpoint": base + "/oauth/device/code",
			"token_endpoint":                "https://evil.example.com/oauth/token",
		}
	})

	if _, err := DiscoverOAuth(context.Background(), srv.URL); err == nil {
		t.Fatal("expected foreign token endpoint to be rejected")
	}
}

// A 5xx is a transient failure, not proof the deployment lacks metadata, so
// falling back to legacy paths would send credentials to the wrong place.
func TestResolveOAuth_SurfacesTransientErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer srv.Close()

	if _, err := ResolveOAuth(context.Background(), srv.URL+"/api"); err == nil {
		t.Fatal("expected 5xx during discovery to surface as an error")
	}
}
