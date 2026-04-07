package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/cache"
)

func TestLoadSpec_FromServer(t *testing.T) {
	spec := map[string]any{
		"openapi": "3.1.0",
		"paths": map[string]any{
			"/api/v1/sessions": map[string]any{
				"get": map[string]any{
					"summary": "List sessions",
					"tags":    []any{"tracer-sessions"},
				},
			},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi.json" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(spec)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	result, err := loadSpec(ts.URL, cacheDir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OpenAPI != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %q", result.OpenAPI)
	}
	if len(result.Paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(result.Paths))
	}
}

func TestLoadSpec_UsesCache(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"openapi": "3.1.0",
			"paths":   map[string]any{},
		})
	}))
	defer ts.Close()

	cacheDir := t.TempDir()

	_, err := loadSpec(ts.URL, cacheDir, false)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 server call, got %d", callCount)
	}

	_, err = loadSpec(ts.URL, cacheDir, false)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected cache hit (still 1 call), got %d calls", callCount)
	}
}

func TestLoadSpec_RefreshBypassesCache(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"openapi": "3.1.0",
			"paths":   map[string]any{},
		})
	}))
	defer ts.Close()

	cacheDir := t.TempDir()

	_, _ = loadSpec(ts.URL, cacheDir, false)
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	_, _ = loadSpec(ts.URL, cacheDir, true)
	if callCount != 2 {
		t.Errorf("expected 2 calls after refresh, got %d", callCount)
	}
}

func TestLoadSpec_ExpiredCache(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"openapi": "3.1.0",
			"paths":   map[string]any{},
		})
	}))
	defer ts.Close()

	cacheDir := t.TempDir()

	_, _ = loadSpec(ts.URL, cacheDir, false)
	cachePath := cache.PathForKey(cacheDir, "openapi", ts.URL)
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(cachePath, old, old); err != nil {
		t.Fatal(err)
	}

	_, _ = loadSpec(ts.URL, cacheDir, false)
	if callCount != 2 {
		t.Errorf("expected 2 calls after TTL expiry, got %d", callCount)
	}
}

func TestEndpoints(t *testing.T) {
	spec := &OpenAPISpec{
		Paths: map[string]map[string]json.RawMessage{
			"/api/v1/sessions": {
				"get":  json.RawMessage(`{"summary": "List sessions", "tags": ["tracer-sessions"]}`),
				"post": json.RawMessage(`{"summary": "Create session", "tags": ["tracer-sessions"]}`),
			},
			"/api/v1/datasets": {
				"get": json.RawMessage(`{"summary": "List datasets", "tags": ["datasets"]}`),
			},
		},
	}
	endpoints := spec.Endpoints()
	if len(endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(endpoints))
	}
	// Should be sorted by path then method
	if endpoints[0].Path != "/api/v1/datasets" {
		t.Errorf("expected first endpoint /api/v1/datasets, got %s", endpoints[0].Path)
	}
}

func TestLookupEndpoint(t *testing.T) {
	spec := &OpenAPISpec{
		Paths: map[string]map[string]json.RawMessage{
			"/api/v1/sessions": {
				"get": json.RawMessage(`{
					"summary": "List sessions",
					"tags": ["tracer-sessions"],
					"parameters": [
						{"name": "limit", "in": "query", "required": false, "schema": {"type": "integer"}, "description": "Max results"}
					],
					"responses": {
						"200": {"content": {"application/json": {"schema": {"type": "array"}}}}
					}
				}`),
			},
		},
	}

	// Absolute path
	detail, err := spec.LookupEndpoint("GET", "/api/v1/sessions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Method != "GET" {
		t.Errorf("expected GET, got %s", detail.Method)
	}
	if len(detail.Parameters) != 1 {
		t.Errorf("expected 1 param, got %d", len(detail.Parameters))
	}
	if detail.Parameters[0].Name != "limit" {
		t.Errorf("expected param name 'limit', got %q", detail.Parameters[0].Name)
	}

	// Shorthand path
	detail2, err := spec.LookupEndpoint("GET", "sessions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail2.Path != "/api/v1/sessions" {
		t.Errorf("expected /api/v1/sessions, got %s", detail2.Path)
	}

	// Not found
	_, err = spec.LookupEndpoint("GET", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent endpoint")
	}
}
