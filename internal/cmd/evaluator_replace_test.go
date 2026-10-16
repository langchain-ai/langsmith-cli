package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

const (
	replaceRuleID    = "6f1d2c3b-4a59-4e8d-9c7b-1a2b3c4d5e6f"
	replaceDatasetID = "8a7b6c5d-4e3f-4a1b-9c8d-7e6f5a4b3c2d"
	replaceTargetDS  = "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e"
	replaceQueueID   = "4c5d6e7f-8a9b-4c0d-8e1f-2a3b4c5d6e7f"
)

// fullReplaceColumns are the rule fields PATCH /runs/rules/{id} overwrites from
// the request body, so an omitted field is cleared. Fields not listed here
// (group_by, spend_limit, retention flags, ...) are kept when omitted.
var fullReplaceColumns = []string{
	"display_name", "session_id", "dataset_id", "is_enabled", "filter",
	"trace_filter", "tree_filter", "sampling_rate", "add_to_annotation_queue_id",
	"add_to_dataset_id", "evaluators", "code_evaluators", "alerts", "webhooks",
	"add_to_dataset_prefer_correction", "extend_only", "include_extended_stats",
}

// retentionFlags need an extra permission to send on PATCH and are kept when omitted.
var retentionFlags = []string{
	"extend_dataset_trace_retention", "extend_annotation_queue_trace_retention",
	"extend_webhook_trace_retention", "extend_evaluator_trace_retention",
}

// fullReplaceRuleServer serves one stored rule and applies PATCH with the
// server's full-replace semantics.
type fullReplaceRuleServer struct {
	mu        sync.Mutex
	rule      map[string]any
	patchBody map[string]any
}

func (s *fullReplaceRuleServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/datasets" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": replaceTargetDS, "name": "eval-set"}})
		case r.URL.Path == "/api/v1/runs/rules" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{s.rule})
		case r.URL.Path == "/api/v1/runs/rules/"+replaceRuleID && r.Method == http.MethodPatch:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decoding patch body: %v", err)
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			if alerts, _ := body["alerts"].([]any); len(alerts) > 0 {
				http.Error(w, "Alerts are no longer supported via Run Rules", http.StatusUnprocessableEntity)
				return
			}
			s.patchBody = body
			for _, col := range fullReplaceColumns {
				if v, ok := body[col]; ok {
					s.rule[col] = v
				} else {
					delete(s.rule, col)
				}
			}
			_ = json.NewEncoder(w).Encode(s.rule)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}
}

// configuredRule is an evaluator rule with non-default settings on a dataset.
func configuredRule(evaluator map[string]any) map[string]any {
	rule := map[string]any{
		"id":                               replaceRuleID,
		"display_name":                     "accuracy",
		"dataset_id":                       replaceTargetDS,
		"dataset_name":                     "eval-set",
		"is_enabled":                       false,
		"sampling_rate":                    0.1,
		"filter":                           "eq(is_root, true)",
		"trace_filter":                     "eq(name, \"agent\")",
		"tree_filter":                      "eq(run_type, \"llm\")",
		"webhooks":                         []any{map[string]any{"url": "https://example.com/hook", "headers": map[string]any{"Authorization": "Bearer abc"}}},
		"add_to_dataset_id":                replaceDatasetID,
		"add_to_dataset_prefer_correction": true,
		"add_to_annotation_queue_id":       replaceQueueID,
		"include_extended_stats":           true,
		"extend_only":                      false,
		"alerts":                           []any{},
	}
	for k, v := range evaluator {
		rule[k] = v
	}
	return rule
}

func codeRule() map[string]any {
	return configuredRule(map[string]any{
		"code_evaluators": []any{map[string]any{"code": "def perform_eval(run, example):\n    return {}", "language": "python"}},
	})
}

func llmRule() map[string]any {
	return configuredRule(map[string]any{
		"evaluators": []any{map[string]any{"structured": map[string]any{"hub_ref": "my-org/old:latest"}}},
	})
}

