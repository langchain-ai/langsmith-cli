package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestSpecServer(t *testing.T) *httptest.Server {
	t.Helper()
	spec := map[string]any{
		"openapi": "3.1.0",
		"paths": map[string]any{
			"/api/v1/sessions": map[string]any{
				"get":  map[string]any{"summary": "List sessions", "tags": []any{"tracer-sessions"}},
				"post": map[string]any{"summary": "Create session", "tags": []any{"tracer-sessions"}},
			},
			"/api/v1/datasets": map[string]any{
				"get":  map[string]any{"summary": "List datasets", "tags": []any{"datasets"}},
				"post": map[string]any{"summary": "Create dataset", "tags": []any{"datasets"}},
			},
			"/api/v1/runs/query": map[string]any{
				"post": map[string]any{"summary": "Query runs", "tags": []any{"run"}},
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			_ = json.NewEncoder(w).Encode(spec)
			return
		}
		http.NotFound(w, r)
	}))
}

func TestLsCmd_JSON(t *testing.T) {
	ts := newTestSpecServer(t)
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--format=json", "api", "ls", "--api-url", ts.URL, "--refresh"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var endpoints []Endpoint
	if err := json.Unmarshal(out.Bytes(), &endpoints); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out.String())
	}
	if len(endpoints) != 5 {
		t.Errorf("expected 5 endpoints, got %d", len(endpoints))
	}
}

func TestLsCmd_FilterByTag(t *testing.T) {
	ts := newTestSpecServer(t)
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--format=json", "api", "ls", "--api-url", ts.URL, "--tag", "datasets", "--refresh"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var endpoints []Endpoint
	if err := json.Unmarshal(out.Bytes(), &endpoints); err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 2 {
		t.Errorf("expected 2 dataset endpoints, got %d", len(endpoints))
	}
	for _, e := range endpoints {
		if e.Tag != "datasets" {
			t.Errorf("expected tag=datasets, got %q", e.Tag)
		}
	}
}

func TestLsCmd_Search(t *testing.T) {
	ts := newTestSpecServer(t)
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--format=json", "api", "ls", "--api-url", ts.URL, "--search", "query", "--refresh"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var endpoints []Endpoint
	if err := json.Unmarshal(out.Bytes(), &endpoints); err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 {
		t.Errorf("expected 1 match for 'query', got %d", len(endpoints))
	}
}

func TestLsCmd_JQ(t *testing.T) {
	ts := newTestSpecServer(t)
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"api", "ls", "--api-url", ts.URL, "--jq", ".[0].method", "--refresh"})

	require.NoError(t, root.Execute())
	require.Equal(t, "GET", strings.TrimSpace(out.String()))
}

func TestLsCmd_Pretty(t *testing.T) {
	ts := newTestSpecServer(t)
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"api", "ls", "--api-url", ts.URL, "--format", "pretty", "--refresh"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "METHOD") {
		t.Errorf("expected table header with METHOD, got %q", output)
	}
	if !strings.Contains(output, "/api/v1/sessions") {
		t.Errorf("expected endpoint path in output, got %q", output)
	}
}
