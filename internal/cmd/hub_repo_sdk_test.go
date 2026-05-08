package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func decodeRequestJSONBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	defer r.Body.Close()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode request body: %v\nbody: %s", err, string(body))
	}
	return out
}

func newHubRepoSDKTestClient(t *testing.T, serverURL string) func() {
	t.Helper()
	cleanup := setupTestEnv(t, serverURL)
	return cleanup
}

func TestHubRepoTypeParam(t *testing.T) {
	tests := []struct {
		name     string
		repoType string
		wantErr  bool
	}{
		{name: "agent", repoType: "agent"},
		{name: "skill", repoType: "skill"},
		{name: "invalid", repoType: "prompt", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := hubRepoTypeParam(tc.repoType)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.repoType)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.repoType, err)
			}
			if string(got) != tc.repoType {
				t.Fatalf("got %q, want %q", got, tc.repoType)
			}
		})
	}
}

func TestEnsureHubRepo_RepoExists_NoMetadata_NoMutation(t *testing.T) {
	var sawGet, sawUpdate, sawCreate bool

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/repos/acme/my-skill":
			sawGet = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case r.Method == "PATCH" && r.URL.Path == "/api/v1/repos/acme/my-skill":
			sawUpdate = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/repos":
			sawCreate = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer newHubRepoSDKTestClient(t, srv.URL)()

	err := ensureHubRepo(context.Background(), MustGetClient(), "acme", "my-skill", "skill", hubRepoMeta{})
	if err != nil {
		t.Fatalf("ensureHubRepo: %v", err)
	}
	if !sawGet {
		t.Fatal("expected GET /api/v1/repos/acme/my-skill")
	}
	if sawUpdate {
		t.Fatal("did not expect repo update")
	}
	if sawCreate {
		t.Fatal("did not expect repo create")
	}
}

func TestEnsureHubRepo_RepoExists_WithMetadata_Updates(t *testing.T) {
	var (
		sawGet    bool
		sawUpdate bool
		body      map[string]any
	)

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/repos/acme/my-skill":
			sawGet = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case r.Method == "PATCH" && r.URL.Path == "/api/v1/repos/acme/my-skill":
			sawUpdate = true
			body = decodeRequestJSONBody(t, r)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer newHubRepoSDKTestClient(t, srv.URL)()

	desc := "test description"
	readme := "# test readme"
	isPublic := true
	tags := []string{"tag-a", "tag-b"}
	meta := hubRepoMeta{
		Description: &desc,
		Readme:      &readme,
		IsPublic:    &isPublic,
		Tags:        tags,
	}

	err := ensureHubRepo(context.Background(), MustGetClient(), "acme", "my-skill", "skill", meta)
	if err != nil {
		t.Fatalf("ensureHubRepo: %v", err)
	}
	if !sawGet || !sawUpdate {
		t.Fatalf("expected GET and PATCH, got GET=%v PATCH=%v", sawGet, sawUpdate)
	}

	if got, _ := body["description"].(string); got != desc {
		t.Fatalf("description = %q, want %q", got, desc)
	}
	if got, _ := body["readme"].(string); got != readme {
		t.Fatalf("readme = %q, want %q", got, readme)
	}
	if got, _ := body["is_public"].(bool); got != isPublic {
		t.Fatalf("is_public = %v, want %v", got, isPublic)
	}

	gotTags, ok := body["tags"].([]any)
	if !ok {
		t.Fatalf("tags type = %T, want []any", body["tags"])
	}
	if len(gotTags) != 2 || gotTags[0] != "tag-a" || gotTags[1] != "tag-b" {
		t.Fatalf("tags = %#v, want [tag-a tag-b]", gotTags)
	}
}

func TestEnsureHubRepo_404_CreatesRepo(t *testing.T) {
	var (
		sawGet    bool
		sawCreate bool
		body      map[string]any
	)

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/repos/acme/new-skill":
			sawGet = true
			http.Error(w, "not found", http.StatusNotFound)
		case r.Method == "POST" && r.URL.Path == "/api/v1/repos":
			sawCreate = true
			body = decodeRequestJSONBody(t, r)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer newHubRepoSDKTestClient(t, srv.URL)()

	err := ensureHubRepo(context.Background(), MustGetClient(), "acme", "new-skill", "skill", hubRepoMeta{})
	if err != nil {
		t.Fatalf("ensureHubRepo: %v", err)
	}
	if !sawGet || !sawCreate {
		t.Fatalf("expected GET and POST, got GET=%v POST=%v", sawGet, sawCreate)
	}

	if got, _ := body["repo_handle"].(string); got != "new-skill" {
		t.Fatalf("repo_handle = %q, want new-skill", got)
	}
	if got, _ := body["repo_type"].(string); got != "skill" {
		t.Fatalf("repo_type = %q, want skill", got)
	}
	if got, _ := body["is_public"].(bool); got {
		t.Fatalf("is_public = %v, want false", got)
	}
}

func TestEnsureHubRepo_404_CreateConflict_IsIgnored(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/repos/acme/my-skill":
			http.Error(w, "not found", http.StatusNotFound)
		case r.Method == "POST" && r.URL.Path == "/api/v1/repos":
			http.Error(w, "conflict", http.StatusConflict)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer newHubRepoSDKTestClient(t, srv.URL)()

	err := ensureHubRepo(context.Background(), MustGetClient(), "acme", "my-skill", "skill", hubRepoMeta{})
	if err != nil {
		t.Fatalf("expected nil on 409 create conflict, got: %v", err)
	}
}

func TestEnsureHubRepo_Non404GetError_ReturnsCheckingError(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/v1/repos/acme/my-skill" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer newHubRepoSDKTestClient(t, srv.URL)()

	err := ensureHubRepo(context.Background(), MustGetClient(), "acme", "my-skill", "skill", hubRepoMeta{})
	if err == nil || !strings.Contains(err.Error(), "checking acme/my-skill") {
		t.Fatalf("expected checking error, got: %v", err)
	}
}

func TestEnsureHubRepo_InvalidRepoType_ReturnsErrorBeforeCreate(t *testing.T) {
	var sawCreate bool

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/repos/acme/my-skill":
			http.Error(w, "not found", http.StatusNotFound)
		case r.Method == "POST" && r.URL.Path == "/api/v1/repos":
			sawCreate = true
			http.Error(w, "unexpected", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer newHubRepoSDKTestClient(t, srv.URL)()

	err := ensureHubRepo(context.Background(), MustGetClient(), "acme", "my-skill", "prompt", hubRepoMeta{})
	if err == nil || !strings.Contains(err.Error(), "repo type must be 'agent' or 'skill'") {
		t.Fatalf("expected repo type error, got: %v", err)
	}
	if sawCreate {
		t.Fatal("did not expect create call for invalid repo type")
	}
}
