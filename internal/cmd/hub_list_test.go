package cmd

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestHubList_QueryParams(t *testing.T) {
	var got url.Values
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"repos": []any{}, "total": 0})
	})
	defer setupTestEnv(t, srv.URL)()

	captureStdout(t, func() {
		cmd := newHubCmd()
		cmd.SetArgs([]string{"list", "--type", "skill", "--query", "foo", "--public", "--limit", "10", "--offset", "5"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	checks := map[string]string{
		"repo_type":    "skill",
		"query":        "foo",
		"match_prefix": "true",
		"is_public":    "true",
		"limit":        "10",
		"offset":       "5",
		"is_archived":  "false",
	}
	for k, v := range checks {
		if got.Get(k) != v {
			t.Errorf("query %s = %q, want %q", k, got.Get(k), v)
		}
	}
}

func TestHubList_RejectsBadType(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer setupTestEnv(t, srv.URL)()

	cmd := newHubCmd()
	cmd.SetArgs([]string{"list", "--type", "prompt"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "agent") {
		t.Errorf("expected error mentioning valid types; got %v", err)
	}
}

func TestHubList_RendersJSON(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"repos": []map[string]any{{"full_name": "acme/foo", "repo_type": "skill"}},
			"total": 1,
		})
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newHubCmd()
		cmd.SetArgs([]string{"list"})
		_ = cmd.Execute()
	})
	if !strings.Contains(out, `"total": 1`) || !strings.Contains(out, `"acme/foo"`) {
		t.Errorf("missing fields in output:\n%s", out)
	}
}
