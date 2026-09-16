package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestFeedbackReconciliation(t *testing.T) {
	for _, mode := range []string{"created", "skipped", "conflict", "recovered", "unverified"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			stored := mode == "skipped" || mode == "conflict"
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v1/runs/query":
					fmt.Fprintf(w, `{"runs":[{"id":%q,"trace_id":%q,"start_time":"2026-09-11T00:00:00Z"}]}`, "11111111-1111-4111-8111-111111111111", "11111111-1111-4111-8111-111111111111")
				case "/api/v1/feedback/" + "22222222-2222-4222-8222-222222222222":
					if !stored {
						w.WriteHeader(404)
						fmt.Fprint(w, `{}`)
						return
					}
					score := 0
					if mode == "conflict" {
						score = 1
					}
					fmt.Fprintf(w, `{"id":%q,"run_id":%q,"session_id":%q,"key":"quality","score":%d,"comment":null}`, "22222222-2222-4222-8222-222222222222", "11111111-1111-4111-8111-111111111111", deleteTestProjectID, score)
				case "/api/v1/feedback":
					writes++
					stored = mode != "unverified"
					if mode == "recovered" || mode == "unverified" {
						w.WriteHeader(500)
						fmt.Fprint(w, `{"detail":"secret-upstream-body"}`)
					} else {
						fmt.Fprint(w, `{}`)
					}
				default:
					t.Errorf("unexpected %s", r.URL.Path)
				}
			})
			defer setupTestEnv(t, ts.URL)()
			flagOutputFormat = "json"
			c := newFeedbackCreateCmd()
			c.SetArgs([]string{"--project-id", deleteTestProjectID, "--run-id", "11111111-1111-4111-8111-111111111111", "--key", "quality", "--score", "0", "--id", "22222222-2222-4222-8222-222222222222"})
			var err error
			out := captureStdout(t, func() { err = c.Execute() })
			if mode == "conflict" {
				if err == nil || writes != 0 {
					t.Fatalf("conflict err=%v writes=%d", err, writes)
				}
				return
			}
			if (err != nil) != (mode == "unverified") {
				t.Fatalf("unexpected err %v", err)
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if result["status"] != mode || result["feedback_id"] != "22222222-2222-4222-8222-222222222222" || result["run_id"] != "11111111-1111-4111-8111-111111111111" {
				t.Fatalf("bad result %s", out)
			}
			wantWrites := 1
			if mode == "skipped" {
				wantWrites = 0
			}
			if writes != wantWrites {
				t.Fatalf("unexpected write retry: %d", writes)
			}
		})
	}
}

func TestFeedbackMatchingNullAndZero(t *testing.T) {
	for _, value := range []string{"null", "0", "false"} {
		var feedback langsmith.FeedbackSchema
		if err := json.Unmarshal([]byte(fmt.Sprintf(`{"id":"f","run_id":"r","session_id":"p","key":"k","score":%s,"comment":null}`, value)), &feedback); err != nil {
			t.Fatal(err)
		}
		if feedbackMatches(&feedback, "f", "p", "r", "k", 0, true, "", false) != (value == "0") {
			t.Fatalf("zero confused with %s", value)
		}
		if feedbackMatches(&feedback, "f", "p", "r", "k", 0, false, "", false) != (value == "null") {
			t.Fatalf("missing confused with %s", value)
		}
	}
}
