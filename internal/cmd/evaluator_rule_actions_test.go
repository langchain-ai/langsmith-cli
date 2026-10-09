package cmd

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func decodeRule(t *testing.T, rule map[string]any) langsmith.Evaluator {
	t.Helper()
	var r langsmith.Evaluator
	if err := json.Unmarshal([]byte(mustJSON(t, rule)), &r); err != nil {
		t.Fatalf("decoding rule: %v", err)
	}
	return r
}

func TestRuleActions(t *testing.T) {
	base := func(extra map[string]any) map[string]any {
		rule := map[string]any{"id": testRuleID, "display_name": "r", "session_id": testSessionID}
		for k, v := range extra {
			rule[k] = v
		}
		return rule
	}
	tests := []struct {
		name string
		rule map[string]any
		want []string
	}{
		{"none", base(nil), []string{}},
		{"evaluator by id", base(map[string]any{"evaluator_id": testEvaluatorID}), []string{"evaluator"}},
		{"inline code evaluator", base(map[string]any{"code_evaluators": []any{map[string]any{"code": "x", "language": "python"}}}), []string{"evaluator"}},
		{"webhook", base(map[string]any{"webhooks": []any{map[string]any{"url": "https://example.com"}}}), []string{"webhook"}},
		{"add to dataset", base(map[string]any{"add_to_dataset_id": replaceDatasetID}), []string{"add_to_dataset"}},
		{"add to annotation queue", base(map[string]any{"add_to_annotation_queue_id": replaceQueueID}), []string{"add_to_annotation_queue"}},
		{"alert", base(map[string]any{"alerts": []any{map[string]any{"routing_key": "k"}}}), []string{"alert"}},
		{"evaluator and webhook", base(map[string]any{
			"evaluator_id": testEvaluatorID,
			"webhooks":     []any{map[string]any{"url": "https://example.com"}},
		}), []string{"evaluator", "webhook"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ruleActions(decodeRule(t, tt.rule)); !slices.Equal(got, tt.want) {
				t.Errorf("ruleActions = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaskWebhookURL(t *testing.T) {
	tests := map[string]string{
		"https://example.com":                                "https://example.com",
		"https://example.com/":                               "https://example.com",
		"https://user:pass@example.com/hook":                 "https://example.com/...",
		"https://example.com/hook?token=abc":                 "https://example.com/...",
		"https://hooks.example.com/services/T0/B0/secretval": "https://hooks.example.com/...",
		"https://example.com:8443/x#frag":                    "https://example.com:8443/...",
		"not a url":                                          "(hidden)",
		"":                                                   "(hidden)",
	}
	for in, want := range tests {
		if got := maskWebhookURL(in); got != want {
			t.Errorf("maskWebhookURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEvaluatorRuleGet_ShowsActionDetails(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.URL.Query().Get("id") == testRuleID {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": testRuleID, "display_name": "notify", "session_id": testSessionID, "session_name": "my-app",
				"sampling_rate": 0.1, "is_enabled": true,
				"evaluator_id": testEvaluatorID, "evaluator_name": "accuracy",
				"filter": "eq(is_root, true)", "trace_filter": "eq(name, \"agent\")", "tree_filter": "eq(run_type, \"llm\")",
				"group_by": "thread_id",
				"webhooks": []any{map[string]any{
					"url":     "https://user:pw-secret@example.com/hooks/path-secret?token=query-secret",
					"headers": map[string]any{"Authorization": "Bearer header-secret"},
				}},
				"add_to_dataset_id": replaceDatasetID, "add_to_dataset_name": "corrections", "add_to_dataset_prefer_correction": true,
				"add_to_annotation_queue_id": replaceQueueID, "add_to_annotation_queue_name": "review",
				"alerts":      []any{map[string]any{"routing_key": "routing-secret", "type": "pagerduty", "severity": "critical", "summary": "bad score"}},
				"spend_limit": map[string]any{"limit_usd": "25.00", "window": "weekly"},
			}})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		if err := runTestCommand(t, newEvaluatorRuleGetCmd(), []string{testRuleID}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	for _, secret := range []string{"pw-secret", "path-secret", "query-secret", "header-secret", "Authorization", "routing-secret"} {
		if strings.Contains(out, secret) {
			t.Errorf("output leaks %q:\n%s", secret, out)
		}
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	want := map[string]any{
		"actions":      []any{"evaluator", "webhook", "add_to_dataset", "add_to_annotation_queue", "alert"},
		"filter":       "eq(is_root, true)",
		"trace_filter": "eq(name, \"agent\")",
		"tree_filter":  "eq(run_type, \"llm\")",
		"group_by":     "thread_id",
		"webhooks":     []any{map[string]any{"url": "https://example.com/..."}},
		"add_to_dataset": map[string]any{
			"id": replaceDatasetID, "name": "corrections", "prefer_correction": true,
		},
		"add_to_annotation_queue": map[string]any{"id": replaceQueueID, "name": "review"},
		"alerts":                  []any{map[string]any{"type": "pagerduty", "severity": "critical", "summary": "bad score"}},
		"spend_limit":             map[string]any{"limit_usd": "25.00", "window": "weekly"},
	}
	for key, w := range want {
		if mustJSON(t, got[key]) != mustJSON(t, w) {
			t.Errorf("%s = %s, want %s", key, mustJSON(t, got[key]), mustJSON(t, w))
		}
	}
}

func TestEvaluatorRuleGet_OmitsUnsetActions(t *testing.T) {
	r := decodeRule(t, map[string]any{"id": testRuleID, "display_name": "r", "evaluator_id": testEvaluatorID})
	got := ruleDetail(r)
	for _, key := range []string{"webhooks", "add_to_dataset", "add_to_annotation_queue", "alerts", "spend_limit", "trace_filter", "tree_filter", "group_by"} {
		if _, ok := got[key]; ok {
			t.Errorf("expected no %s for a rule without it, got %v", key, got[key])
		}
	}
}

func TestEvaluatorRuleList_ShowsActions(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": testRuleID, "display_name": "graded", "evaluator_id": testEvaluatorID,
					"webhooks": []any{map[string]any{"url": "https://example.com/hook?token=secret"}}},
				{"id": testRuleID2, "display_name": "queue", "add_to_annotation_queue_id": replaceQueueID},
			})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newEvaluatorRuleListCmd()
		_ = cmd.Flags().Set("all", "true")
		if err := runTestCommand(t, cmd, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if strings.Contains(out, "secret") {
		t.Errorf("rule list should not print webhook URLs, got:\n%s", out)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
	if a := mustJSON(t, got[0]["actions"]); a != `["evaluator","webhook"]` {
		t.Errorf("first rule actions = %s", a)
	}
	if a := mustJSON(t, got[1]["actions"]); a != `["add_to_annotation_queue"]` {
		t.Errorf("second rule actions = %s", a)
	}
}
