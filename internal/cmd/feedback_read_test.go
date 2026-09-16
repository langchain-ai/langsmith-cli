package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestFeedbackReadCommands(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	for _, mode := range []string{"get", "list"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" {
					t.Errorf("unexpected method %s", r.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				if mode == "get" {
					if r.URL.Path != "/api/v1/feedback/"+id {
						t.Error("incorrect feedback path")
					}
					fmt.Fprintf(w, `{"id":%q,"key":"quality","score":null,"run_id":%q}`, id, id)
					return
				}
				q := r.URL.Query()
				for key, want := range map[string]string{"run": id, "key": "quality", "source": "app", "has_score": "false", "has_comment": "true", "min_created_at": "2026-09-01T00:00:00Z", "max_created_at": "2026-09-02T00:00:00Z"} {
					if q.Get(key) != want {
						t.Errorf("%s = %q; want %q", key, q.Get(key), want)
					}
				}
				fmt.Fprint(w, `[]`)
			})
			defer setupTestEnv(t, ts.URL)()
			flagOutputFormat = "json"
			cmd := newRunCmd()
			args := []string{"feedback", "get", id}
			if mode == "list" {
				args = []string{"feedback", "list", "--run-id", id, "--key", "quality", "--source", "app", "--has-score=false", "--has-comment", "--start-time", "2026-09-01T00:00:00Z", "--end-time", "2026-09-02T00:00:00Z"}
			}
			cmd.SetArgs(args)
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid([]byte(out)) || calls != 1 {
				t.Fatalf("invalid output or extra requests: %s, %d", out, calls)
			}
		})
	}
}

func TestFeedbackFilterValidation(t *testing.T) {
	for _, args := range [][]string{{"--source", "invalid"}, {"--key", " "}, {"--start-time", "yesterday"}, {"--start-time", "2026-09-02T00:00:00Z", "--end-time", "2026-09-01T00:00:00Z"}} {
		cmd := newFeedbackListCmd()
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatal(err)
		}
		// Execute rejects invalid filters before consulting authentication or the API.
		cmd.SetArgs(append([]string{"--run-id", "11111111-1111-4111-8111-111111111111"}, args...))
		if err := cmd.Execute(); err == nil {
			t.Errorf("accepted invalid filters %v", args)
		}
	}
}

func TestFeedbackPrettyScores(t *testing.T) {
	var items []langsmith.FeedbackSchema
	if err := json.Unmarshal([]byte(`[{"id":"a","key":"zero","score":0},{"id":"b","key":"missing","score":null}]`), &items); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() { printFeedback(items) })
	if !strings.Contains(out, "N/A") || !strings.Contains(out, "zero") {
		t.Fatalf("missing score rendering: %s", out)
	}
}

func TestFeedbackSafeWriteDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   string
	}{{403, "feedback_permission_denied"}, {422, "feedback_invalid_request"}, {500, "feedback_unverified"}} {
		err := feedbackWriteDiagnostic(&langsmith.Error{StatusCode: tc.status}, nil)
		d := err.(commandDiagnostic)
		if d.code != tc.code || d.next == "" {
			t.Errorf("incorrect diagnostic: %+v", d)
		}
	}
}

func TestFeedbackListLimitMatchesAPI(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("limit") != "100" {
			t.Error("unexpected API page size")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	defer setupTestEnv(t, ts.URL)()
	flagOutputFormat = "json"
	for _, limit := range []string{"100", "101", "1000", "0"} {
		cmd := newFeedbackListCmd()
		cmd.SetArgs([]string{"--run-id", "11111111-1111-4111-8111-111111111111", "--limit", limit})
		var err error
		captureStdout(t, func() { err = cmd.Execute() })
		if (err == nil) != (limit == "100") {
			t.Errorf("limit %s: %v", limit, err)
		}
	}
	if calls != 1 {
		t.Errorf("invalid limit reached API: %d calls", calls)
	}
}

func TestFeedbackRejectsBlankCommentOnly(t *testing.T) {
	for _, comment := range []string{"", " ", "\t\n"} {
		cmd := newFeedbackCreateCmd()
		cmd.SetArgs([]string{"--run-id", "11111111-1111-4111-8111-111111111111", "--key", "review", "--comment", comment})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--comment must not be blank") {
			t.Errorf("blank comment was not rejected: %v", err)
		}
	}
}