func runUploadReplace(t *testing.T, srv *fullReplaceRuleServer, extra map[string]string) {
	t.Helper()
	evaluatorFile := t.TempDir() + "/eval.py"
	if err := os.WriteFile(evaluatorFile, []byte("def check(run, example):\n    return {\"score\": 1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := map[string]string{"name": "accuracy", "function": "check"}
	for k, v := range extra {
		flags[k] = v
	}
	runReplace(t, srv, newEvaluatorUploadCmd(), flags, []string{evaluatorFile})
}

func runCreateLLMReplace(t *testing.T, srv *fullReplaceRuleServer, extra map[string]string) {
	t.Helper()
	modelConfig := t.TempDir() + "/model.json"
	if err := os.WriteFile(modelConfig, []byte(`{"type":"chat"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := map[string]string{"name": "accuracy", "hub-ref": "my-org/new:latest", "model-config": modelConfig}
	for k, v := range extra {
		flags[k] = v
	}
	runReplace(t, srv, newEvaluatorCreateLLMCmd(), flags, nil)
}

func runReplace(t *testing.T, srv *fullReplaceRuleServer, cmd *cobra.Command, flags map[string]string, args []string) {
	t.Helper()
	ts := newTestServer(t, srv.handler(t))
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	for k, v := range map[string]string{"dataset": "eval-set", "replace": "true", "yes": "true"} {
		_ = cmd.Flags().Set(k, v)
	}
	for k, v := range flags {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatalf("setting --%s: %v", k, err)
		}
	}
	captureStdout(t, func() {
		if err := runTestCommand(t, cmd, args); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if srv.patchBody == nil {
		t.Fatal("expected a PATCH of the existing rule")
	}
}

func assertRuleSettingsKept(t *testing.T, srv *fullReplaceRuleServer, wantSampling float64) {
	t.Helper()
	got := srv.rule
	if got["sampling_rate"] != wantSampling {
		t.Errorf("sampling_rate = %v, want %v", got["sampling_rate"], wantSampling)
	}
	for field, want := range map[string]any{
		"filter":                           "eq(is_root, true)",
		"trace_filter":                     "eq(name, \"agent\")",
		"tree_filter":                      "eq(run_type, \"llm\")",
		"add_to_dataset_id":                replaceDatasetID,
		"add_to_dataset_prefer_correction": true,
		"add_to_annotation_queue_id":       replaceQueueID,
		"is_enabled":                       false,
		"include_extended_stats":           true,
		"display_name":                     "accuracy",
	} {
		if got[field] != want {
			t.Errorf("%s = %#v, want %#v", field, got[field], want)
		}
	}
	webhooks, _ := got["webhooks"].([]any)
	if len(webhooks) != 1 {
		t.Fatalf("webhooks = %#v, want the existing webhook", got["webhooks"])
	}
	hook, _ := webhooks[0].(map[string]any)
	headers, _ := hook["headers"].(map[string]any)
	if hook["url"] != "https://example.com/hook" || headers["Authorization"] != "Bearer abc" {
		t.Errorf("webhook = %#v, want URL and headers unchanged", hook)
	}
	for _, flag := range retentionFlags {
		if _, ok := srv.patchBody[flag]; ok {
			t.Errorf("PATCH sent %s; omitting it keeps the stored value and sending it needs an extra permission", flag)
		}
	}
	if _, ok := srv.patchBody["evaluator_id"]; ok {
		t.Error("PATCH must not send evaluator_id alongside an inline evaluator")
	}
}

func TestEvaluatorUploadReplaceKeepsRuleSettings(t *testing.T) {
	srv := &fullReplaceRuleServer{rule: codeRule()}
	runUploadReplace(t, srv, nil)
	assertRuleSettingsKept(t, srv, 0.1)
	code, _ := srv.rule["code_evaluators"].([]any)
	if len(code) != 1 || !strings.Contains(code[0].(map[string]any)["code"].(string), "def perform_eval(") {
		t.Errorf("expected the new code to replace the old, got %#v", srv.rule["code_evaluators"])
	}
}

func TestEvaluatorCreateLLMReplaceKeepsRuleSettings(t *testing.T) {
	srv := &fullReplaceRuleServer{rule: llmRule()}
	runCreateLLMReplace(t, srv, nil)
	assertRuleSettingsKept(t, srv, 0.1)
	evaluators, _ := srv.rule["evaluators"].([]any)
	if len(evaluators) != 1 || !strings.Contains(mustJSON(t, evaluators[0]), "my-org/new:latest") {
		t.Errorf("expected the new prompt to replace the old, got %#v", srv.rule["evaluators"])
	}
}

func TestEvaluatorReplaceExplicitFlagsOverride(t *testing.T) {
	for name, run := range map[string]func(*testing.T, *fullReplaceRuleServer, map[string]string){
		"upload":     runUploadReplace,
		"create-llm": runCreateLLMReplace,
	} {
		t.Run(name, func(t *testing.T) {
			rule := codeRule()
			if name == "create-llm" {
				rule = llmRule()
			}
			srv := &fullReplaceRuleServer{rule: rule}
			run(t, srv, map[string]string{"sampling-rate": "0.5", "trace-filter": "eq(name, \"other\")"})
			if srv.rule["sampling_rate"] != 0.5 {
				t.Errorf("sampling_rate = %v, want 0.5", srv.rule["sampling_rate"])
			}
			if srv.rule["trace_filter"] != "eq(name, \"other\")" {
				t.Errorf("trace_filter = %v, want the --trace-filter value", srv.rule["trace_filter"])
			}
			if srv.rule["filter"] != "eq(is_root, true)" {
				t.Errorf("filter = %v, want it kept", srv.rule["filter"])
			}
		})
	}
}

func TestEvaluatorReplaceRefusesRuleWithAlerts(t *testing.T) {
	rule := codeRule()
	rule["alerts"] = []any{map[string]any{"routing_key": "k", "type": "pagerduty"}}
	srv := &fullReplaceRuleServer{rule: rule}
	ts := newTestServer(t, srv.handler(t))
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	evaluatorFile := t.TempDir() + "/eval.py"
	if err := os.WriteFile(evaluatorFile, []byte("def check(run, example):\n    return {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newEvaluatorUploadCmd()
	for k, v := range map[string]string{"name": "accuracy", "function": "check", "dataset": "eval-set", "replace": "true", "yes": "true"} {
		_ = cmd.Flags().Set(k, v)
	}
	err := runTestCommand(t, cmd, []string{evaluatorFile})
	if err == nil || !strings.Contains(err.Error(), "alert") {
		t.Fatalf("expected a refusal naming the alerts, got %v", err)
	}
	if srv.patchBody != nil {
		t.Error("expected no PATCH")
	}
}

func TestEvaluatorRuleDeleteConfirmationListsOtherActions(t *testing.T) {
	var sawDelete bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			sawDelete = true
		}
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == http.MethodGet {
			rule := codeRule()
			rule["add_to_dataset_name"] = "corrections"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{rule})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	cmd := newEvaluatorRuleDeleteCmd()
	var stderr strings.Builder
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader("n\n"))
	err := runTestCommand(t, cmd, []string{replaceRuleID})
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected the declined prompt to abort, got %v", err)
	}
	if sawDelete {
		t.Error("expected no DELETE request")
	}
	prompt := stderr.String()
	for _, want := range []string{"1 webhook(s)", `add to dataset "corrections"`, "add to annotation queue " + replaceQueueID} {
		if !strings.Contains(prompt, want) {
			t.Errorf("confirmation should mention %q, got:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "example.com") || strings.Contains(prompt, "Bearer") {
		t.Errorf("confirmation should not print webhook URLs or headers, got:\n%s", prompt)
	}
}

func TestEvaluatorDeleteConfirmationListsRuleActions(t *testing.T) {
	var sawDelete bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodDelete:
			sawDelete = true
		case r.URL.Path == "/api/v1/platform/evaluators/"+testEvaluatorID:
			_ = json.NewEncoder(w).Encode(readyForTaskGrade())
		case r.URL.Path == "/api/v1/runs/rules" && r.URL.Query().Get("evaluator_id") == testEvaluatorID:
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": testRuleID, "display_name": "test", "session_id": testSessionID, "session_name": "my-app",
				"evaluator_id": testEvaluatorID,
				"webhooks":     []map[string]any{{"url": "https://example.com/hook"}},
			}})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	cmd := newEvaluatorDeleteCmd()
	_ = cmd.Flags().Set("delete-rules", "true")
	var stderr strings.Builder
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader("n\n"))
	if err := runTestCommand(t, cmd, []string{testEvaluatorID}); err == nil {
		t.Fatal("expected the declined prompt to abort")
	}
	if sawDelete {
		t.Error("expected no DELETE request")
	}
	if prompt := stderr.String(); !strings.Contains(prompt, `rule "test" (project my-app): 1 webhook(s)`) {
		t.Errorf("confirmation should list the rule's webhook, got:\n%s", prompt)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
