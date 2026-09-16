package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestQueueReviewerSettingsPreserveNulls(t *testing.T) {
	var current langsmith.AnnotationQueueGetResponse
	if err := json.Unmarshal([]byte(`{"num_reviewers_per_item":null,"enable_reservations":false,"reservation_minutes":null}`), &current); err != nil {
		t.Fatal(err)
	}
	params := langsmith.AnnotationQueueUpdateParams{}
	if _, err := preserveQueueReviewSettings(&current, &params); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["num_reviewers_per_item"]) != "null" || string(fields["reservation_minutes"]) != "null" || string(fields["enable_reservations"]) != "false" {
		t.Fatalf("null/false converted: %s", b)
	}
	if _, err := preserveQueueReviewSettings(nil, &params); err == nil {
		t.Fatal("accepted missing queue")
	}
	if err := json.Unmarshal([]byte(`{"enable_reservations":true}`), &current); err != nil {
		t.Fatal(err)
	}
	if _, err := preserveQueueReviewSettings(&current, &params); err == nil {
		t.Fatal("accepted incomplete reviewer settings")
	}
}

func TestQueueListPreservesNullSettings(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"id":%q,"reservation_minutes":null,"enable_reservations":false}]`, importDatasetID)
	})
	defer setupTestEnv(t, ts.URL)()
	cmd := newQueueCmd()
	cmd.SetArgs([]string{"list", "--limit", "1"})
	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	var result struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || string(result.Items[0]["reservation_minutes"]) != "null" || string(result.Items[0]["enable_reservations"]) != "false" {
		t.Fatalf("null/false not preserved: %s", out)
	}
}

func TestQueueValidationDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"create", "--name", " "}, "invalid_queue_name"},
		{[]string{"list", "--limit", "0"}, "invalid_queue_pagination"},
		{[]string{"list", "--offset", "-1"}, "invalid_queue_pagination"},
		{[]string{"items", importDatasetID, "--status", "invalid"}, "invalid_queue_status"},
		{[]string{"items", importDatasetID, "--limit", "101"}, "invalid_queue_page_size"},
	} {
		t.Run(tc.code+strings.Join(tc.args, " "), func(t *testing.T) {
			cmd := newQueueCmd()
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			var diagnostic commandDiagnostic
			if !errors.As(err, &diagnostic) {
				t.Fatalf("expected actionable diagnostic, got %v", err)
			}
			code, message, next := diagnostic.CLIDiagnostic()
			if code != tc.code || message == "" || next == "" {
				t.Fatalf("diagnostic=%v", diagnostic)
			}
		})
	}
}

func TestQueueRubricValidation(t *testing.T) {
	for _, body := range []string{
		`null`, `{}`, `[{}]`, `[{"feedback_key":" "}]`,
		`[{"feedback_key":"x"},{"feedback_key":"x"}]`,
		`[{"feedback_key":" x"}]`, `[{"feedback_key":"x","unknown":true}]`,
		`[{"feedback_key":"x","feedback_key":"y"}]`,
		`[{"feedback_key":"x","is_required":"true"}]`, `[] []`,
		`[{"feedback_key":"x","is_assertion":true}]`,
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

func TestQueueFeedbackConfigPreflight(t *testing.T) {
	for _, operation := range []string{"create", "configure"} {
		for _, response := range []string{`[]`, `[{"feedback_key":"correctness","feedback_config":null}]`, `invalid`} {
			t.Run(operation+response, func(t *testing.T) {
				writes := 0
				ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodGet {
						writes++
					}
					w.Header().Set("Content-Type", "application/json")
					if r.URL.Path == "/api/v1/feedback-configs" {
						fmt.Fprint(w, response)
						return
					}
					fmt.Fprintf(w, `{"id":%q}`, importDatasetID)
				})
				defer setupTestEnv(t, ts.URL)()
				file := workflowFile(t, `[{"feedback_key":"correctness"}]`)
				args := []string{"create", "--name", "test", "--rubric", file}
				if operation == "configure" {
					args = []string{"configure", importDatasetID, "--rubric", file, "--dry-run"}
				}
				cmd := newQueueCmd()
				cmd.SetArgs(args)
				var err error
				out := captureStdout(t, func() { err = cmd.Execute() })
				var diagnostic commandDiagnostic
				if !errors.As(err, &diagnostic) || writes != 0 || out != "" {
					t.Fatalf("preflight err=%v writes=%d output=%s", err, writes, out)
				}
				code, _, _ := diagnostic.CLIDiagnostic()
				if code != "queue_feedback_config_missing" && code != "queue_feedback_configs_unavailable" {
					t.Fatal(code)
				}
			})
		}
	}
}

func TestQueueRubricRequests(t *testing.T) {
	for _, mode := range []string{"create", "preview", "apply", "clear", "instructions-only", "rubric-only", "failure"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v1/feedback-configs" {
					fmt.Fprint(w, `[{"feedback_key":"correctness","feedback_config":{"type":"continuous"}}]`)
					return
				}
				if r.Method == http.MethodGet {
					fmt.Fprintf(w, `{"id":%q,"num_reviewers_per_item":3,"enable_reservations":false,"reservation_minutes":7}`, importDatasetID)
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
				if mode != "create" {
					if string(body["num_reviewers_per_item"]) != "3" || string(body["enable_reservations"]) != "false" || string(body["reservation_minutes"]) != "7" {
						t.Errorf("reviewer settings not preserved: %v", body)
					}
					delete(body, "num_reviewers_per_item")
					delete(body, "enable_reservations")
					delete(body, "reservation_minutes")
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
	var items []struct {
		IsRequired *bool `json:"is_required"`
	}
	if err := json.Unmarshal(result.Queue["rubric_items"], &items); err != nil {
		t.Fatal(err)
	}
	if result.QueueID != importDatasetID || string(result.Queue["rubric_instructions"]) != "null" || len(items) != 1 || items[0].IsRequired == nil || *items[0].IsRequired {
		t.Fatalf("raw queue fields not preserved: %s", out)
	}
}
