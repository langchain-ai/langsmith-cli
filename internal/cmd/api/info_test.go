package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newDetailedSpecServer(t *testing.T) *httptest.Server {
	t.Helper()
	spec := map[string]any{
		"openapi": "3.1.0",
		"paths": map[string]any{
			"/api/v1/sessions": map[string]any{
				"get": map[string]any{
					"summary": "List sessions",
					"tags":    []any{"tracer-sessions"},
					"parameters": []any{
						map[string]any{
							"name": "limit", "in": "query", "required": false,
							"schema":      map[string]any{"type": "integer"},
							"description": "Max results",
						},
					},
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"type": "array"},
								},
							},
						},
					},
				},
			},
			"/api/v1/runs/query": map[string]any{
				"post": map[string]any{
					"summary": "Query runs",
					"tags":    []any{"run"},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"session_id": map[string]any{"type": "string"},
									},
									"required": []any{"session_id"},
								},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"type": "object"},
								},
							},
						},
					},
				},
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

func TestInfoCmd_JSON(t *testing.T) {
	ts := newDetailedSpecServer(t)
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--format=json", "api", "info", "--api-url", ts.URL, "GET", "/api/v1/sessions"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var detail EndpointDetail
	if err := json.Unmarshal(out.Bytes(), &detail); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if detail.Method != "GET" {
		t.Errorf("expected method GET, got %q", detail.Method)
	}
	if detail.Path != "/api/v1/sessions" {
		t.Errorf("expected path /api/v1/sessions, got %q", detail.Path)
	}
	if len(detail.Parameters) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(detail.Parameters))
	}
	if detail.Parameters[0].Name != "limit" {
		t.Errorf("expected param name 'limit', got %q", detail.Parameters[0].Name)
	}
}

func TestInfoCmd_Shorthand(t *testing.T) {
	ts := newDetailedSpecServer(t)
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--format=json", "api", "info", "--api-url", ts.URL, "GET", "sessions"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var detail EndpointDetail
	if err := json.Unmarshal(out.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Path != "/api/v1/sessions" {
		t.Errorf("expected resolved path /api/v1/sessions, got %q", detail.Path)
	}
}

func TestInfoCmd_WithRequestBody(t *testing.T) {
	ts := newDetailedSpecServer(t)
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--format=json", "api", "info", "--api-url", ts.URL, "POST", "runs/query"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var detail EndpointDetail
	if err := json.Unmarshal(out.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.RequestBody == nil {
		t.Fatal("expected request_body to be non-nil")
	}
}

func TestInfoCmd_NotFound(t *testing.T) {
	ts := newDetailedSpecServer(t)
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"api", "info", "--api-url", ts.URL, "GET", "nonexistent"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent endpoint")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %q", err.Error())
	}
}
