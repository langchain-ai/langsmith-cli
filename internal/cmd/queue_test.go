package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueueAdd(t *testing.T) {
	for _, mode := range []string{"dry-run", "apply", "empty-response"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "GET" {
					fmt.Fprintf(w, `{"id":%q}`, "22222222-2222-4222-8222-222222222222")
					return
				}
				if r.URL.Path != "/api/v1/platform/annotation-queues/"+"22222222-2222-4222-8222-222222222222"+"/items" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				writes++
				var body struct {
					Items []struct {
						ProjectID string `json:"project_id"`
						ThreadID  string `json:"thread_id"`
						ItemType  string `json:"item_type"`
					} `json:"items"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if len(body.Items) != 1 || body.Items[0].ProjectID != deleteTestProjectID || body.Items[0].ThreadID != "conversation" || body.Items[0].ItemType != "THREAD" {
					t.Errorf("bad body %#v", body)
				}
				if mode == "empty-response" {
					fmt.Fprint(w, `{"items":[]}`)
				} else {
					fmt.Fprintf(w, `{"items":[{"id":%q,"queue_id":%q,"project_id":%q,"thread_id":"conversation","item_type":"THREAD"}]}`, "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", deleteTestProjectID)
				}
			})
			defer setupTestEnv(t, ts.URL)()
			args := []string{"22222222-2222-4222-8222-222222222222", "--project-id", deleteTestProjectID, "--thread-id", "conversation"}
			if mode == "dry-run" {
				args = append(args, "--dry-run")
			}
			c := newQueueAddCmd()
			c.SetArgs(args)
			var err error
			out := captureStdout(t, func() { err = c.Execute() })
			if mode == "empty-response" {
				if err == nil {
					t.Fatal("missing results reported success")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if mode == "dry-run" && writes != 0 {
				t.Fatal("dry run wrote")
			}
			if !json.Valid([]byte(out)) {
				t.Fatalf("not JSON: %s", out)
			}
		})
	}
}

func TestQueuePlanValidation(t *testing.T) {
	for _, mode := range []string{"valid", "wrong-project", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "GET" {
					fmt.Fprintf(w, `{"id":%q}`, "22222222-2222-4222-8222-222222222222")
					return
				}
				writes++
				if r.URL.Path == "/api/v1/runs/query" {
					t.Error("plan replay must not rediscover runs")
				}
				fmt.Fprintf(w, `{"items":[{"id":%q,"queue_id":%q,"project_id":%q,"thread_id":"conversation","item_type":"THREAD"}]}`, "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", deleteTestProjectID)
			})
			defer setupTestEnv(t, ts.URL)()
			plan := queueAddPlan{Version: 1, APIURL: ts.URL, WorkspaceID: GetWorkspaceID(), ProjectID: deleteTestProjectID, QueueID: "22222222-2222-4222-8222-222222222222", Items: []queueAddItem{{ThreadID: "conversation"}}}
			if mode == "wrong-project" {
				plan.ProjectID = "11111111-1111-4111-8111-111111111111"
			}
			if mode == "duplicate" {
				plan.Items = append(plan.Items, plan.Items[0])
			}
			b, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "plan.json")
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			c := newQueueAddCmd()
			c.SetArgs([]string{"22222222-2222-4222-8222-222222222222", "--project-id", deleteTestProjectID, "--plan", path})
			captureStdout(t, func() { err = c.Execute() })
			if mode == "valid" {
				if err != nil || writes != 1 {
					t.Fatalf("err=%v writes=%d", err, writes)
				}
			} else if err == nil || writes != 0 {
				t.Fatalf("invalid plan err=%v writes=%d", err, writes)
			}
		})
	}
}

func TestQueueDeleteNoninteractive(t *testing.T) {
	c := newQueueCmd()
	c.SetArgs([]string{"delete", "22222222-2222-4222-8222-222222222222"})
	if err := c.Execute(); err == nil {
		t.Fatal("delete must require confirmation")
	}
}

func TestQueueCreateDoesNotReportFalseSuccess(t *testing.T) {
	for _, failHTTP := range []bool{true, false} {
		t.Run(fmt.Sprint(failHTTP), func(t *testing.T) {
			writes := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				writes++
				w.Header().Set("Content-Type", "application/json")
				if failHTTP {
					w.WriteHeader(http.StatusInternalServerError)
				}
				fmt.Fprint(w, `{}`)
			})
			defer setupTestEnv(t, ts.URL)()
			c := newQueueCmd()
			c.SetArgs([]string{"create", "--name", "status-test"})
			var err error
			out := captureStdout(t, func() { err = c.Execute() })
			if err == nil || strings.Contains(out, `"created"`) || writes != 1 {
				t.Fatalf("err=%v writes=%d stdout=%s", err, writes, out)
			}
		})
	}
}
