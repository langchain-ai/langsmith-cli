package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func renderTestResponse(t *testing.T, resp apiResponse, args ...string) (string, error) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("format", "pretty", "")
	cmd.Flags().String("jq", "", "")
	cmd.SetArgs(args)
	require.NoError(t, cmd.ParseFlags(args))
	var out bytes.Buffer
	cmd.SetOut(&out)
	var model any
	if resp.IsJSON {
		model = resp.Body
	}
	err := structured.Render(cmd, structured.Result{Model: model, UnstructuredModel: resp}, nil)
	return out.String(), err
}

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

	c := client.New("test-key", ts.URL)
	resp, err := runRequest(c, "GET", "sessions", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	out, err := renderTestResponse(t, resp)
	require.NoError(t, err)
	if !strings.Contains(out, `"id"`) {
		t.Errorf("expected JSON output, got %q", out)
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

	c := client.New("key", ts.URL)
	resp, err := runRequest(c, "POST", "sessions", `{"name":"test"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
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

	c := client.New("key", ts.URL)
	_, err := runRequest(c, "GET", "sessions", "", []string{"X-Custom:val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRequest_FormatJSONBodyOnly(t *testing.T) {
	resp := apiResponse{
		StatusCode: 200,
		Body:       map[string]any{"ok": true},
		IsJSON:     true,
	}

	out, err := renderTestResponse(t, resp, "--format", "json")

	require.NoError(t, err)
	require.JSONEq(t, `{"ok":true}`, out)
}

func TestRunRequest_NonJSONHasNoJSONModel(t *testing.T) {
	resp := apiResponse{
		StatusCode: 200,
		Body:       "plain text",
	}

	out, err := renderTestResponse(t, resp)

	require.EqualError(t, err, "JSON model is not available")
	require.Empty(t, out)
}

func TestRunRequest_JQScalar(t *testing.T) {
	resp := apiResponse{
		StatusCode: 200,
		Body:       map[string]any{"name": "alpha"},
		IsJSON:     true,
	}

	out, err := renderTestResponse(t, resp, "--jq", ".name")

	require.NoError(t, err)
	require.Equal(t, "alpha\n", out)
}

func TestRunRequest_ReturnsErrorAfterRender(t *testing.T) {
	resp := apiResponse{
		StatusCode: 404,
		Body:       map[string]any{"detail": "not found"},
		IsJSON:     true,
	}
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("format", "pretty", "")
	cmd.Flags().String("jq", "", "")
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := structured.Render(cmd, structured.Result{
		Model:             resp.Body,
		UnstructuredModel: resp,
		ErrAfterRender:    fmt.Errorf("HTTP 404"),
	}, nil)

	require.EqualError(t, err, "HTTP 404")
	require.Contains(t, out.String(), "not found")
}

func TestRunRequest_4xxPrintsBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"detail":"not found"}`))
	}))
	defer ts.Close()

	c := client.New("key", ts.URL)
	resp, err := runRequest(c, "GET", "sessions", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	out, err := renderTestResponse(t, resp)
	require.NoError(t, err)
	if !strings.Contains(out, "not found") {
		t.Errorf("expected error body in output, got %q", out)
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

	c := client.New("key", ts.URL)
	resp, err := runRequest(c, "POST", "sessions", "@"+f.Name(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	out, err := renderTestResponse(t, resp)
	require.NoError(t, err)
	if !strings.Contains(out, "from") {
		t.Errorf("expected file body echoed, got %q", out)
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

	c := client.New("key", "https://different.host")
	resp, err := runRequest(c, "GET", ts.URL+"/custom/endpoint", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
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

	c := client.New("key", ts.URL)
	_, err := runRequest(c, "GET", "sessions", "", []string{"X-Multi:one", "X-Multi:two"})
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

	resp, err := runRequest(c, "GET", ts.URL+"/steal", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
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
