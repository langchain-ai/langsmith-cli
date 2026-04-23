package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/client"
)

func TestRunRequest_GET(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sessions" {
			t.Errorf("expected /api/v1/sessions, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key=test-key, got %q", r.Header.Get("x-api-key"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[{"id":"s1"}]`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	c := client.New("test-key", ts.URL)
	code, err := runRequest(c, "GET", "sessions", "", nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Errorf("expected status 200, got %d", code)
	}
	if !strings.Contains(out.String(), `"id"`) {
		t.Errorf("expected JSON output, got %q", out.String())
	}
}

func TestRunRequest_POSTWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var data map[string]any
		_ = json.Unmarshal(body, &data)
		if data["name"] != "test" {
			t.Errorf("expected name=test, got %v", data["name"])
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"new"}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	c := client.New("key", ts.URL)
	code, err := runRequest(c, "POST", "sessions", `{"name":"test"}`, nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 201 {
		t.Errorf("expected 201, got %d", code)
	}
}

func TestRunRequest_ExtraHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "val" {
			t.Errorf("expected X-Custom=val, got %q", r.Header.Get("X-Custom"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	c := client.New("key", ts.URL)
	_, err := runRequest(c, "GET", "sessions", "", []string{"X-Custom:val"}, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRequest_Include(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "abc")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	c := client.New("key", ts.URL)
	_, err := runRequest(c, "GET", "sessions", "", nil, true, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "200") {
		t.Errorf("expected status line, got %q", out.String())
	}
	if !strings.Contains(out.String(), "X-Request-Id") {
		t.Errorf("expected header in output, got %q", out.String())
	}
}

func TestRunRequest_4xxPrintsBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"detail":"not found"}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	c := client.New("key", ts.URL)
	code, err := runRequest(c, "GET", "sessions", "", nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 404 {
		t.Errorf("expected 404, got %d", code)
	}
	if !strings.Contains(out.String(), "not found") {
		t.Errorf("expected error body in output, got %q", out.String())
	}
}

func TestRunRequest_BodyFromFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(200)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	f, _ := os.CreateTemp(t.TempDir(), "body-*.json")
	_, _ = f.WriteString(`{"from":"file"}`)
	f.Close()

	var out bytes.Buffer
	c := client.New("key", ts.URL)
	code, err := runRequest(c, "POST", "sessions", "@"+f.Name(), nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Errorf("expected 200, got %d", code)
	}
	if !strings.Contains(out.String(), "from") {
		t.Errorf("expected file body echoed, got %q", out.String())
	}
}

func TestRunRequest_FullURLDifferentHost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/endpoint" {
			t.Errorf("expected /custom/endpoint, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "" {
			t.Errorf("expected no x-api-key for external host, got %q", r.Header.Get("x-api-key"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"full_url":true}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	c := client.New("key", "https://different.host")
	code, err := runRequest(c, "GET", ts.URL+"/custom/endpoint", "", nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Errorf("expected 200, got %d", code)
	}
}

func TestRunRequest_MultiValueHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vals := r.Header.Values("X-Multi")
		if len(vals) != 2 || vals[0] != "one" || vals[1] != "two" {
			t.Errorf("expected X-Multi=[one, two], got %v", vals)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	c := client.New("key", ts.URL)
	_, err := runRequest(c, "GET", "sessions", "", []string{"X-Multi:one", "X-Multi:two"}, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRequest_PrefixConfusionAttack(t *testing.T) {
	// Verify that a malicious URL sharing a string prefix with apiURL
	// (e.g. https://api.host.evil.com vs https://api.host) does NOT
	// receive the API key.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "" {
			t.Errorf("API key leaked to prefix-confused host, got %q", r.Header.Get("x-api-key"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	// apiURL is a prefix of the malicious URL at the string level
	// e.g. apiURL="http://127.0.0.1:PORT" and fullURL="http://127.0.0.1:PORT.evil.com/steal"
	// We simulate by setting apiURL to a substring of the test server URL.
	// Use ts.URL minus the last char as apiURL so ts.URL starts with apiURL
	// but is a different host.
	apiURL := ts.URL[:len(ts.URL)-1] // e.g. "http://127.0.0.1:5432" → "http://127.0.0.1:543"
	c := client.New("secret-key", apiURL)

	var out bytes.Buffer
	code, err := runRequest(c, "GET", ts.URL+"/steal", "", nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Errorf("expected 200, got %d", code)
	}
}

func TestIsSameHost(t *testing.T) {
	tests := []struct {
		name    string
		fullURL string
		baseURL string
		want    bool
	}{
		{"exact match", "https://api.example.com", "https://api.example.com", true},
		{"with path", "https://api.example.com/api/v1/sessions", "https://api.example.com", true},
		{"with query", "https://api.example.com?foo=bar", "https://api.example.com", true},
		{"prefix attack", "https://api.example.com.evil.com/steal", "https://api.example.com", false},
		{"prefix attack with dot", "https://api.example.comevil.com", "https://api.example.com", false},
		{"different host", "https://other.host/path", "https://api.example.com", false},
		{"no prefix match", "https://different.com", "https://api.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSameHost(tt.fullURL, tt.baseURL); got != tt.want {
				t.Errorf("isSameHost(%q, %q) = %v, want %v", tt.fullURL, tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestResolveBody_InlineJSON(t *testing.T) {
	r, err := resolveBody(`{"key":"val"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := io.ReadAll(r)
	if string(data) != `{"key":"val"}` {
		t.Errorf("unexpected body: %s", data)
	}
}

func TestResolveBody_Empty(t *testing.T) {
	r, err := resolveBody("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r != nil {
		t.Error("expected nil reader for empty body")
	}
}

func TestResolveBody_FileNotFound(t *testing.T) {
	_, err := resolveBody("@/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
