package api

import (
	"encoding/json"
	"testing"
)

func TestResolveEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		path    string
		want    string
	}{
		{"absolute path", "https://api.smith.langchain.com", "/api/v1/sessions", "https://api.smith.langchain.com/api/v1/sessions"},
		{"shorthand", "https://api.smith.langchain.com", "sessions", "https://api.smith.langchain.com/api/v1/sessions"},
		{"shorthand with subpath", "https://api.smith.langchain.com", "runs/query", "https://api.smith.langchain.com/api/v1/runs/query"},
		{"shorthand with query params", "https://api.smith.langchain.com", "sessions?limit=5", "https://api.smith.langchain.com/api/v1/sessions?limit=5"},
		{"full URL https", "https://api.smith.langchain.com", "https://other.host/foo", "https://other.host/foo"},
		{"full URL http", "https://api.smith.langchain.com", "http://other.host/foo", "http://other.host/foo"},
		{"self-hosted base", "https://myhost.com", "sessions", "https://myhost.com/api/v1/sessions"},
		{"trailing slash on base", "https://api.smith.langchain.com/", "sessions", "https://api.smith.langchain.com/api/v1/sessions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEndpoint(tt.baseURL, tt.path, nil)
			if got != tt.want {
				t.Errorf("resolveEndpoint(%q, %q) = %q, want %q", tt.baseURL, tt.path, got, tt.want)
			}
		})
	}
}

func TestResolveEndpointWithSpec(t *testing.T) {
	spec := &OpenAPISpec{
		Paths: map[string]map[string]json.RawMessage{
			"/api/v1/sessions":     {"get": json.RawMessage(`{}`)},
			"/v2/sandboxes/boxes":  {"post": json.RawMessage(`{}`)},
			"/commits":             {"post": json.RawMessage(`{}`)},
			"/v2/sandboxes/widgets": {"get": json.RawMessage(`{}`)},
		},
	}
	base := "https://api.smith.langchain.com"
	tests := []struct {
		name string
		path string
		want string
	}{
		{"api/v1 shorthand still preferred", "sessions", base + "/api/v1/sessions"},
		{"v2 shorthand resolves via spec", "sandboxes/boxes", base + "/v2/sandboxes/boxes"},
		{"non-api-v1 root resolves", "commits", base + "/commits"},
		{"shorthand with query preserved", "sandboxes/boxes?foo=1", base + "/v2/sandboxes/boxes?foo=1"},
		{"unknown shorthand falls back to /api/v1", "nope/missing", base + "/api/v1/nope/missing"},
		{"absolute path untouched", "/v2/sandboxes/boxes", base + "/v2/sandboxes/boxes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEndpoint(base, tt.path, spec)
			if got != tt.want {
				t.Errorf("resolveEndpoint(%q, %q) = %q, want %q", base, tt.path, got, tt.want)
			}
		})
	}
}

func TestIsHTTPMethod(t *testing.T) {
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		if !isHTTPMethod(m) {
			t.Errorf("expected %q to be recognized as HTTP method", m)
		}
	}
	for _, m := range []string{"get", "ls", "info", "FOO", ""} {
		if isHTTPMethod(m) {
			t.Errorf("expected %q to NOT be recognized as HTTP method", m)
		}
	}
}
