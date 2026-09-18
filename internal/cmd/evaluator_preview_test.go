package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestEvaluatorMappingRejectsPluralRoots(t *testing.T) {
	for _, path := range []string{"inputs.message", "outputs.response", "inputs", "outputs[0]", " input.message"} {
		b, _ := json.Marshal(map[string]string{"question": path})
		if _, err := parseVariableMapping(string(b)); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	for _, path := range []string{"input.message", "output.response", "reference.answer", "all_messages", "all_messages[0]", "run.total_tokens"} {
		b, _ := json.Marshal(map[string]string{"question": path})
		if _, err := parseVariableMapping(string(b)); err != nil {
			t.Fatalf("rejected %q: %v", path, err)
		}
	}
}

func TestEvaluatorPreviewCommandValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
	}{
		{"requires dry run", nil},
		{"rejects dataset", []string{"--dry-run", "--dataset", "golden"}},
		{"rejects thread", []string{"--dry-run", "--group-by", "thread_id"}},
		{"rejects hub prompt", []string{"--dry-run", "--hub-ref", "test/judge"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("invalid preview made a request: %s %s", r.Method, r.URL.Path)
			})
			defer setupTestEnv(t, ts.URL)()
			isolateConfig(t)
			cmd := newEvaluatorCreateLLMCmd()
			args := []string{"--name", "judge", "--preview-run", importRunID, "--model-config", `{}`}
			cmd.SetArgs(append(args, tc.flags...))
			err := cmd.Execute()
			var diagnostic commandDiagnostic
			if !errors.As(err, &diagnostic) || diagnostic.code != "invalid_evaluator_preview" {
				t.Fatalf("expected invalid_evaluator_preview, got %v", err)
			}
		})
	}
}

func TestEvaluatorPreviewCommandReadOnly(t *testing.T) {
	for _, tc := range []struct {
		name, outputs string
		wantError     bool
	}{
		{"present", `{"response":"helpful"}`, false},
		{"missing", `{}`, true},
		{"null", `{"response":null}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/runs/query" {
					t.Errorf("preview must only query runs: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				var query map[string]any
				if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
					t.Error(err)
					return
				}
				if !reflect.DeepEqual(query["session"], []any{deleteTestProjectID}) || !reflect.DeepEqual(query["id"], []any{importRunID}) || query["is_root"] != true {
					t.Errorf("preview lost its scope: %#v", query)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"runs":[{"id":%q,"session_id":%q,"inputs":{"message":"question"},"outputs":%s}]}`, importRunID, deleteTestProjectID, tc.outputs)
			})
			defer setupTestEnv(t, ts.URL)()
			isolateConfig(t)
			flagOutputFormat = "json"
			cmd := newEvaluatorCreateLLMCmd()
			cmd.SetArgs([]string{"--name", "judge", "--project-id", deleteTestProjectID, "--dry-run", "--preview-run", importRunID,
				"--model-config", `{"id":["test-model"]}`, "--prompt", `[["human","{{question}} {{answer}}"]]`,
				"--schema", `{"type":"object","properties":{"score":{"type":"number"}}}`,
				"--variable-mapping", `{"question":"input.message","answer":"output.response"}`})
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if tc.wantError {
				var diagnostic commandDiagnostic
				if !errors.As(err, &diagnostic) || diagnostic.code != "evaluator_bindings_missing" {
					t.Fatalf("expected evaluator_bindings_missing, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Status  string `json:"status"`
				Preview struct {
					Validated bool           `json:"bindings_validated"`
					Bindings  map[string]any `json:"bindings"`
				} `json:"preview"`
			}
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatalf("stdout is not one JSON result: %v", err)
			}
			if calls != 1 || result.Status != "dry_run" || result.Preview.Validated == tc.wantError || result.Preview.Bindings["question"] != "question" {
				t.Fatalf("unexpected preview: calls=%d result=%+v", calls, result)
			}
		})
	}
}

func TestEvaluatorPreviewBindings(t *testing.T) {
	for _, tc := range []struct {
		name, outputs  string
		valid          bool
		missing, nulls int
	}{
		{"present", `{"response":"hello","zero":0,"flag":false}`, true, 0, 0},
		{"empty string", `{"response":""}`, true, 0, 0},
		{"missing", `{}`, false, 1, 0},
		{"null", `{"response":null}`, false, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/runs/query" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				var query map[string]any
				if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
					t.Fatal(err)
				}
				if query["session"].([]any)[0] != deleteTestProjectID {
					t.Fatal("missing project scope")
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"runs":[{"id":%q,"session_id":%q,"inputs":{"message":"question"},"outputs":%s}]}`, importRunID, deleteTestProjectID, tc.outputs)
			})
			defer setupTestEnv(t, ts.URL)()
			isolateConfig(t)
			c, err := getClient()
			if err != nil {
				t.Fatal(err)
			}
			got, err := previewEvaluatorRun(context.Background(), c, deleteTestProjectID, importRunID, map[string]string{"question": "input.message", "answer": "output.response"})
			if err != nil {
				t.Fatal(err)
			}
			if got["bindings_validated"] != tc.valid || len(got["missing_variables"].([]string)) != tc.missing || len(got["null_variables"].([]string)) != tc.nulls {
				t.Fatalf("unexpected preview: %#v", got)
			}
		})
	}
}

func TestEvaluatorPreviewUnsupportedBeforeRead(t *testing.T) {
	for _, mapping := range []map[string]string{nil, {"x": "all_messages"}, {"x": "input.messages[0]"}, {"x": "reference.answer"}} {
		if _, err := previewEvaluatorRun(context.Background(), nil, deleteTestProjectID, importRunID, mapping); err == nil {
			t.Fatal("accepted unsupported preview")
		}
	}
}
