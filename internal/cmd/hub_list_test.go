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
		cmd.SetArgs([]string{"list", "--type", "skill", "--source", "external", "--query", "foo", "--public", "--limit", "10", "--offset", "5"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	checks := map[string]string{
		"repo_type":    "skill",
		"source":       "external",
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

func TestHubList_RejectsBadSource(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer setupTestEnv(t, srv.URL)()

	cmd := newHubCmd()
	cmd.SetArgs([]string{"list", "--source", "unknown"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "internal") {
		t.Errorf("expected error mentioning valid sources; got %v", err)
	}
}

func TestHubList_RendersTable(t *testing.T) {
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
	if !strings.Contains(out, "Hub repos") || !strings.Contains(out, "acme/foo") || !strings.Contains(out, "skill") {
		t.Errorf("missing fields in output:\n%s", out)
	}
}

// hub list adds cursors.next to its JSON and resumes from --cursor without an
// offset, which the server rejects next to a cursor.
func TestHubList_Cursor(t *testing.T) {
	var got []url.Values
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"repos":       []any{map[string]any{"full_name": "acme/a"}},
			"total":       3,
			"next_cursor": "c-next",
		})
	})
	defer setupTestEnv(t, srv.URL)()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newHubCmd()
		cmd.SetArgs([]string{"list", "--limit", "1", "--cursor", "c-prev"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if len(got) != 1 || got[0].Get("cursor") != "c-prev" || got[0].Has("offset") || got[0].Get("limit") != "1" {
		t.Errorf("queries = %v, want one request with cursor=c-prev, limit=1 and no offset", got)
	}
	var body struct {
		Total   int            `json:"total"`
		Repos   []any          `json:"repos"`
		Cursors map[string]any `json:"cursors"`
	}
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatalf("parsing %q: %v", out, err)
	}
	if body.Total != 3 || len(body.Repos) != 1 || body.Cursors["next"] != "c-next" {
		t.Errorf("output = %+v, want total 3, one repo and cursors.next c-next", body)
	}
}

func TestHubList_RejectsCursorWithOffset(t *testing.T) {
	cmd := newHubCmd()
	cmd.SetArgs([]string{"list", "--cursor", "c", "--offset", "5"})
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--offset") {
		t.Fatalf("err = %v, want a --cursor/--offset conflict", err)
	}
}
