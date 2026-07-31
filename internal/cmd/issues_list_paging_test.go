package cmd

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

// These assert the query the server actually receives from `issues list`.
//
// Only TestIssuesList_SendsLimitAndOffsetToServer is a regression test: the
// command used to build its own query string and apply --limit client-side, so
// neither param reached the server and a board larger than one page was
// unreachable. The rest characterize behavior that the move to the generated
// SDK had to preserve, and would catch a future switch to the typed status
// constants (which omit `fixing` and `watching`) silently dropping a filter.
func TestIssuesList_SendsLimitAndOffsetToServer(t *testing.T) {
	var got url.Values
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	})
	if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
		"project", "issues", "list",
		"--project", "my-app", "--limit", "200", "--offset", "400",
	); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if got.Get("limit") != "200" {
		t.Errorf("limit not sent to server: got %q, want %q", got.Get("limit"), "200")
	}
	if got.Get("offset") != "400" {
		t.Errorf("offset not sent to server: got %q, want %q", got.Get("offset"), "400")
	}
	if got.Get("session_name") != "my-app" {
		t.Errorf("session_name: got %q, want %q", got.Get("session_name"), "my-app")
	}
}

// offset defaults to 0 and is omitted rather than sent as offset=0.
func TestIssuesList_OmitsOffsetWhenUnset(t *testing.T) {
	var got url.Values
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	})
	if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
		"project", "issues", "list", "--project", "my-app"); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if got.Has("offset") {
		t.Errorf("offset should be omitted when unset, got %q", got.Get("offset"))
	}
}

// `fixing` and `watching` are valid server-side but were missing from the
// OpenAPI status enum, so the generated SDK constant set omits them. The command
// converts the raw flag value, which must still reach the wire unaltered.
func TestIssuesList_SendsNonConstantStatuses(t *testing.T) {
	for _, status := range []string{"open", "fixing", "watching", "completed", "ignored"} {
		t.Run(status, func(t *testing.T) {
			var got url.Values
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			})
			if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
				"project", "issues", "list",
				"--project", "my-app", "--status", status,
			); err != nil {
				t.Fatalf("command failed: %v", err)
			}

			if got.Get("status") != status {
				t.Errorf("status not sent verbatim: got %q, want %q", got.Get("status"), status)
			}
		})
	}
}

// --priority maps to the numeric severity the API expects.
func TestIssuesList_MapsPriorityToSeverity(t *testing.T) {
	for priority, want := range map[string]string{
		"urgent": "0",
		"high":   "1",
		"medium": "2",
		"low":    "3",
	} {
		t.Run(priority, func(t *testing.T) {
			var got url.Values
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			})
			if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
				"project", "issues", "list",
				"--project", "my-app", "--priority", priority,
			); err != nil {
				t.Fatalf("command failed: %v", err)
			}

			if got.Get("severity") != want {
				t.Errorf("severity for %s: got %q, want %q", priority, got.Get("severity"), want)
			}
		})
	}
}

// Project names carry spaces and punctuation, and encoding moved from a
// hand-rolled replacer to the SDK's query encoder. Pins the round-trip.
func TestIssuesList_EncodesProjectNameWithSpecialChars(t *testing.T) {
	const project = "my app/v2?x=1&y=2"

	var got url.Values
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	})
	if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
		"project", "issues", "list", "--project", project); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if got.Get("session_name") != project {
		t.Errorf("session_name round-trip: got %q, want %q", got.Get("session_name"), project)
	}
}

// The Engine issues-agent redirects this command's JSON to a file and reads it
// with `jq -r '.[] | [.id, .status, .name] | @tsv'`, so the output must stay a
// bare array of objects carrying those keys. Moving to the SDK replaced a
// hand-built map with raw response passthrough; this pins the shape jq needs.
func TestIssuesList_JSONOutputStaysArrayWithGreppedKeys(t *testing.T) {
	const body = `[{"id":"11111111-1111-1111-1111-111111111111",` +
		`"session_id":"22222222-2222-2222-2222-222222222222",` +
		`"name":"Agent retries failed tool call","status":"open","severity":1,` +
		`"tags":["regression"],"created_at":"2026-07-30T12:00:00Z"}]`

	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	out := captureStdout(t, func() {
		if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
			"--format", "json", "project", "issues", "list", "--project", "my-app",
		); err != nil {
			t.Fatalf("command failed: %v", err)
		}
	})

	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output is not a JSON array: %v\noutput: %s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d: %s", len(rows), out)
	}
	for _, key := range []string{"id", "status", "name"} {
		if _, ok := rows[0][key]; !ok {
			t.Errorf("key %q missing from JSON output; Engine's jq depends on it: %s", key, out)
		}
	}
	if rows[0]["status"] != "open" {
		t.Errorf("status = %v, want open", rows[0]["status"])
	}
	if rows[0]["name"] != "Agent retries failed tool call" {
		t.Errorf("name = %v", rows[0]["name"])
	}
}
