package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestQueueRubricValidation(t *testing.T) {
	for _, body := range []string{
		`null`, `{}`, `[{}]`, `[{"feedback_key":" "}]`,
		`[{"feedback_key":"x"},{"feedback_key":"x"}]`,
		`[{"feedback_key":" x"}]`, `[{"feedback_key":"x","unknown":true}]`,
		`[{"feedback_key":"x","feedback_key":"y"}]`,
		`[{"feedback_key":"x","is_required":"true"}]`, `[] []`,
	} {
		t.Run(body, func(t *testing.T) {
			cmd := newQueueConfigureCmd()
			cmd.SetArgs([]string{importDatasetID, "--rubric", workflowFile(t, body), "--dry-run"})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "rubric") {
				t.Fatalf("expected rubric error, got %v", err)
			}
		})
	}
	for _, args := range [][]string{
		{importDatasetID}, {importDatasetID, "--instructions", "review"},
		{importDatasetID, "--dry-run"},
		{importDatasetID, "--instructions", "review", "--dry-run", "--apply"},
	} {
		cmd := newQueueConfigureCmd()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestQueueRubricRequests(t *testing.T) {
	for _, mode := range []string{"create", "preview", "apply", "clear", "instructions-only", "rubric-only", "failure"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet {
					fmt.Fprintf(w, `{"id":%q}`, importDatasetID)
					return
				}
				writes++
				method, path := http.MethodPatch, "/api/v1/annotation-queues/"+importDatasetID
				if mode == "create" {
					method, path = http.MethodPost, "/api/v1/annotation-queues"
				}
				if r.Method != method || r.URL.Path != path {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if mode == "instructions-only" {
					if len(body) != 1 || body["rubric_instructions"] == nil {
						t.Errorf("unexpected fields: %v", body)
					}
				} else {
					var items []map[string]any
					if err := json.Unmarshal(body["rubric_items"], &items); err != nil {
						t.Error(err)
						return
					}
					if mode == "clear" {
						if string(body["rubric_items"]) != "[]" || string(body["rubric_instructions"]) != `""` {
							t.Errorf("clear not preserved: %v", body)
						}
					} else if len(items) != 1 || items[0]["is_required"] != false || items[0]["feedback_key"] != "correctness" {
						t.Errorf("invalid rubric: %v", items)
					}
					if mode == "rubric-only" && (len(body) != 1 || body["rubric_instructions"] != nil) {
						t.Errorf("omitted instructions sent: %v", body)
					}
				}
				if mode == "failure" {
					http.Error(w, `{"detail":"unavailable"}`, http.StatusServiceUnavailable)
					return
				}
				fmt.Fprintf(w, `{"id":%q}`, importDatasetID)
			})
			defer setupTestEnv(t, ts.URL)()
			body := `[{"feedback_key":"correctness","is_required":false,"score_descriptions":{"0":"Incorrect","1":"Correct"}}]`
			if mode == "clear" {
				body = `[]`
			}
			args := []string{"configure", importDatasetID, "--apply"}
			if mode == "preview" {
				args = []string{"configure", importDatasetID, "--dry-run"}
			} else if mode == "create" {
				args = []string{"create", "--name", "test-review"}
			}
			if mode != "instructions-only" {
				args = append(args, "--rubric", workflowFile(t, body))
			}
			if mode != "rubric-only" {
				args = append(args, "--instructions", "")
			}
			cmd := newQueueCmd()
			cmd.SetArgs(args)
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if mode == "failure" {
				if err == nil || writes != 1 || out != "" {
					t.Fatalf("failure retried or reported success: writes=%d out=%s err=%v", writes, out, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if mode == "preview" {
				if writes != 0 || result["status"] != "dry_run" {
					t.Fatalf("preview wrote: %s", out)
				}
			} else if writes != 1 {
				t.Fatalf("writes=%d", writes)
			}
		})
	}
}

func TestQueueGetRubric(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/annotation-queues/"+importDatasetID {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%q,"rubric_instructions":null,"rubric_items":[{"feedback_key":"correctness","is_required":false}]}`, importDatasetID)
	})
	defer setupTestEnv(t, ts.URL)()
	cmd := newQueueCmd()
	cmd.SetArgs([]string{"get", importDatasetID})
	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	var result struct {
		QueueID string                     `json:"queue_id"`
		Queue   map[string]json.RawMessage `json:"queue"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result.QueueID != importDatasetID || string(result.Queue["rubric_instructions"]) != "null" || !strings.Contains(string(result.Queue["rubric_items"]), `"is_required":false`) {
		t.Fatalf("raw queue fields not preserved: %s", out)
	}
}
