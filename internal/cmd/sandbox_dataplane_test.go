package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDataplaneURL_TrailingSlash(t *testing.T) {
	tests := []struct {
		base     string
		path     string
		expected string
	}{
		{"https://example.com/sandbox/", "/execute", "https://example.com/sandbox/execute"},
		{"https://example.com/sandbox", "/execute", "https://example.com/sandbox/execute"},
		{"https://example.com/sandbox///", "/execute", "https://example.com/sandbox/execute"},
		{"https://example.com", "/upload?path=/root/.ssh", "https://example.com/upload?path=/root/.ssh"},
	}
	for _, tc := range tests {
		got := dataplaneURL(tc.base, tc.path)
		if got != tc.expected {
			t.Errorf("dataplaneURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.expected)
		}
	}
}

func TestDataplaneWSURL(t *testing.T) {
	tests := []struct {
		base     string
		path     string
		expected string
	}{
		{"https://example.com/sandbox", "/execute/ws", "wss://example.com/sandbox/execute/ws"},
		{"http://example.com/sandbox", "/execute/ws", "ws://example.com/sandbox/execute/ws"},
		{"https://example.com/sandbox/", "/execute/ws", "wss://example.com/sandbox/execute/ws"},
		{"http://localhost:8080/sb-123/", "/tunnel", "ws://localhost:8080/sb-123/tunnel"},
	}
	for _, tc := range tests {
		got := dataplaneWSURL(tc.base, tc.path)
		if got != tc.expected {
			t.Errorf("dataplaneWSURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.expected)
		}
	}
}

func TestSandboxAuthHeaders_OAuthFallback(t *testing.T) {
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	defer func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
	}()
	flagAPIKey = ""
	flagAPIURL = ""
	flagProfile = ""

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "prod",
  "profiles": {
    "prod": {
      "api_url": "https://profile.example.com",
      "workspace_id": "workspace-123",
      "oauth": {
        "access_token": "test-access-token"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	headers, err := sandboxAuthHeaders()
	if err != nil {
		t.Fatalf("sandboxAuthHeaders returned error: %v", err)
	}
	if got := headers["Authorization"]; got != "Bearer test-access-token" {
		t.Fatalf("expected OAuth Authorization header, got %q", got)
	}
	if got := headers["X-Api-Key"]; got != "" {
		t.Fatalf("expected no API key header, got %q", got)
	}
	if got := headers["X-Tenant-Id"]; got != "workspace-123" {
		t.Fatalf("expected workspace header, got %q", got)
	}
}

func TestSandboxAuthHeaders_RefreshesOAuthProfile(t *testing.T) {
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	defer func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
	}()
	flagAPIKey = ""
	flagAPIURL = ""
	flagProfile = ""

	refreshCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Fatalf("expected /oauth/token, got %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("grant_type"); got != "refresh_token" {
			t.Fatalf("expected refresh_token grant, got %q", got)
		}
		if got := r.FormValue("refresh_token"); got != "old-refresh-token" {
			t.Fatalf("expected old refresh token, got %q", got)
		}
		refreshCalls++
		_ = json.NewEncoder(w).Encode(oauthTokenResponse{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			ExpiresIn:    3600,
		})
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "prod",
  "profiles": {
    "prod": {
      "api_url": "`+srv.URL+`",
      "workspace_id": "workspace-123",
      "oauth": {
        "access_token": "old-access-token",
        "refresh_token": "old-refresh-token",
        "expires_at": "2000-01-01T00:00:00Z"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	headers, err := sandboxAuthHeaders()
	if err != nil {
		t.Fatalf("sandboxAuthHeaders returned error: %v", err)
	}
	if refreshCalls != 1 {
		t.Fatalf("expected one refresh call, got %d", refreshCalls)
	}
	if got := headers["Authorization"]; got != "Bearer new-access-token" {
		t.Fatalf("expected refreshed Authorization header, got %q", got)
	}
	if got := headers["X-Tenant-Id"]; got != "workspace-123" {
		t.Fatalf("expected workspace header, got %q", got)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(saved, []byte("new-refresh-token")) {
		t.Fatal("expected refreshed token response to be saved")
	}
}
