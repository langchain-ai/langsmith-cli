package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFeedbackCreate(t *testing.T) {
	for _, found := range []bool{true, false} {
		t.Run(fmt.Sprint(found), func(t *testing.T) {
			writes := 0
			feedbackID := ""
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "GET" && feedbackID != "" && r.URL.Path == "/api/v1/feedback/"+feedbackID {
					fmt.Fprintf(w, `{"id":%q,"run_id":%q,"session_id":%q,"key":"quality","score":0,"comment":null}`, feedbackID, "11111111-1111-4111-8111-111111111111", deleteTestProjectID)
					return
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				switch r.URL.Path {
				case "/api/v1/runs/query":
					sessions, _ := body["session"].([]any)
					if len(sessions) != 1 || sessions[0] != deleteTestProjectID {
						t.Errorf("missing project scope: %#v", body)
					}
					if found {
						fmt.Fprintf(w, `{"runs":[{"id":%q,"trace_id":%q,"start_time":"2026-09-11T00:00:00Z"}]}`, "11111111-1111-4111-8111-111111111111", "11111111-1111-4111-8111-111111111111")
					} else {
						fmt.Fprint(w, `{"runs":[]}`)
					}
				case "/api/v1/feedback":
					writes++
					feedbackID, _ = body["id"].(string)
					if body["score"] != float64(0) || body["run_id"] != "11111111-1111-4111-8111-111111111111" || body["session_id"] != deleteTestProjectID {
						t.Errorf("invalid feedback: %#v", body)
					}
					fmt.Fprintf(w, `{"id":%q,"key":"quality","score":0}`, "11111111-1111-4111-8111-111111111111")
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
			})
			defer setupTestEnv(t, ts.URL)()
			c := newRunCmd()
			c.SetArgs([]string{"feedback", "create", "--run-id", "11111111-1111-4111-8111-111111111111", "--project-id", deleteTestProjectID, "--key", "quality", "--score", "0"})
			var err error
			captureStdout(t, func() { err = c.Execute() })
			if found && (err != nil || writes != 1) {
				t.Fatalf("err=%v writes=%d", err, writes)
			}
			if !found && (err == nil || writes != 0) {
				t.Fatalf("err=%v writes=%d", err, writes)
			}
		})
	}
}

func TestResourceIDCanonicalComparison(t *testing.T) {
	if !sameResourceID("11111111-1111-4111-8111-111111111111", strings.ToUpper("11111111-1111-4111-8111-111111111111")) {
		t.Fatal("UUID case differs")
	}
	if sameResourceID("11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222") || sameResourceID("", "") {
		t.Fatal("different/empty IDs equal")
	}
}

func TestFeedbackNestedUnderRun(t *testing.T) {
	root := NewRootCmd("test", "test")
	for _, action := range []string{"create", "list", "get"} {
		cmd, remaining, err := root.Find([]string{"run", "feedback", action})
		if err != nil || len(remaining) != 0 || cmd.CommandPath() != "langsmith run feedback "+action {
			t.Fatalf("nested command %s not registered: %v", action, err)
		}
	}
	for _, cmd := range root.Commands() {
		if cmd.Name() == "feedback" {
			t.Fatal("feedback must not be registered at the root")
		}
	}
}
