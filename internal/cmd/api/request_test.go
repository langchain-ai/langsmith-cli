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
	code, err := runRequest(ts.URL, "test-key", "GET", "sessions", "", nil, false, &out)
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
	code, err := runRequest(ts.URL, "key", "POST", "sessions", `{"name":"test"}`, nil, false, &out)
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
	_, err := runRequest(ts.URL, "key", "GET", "sessions", "", []string{"X-Custom:val"}, false, &out)
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
	_, err := runRequest(ts.URL, "key", "GET", "sessions", "", nil, true, &out)
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
	code, err := runRequest(ts.URL, "key", "GET", "sessions", "", nil, false, &out)
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
	code, err := runRequest(ts.URL, "key", "POST", "sessions", "@"+f.Name(), nil, false, &out)
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
		if r.Header.Get("x-api-key") != "key" {
			t.Errorf("expected x-api-key=key, got %q", r.Header.Get("x-api-key"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"full_url":true}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	// Pass a full URL as the path — should NOT prepend apiURL
	code, err := runRequest("https://different.host", "key", "GET", ts.URL+"/custom/endpoint", "", nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Errorf("expected 200, got %d", code)
	}
	if !strings.Contains(out.String(), "full_url") {
		t.Errorf("expected full_url in response, got %q", out.String())
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
