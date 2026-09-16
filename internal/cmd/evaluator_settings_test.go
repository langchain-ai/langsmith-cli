package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestEvaluatorSettingsValidation(t *testing.T) {
	for _, args := range [][]string{
		{"--sampling-rate", "NaN"}, {"--sampling-rate", "1.1"}, {"--sampling-rate", "-1"},
		{"--group-by", "bad"}, {"--spend-limit", "0"}, {"--spend-limit", "Inf"},
		{"--backfill-from", "yesterday"}, {"--group-by", "thread_id", "--include-extended-stats"},
	} {
		cmd := newEvaluatorCreateLLMCmd()
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatal(err)
		}
		if err := applyEvaluatorSettings(cmd, map[string]any{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, args := range [][]string{{"--group-by", "thread_id"}, {"--extend-trace-retention=false"}} {
		cmd := newEvaluatorCreateLLMCmd()
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatal(err)
		}
		if err := applyEvaluatorSettings(cmd, map[string]any{"dataset_id": "dataset"}); err == nil {
			t.Fatalf("accepted dataset with %v", args)
		}
	}
}

func TestEvaluatorAdvancedSettingsRequest(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(fmt.Sprint(dryRun), func(t *testing.T) {
			writes := 0
			var body map[string]any
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "GET" {
					fmt.Fprint(w, `[]`)
					return
				}
				writes++
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				body["id"] = workflowExample
				json.NewEncoder(w).Encode(body)
			})
			defer setupTestEnv(t, ts.URL)()
			model := workflowFile(t, `{"secret":"never-preview-model"}`)
			cmd := newEvaluatorCreateLLMCmd()
			args := []string{"--name", "thread-judge", "--project-id", workflowDataset, "--hub-ref", "test/judge", "--model-config", model,
				"--group-by", "thread_id", "--sampling-rate", "0.25", "--enabled=false", "--filter", "filter-expression", "--trace-filter", "trace-expression", "--tree-filter", "tree-expression",
				"--extend-trace-retention=false", "--trace-evaluator-runs=false", "--backfill-from", "2026-09-16T00:00:00.123456Z", "--spend-limit", "1"}
			if dryRun {
				args = append(args, "--dry-run")
			}
			cmd.SetArgs(args)
			out := captureStdout(t, func() {
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			})
			if strings.Contains(out, "never-preview-model") {
				t.Fatal("model configuration leaked")
			}
			var result struct{ Settings map[string]any }
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			settings := result.Settings
			for key, want := range map[string]any{"group_by": "thread_id", "sampling_rate": 0.25, "is_enabled": false, "filter": "filter-expression", "trace_filter": "trace-expression", "tree_filter": "tree-expression", "extend_evaluator_trace_retention": false, "is_tracing_disabled": true, "backfill_from": "2026-09-16T00:00:00.123456Z"} {
				if settings[key] != want {
					t.Errorf("%s=%v want %v", key, settings[key], want)
				}
			}
			spend, ok := settings["spend_limit"].(map[string]any)
			if !ok || spend["limit_usd"] != float64(1) || spend["window"] != "weekly" {
				t.Fatal(settings)
			}
			if dryRun && writes != 0 || !dryRun && writes != 1 {
				t.Fatal(writes)
			}
		})
	}
}

func TestEvaluatorSettingsOmittedAndCleared(t *testing.T) {
	cmd := newEvaluatorCreateLLMCmd()
	payload := map[string]any{}
	if err := applyEvaluatorSettings(cmd, payload); err != nil || len(payload) != 0 {
		t.Fatal(payload, err)
	}
	if err := cmd.ParseFlags([]string{"--group-by", "none", "--filter=", "--trace-filter=", "--enabled=false"}); err != nil {
		t.Fatal(err)
	}
	if err := applyEvaluatorSettings(cmd, payload); err != nil {
		t.Fatal(err)
	}
	if value, ok := payload["group_by"]; !ok || value != nil {
		t.Fatal(payload)
	}
	if value, ok := payload["filter"]; !ok || value != "" {
		t.Fatal(payload)
	}
}

func TestEvaluatorReplacePreservesSettings(t *testing.T) {
	for _, exists := range []bool{false, true} {
		t.Run(fmt.Sprint(exists), func(t *testing.T) {
			var body map[string]any
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "GET" {
					if exists {
						fmt.Fprintf(w, `[{"id":%q,"display_name":"judge","session_id":%q,"sampling_rate":0.2,"is_enabled":false,"group_by":"thread_id"}]`, workflowExample, workflowDataset)
					} else {
						fmt.Fprint(w, `[]`)
					}
					return
				}
				if (r.Method == "PATCH") != exists {
					t.Error(r.Method)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				fmt.Fprintf(w, `{"id":%q}`, workflowExample)
			})
			defer setupTestEnv(t, ts.URL)()
			cmd := newEvaluatorCreateLLMCmd()
			cmd.SetArgs([]string{"--name", "judge", "--project-id", workflowDataset, "--hub-ref", "test/judge", "--model-config", workflowFile(t, `{}`), "--replace", "--yes"})
			out := captureStdout(t, func() {
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			})
			for _, key := range []string{"sampling_rate", "is_enabled", "include_extended_stats"} {
				if _, sent := body[key]; sent == exists {
					t.Errorf("unexpected %s presence: %v", key, body)
				}
			}
			if _, sent := body["group_by"]; sent {
				t.Fatal("replacement reset thread mode")
			}
			want := "created"
			if exists {
				want = "updated"
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(out), &result); err != nil || result["status"] != want {
				t.Fatal(out)
			}
		})
	}
}

func TestProjectConfigureIdleTime(t *testing.T) {
	for _, mode := range []string{"read", "dry-run", "apply", "mismatch"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "PATCH" {
					writes++
					var body struct{ Extra map[string]any }
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body.Extra["other"] != "preserve" || body.Extra["thread_idle_seconds"] != float64(600) {
						t.Error(body)
					}
				}
				seconds := 300
				if writes > 0 && mode != "mismatch" {
					seconds = 600
				}
				fmt.Fprintf(w, `{"id":%q,"extra":{"thread_idle_seconds":%d,"other":"preserve"}}`, workflowDataset, seconds)
			})
			defer setupTestEnv(t, ts.URL)()
			cmd := newProjectConfigureCmd()
			args := []string{"--project-id", workflowDataset}
			if mode != "read" {
				flag := "--apply"
				if mode == "dry-run" {
					flag = "--dry-run"
				}
				args = append(args, "--thread-idle-seconds", "600", flag)
			}
			cmd.SetArgs(args)
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if (err != nil) != (mode == "mismatch") {
				t.Fatal(err, out)
			}
			if (writes == 1) != (mode == "apply" || mode == "mismatch") {
				t.Fatal(writes)
			}
		})
	}
}
