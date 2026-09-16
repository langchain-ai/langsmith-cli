package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestInsightsPromptVariables(t *testing.T) {
	for _, prompt := range []string{"", "Summarize failures", "{{var1}}", "{{run.inputs", "{{#run.inputs}}"} {
		if _, err := insightsPromptVariables(prompt); err == nil {
			t.Errorf("accepted invalid prompt %q", prompt)
		}
	}
	paths, err := insightsPromptVariables("{{run.inputs.question}} {{{run.outputs}}} {{all_thread_messages}} {{run.outputs}}")
	if err != nil || len(paths) != 3 {
		t.Fatalf("paths = %v, error = %v", paths, err)
	}
}

func TestInsightsConfigBoundaries(t *testing.T) {
	for _, sample := range []int64{0, 1, 1000, 1001} {
		_, err := insightsCreateParams(insightsCreateOptions{model: "openai", sample: sample, lastNHours: 24})
		if (err == nil) != (sample >= 1 && sample <= 1000) {
			t.Errorf("sample %d: %v", sample, err)
		}
	}
	for _, content := range []string{`{"sample":1,"sample":2}`, `{"summary_prompt":""}`, `{"partitions":{"A":"one"," A ":"two"}}`, `{"attribute_schemas":{"has space":{"type":"boolean","description":"test"}}}`} {
		var config insightsFileConfig
		err := readInsightsJSON(insightsTestFile(t, content), &config)
		if err == nil {
			o := config.options()
			o.model, o.sample, o.lastNHours = "openai", 1, 24
			_, err = insightsCreateParams(o)
		}
		if err == nil {
			t.Errorf("accepted invalid configuration: %s", content)
		}
	}
}

func TestInsightsPreviewReadOnly(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/v1/runs/query" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["is_root"] != true || !strings.Contains(stringifyPreviewBody(body["session"]), deleteTestProjectID) {
			t.Errorf("missing project/root scope: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"runs":[{"id":"` + deleteTestProjectID + `","inputs":{"zero":0,"false":false,"null":null,"large":9007199254740993},"outputs":{}}]}`))
	})
	defer setupTestEnv(t, ts.URL)()
	flagOutputFormat = "json"
	cmd := newInsightsCreateCmd()
	cmd.SetArgs([]string{"--project-id", deleteTestProjectID, "--model", "openai", "--sample", "20", "--last-n-hours", "24", "--dry-run", "--preview-run", deleteTestProjectID, "--summary-prompt", "{{run.inputs.zero}} {{run.inputs.false}} {{run.inputs.null}} {{run.inputs.large}} {{run.inputs.missing}} {{all_thread_messages}} {{run.unsupported_field}}"})
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"run.inputs.zero": 0`, `"run.inputs.false": false`, `"run.inputs.null": null`, `"missing_paths"`, `"run.inputs.missing"`, `"unchecked_paths"`, `"paths_validated": false`} {
		if !strings.Contains(out, expected) {
			t.Errorf("missing %s in %s", expected, out)
		}
	}
	if calls != 1 {
		t.Errorf("expected one read, got %d", calls)
	}
	var result struct {
		Preview struct {
			Bindings  map[string]json.RawMessage `json:"bindings"`
			Missing   []string                   `json:"missing_paths"`
			Unchecked []string                   `json:"unchecked_paths"`
		} `json:"preview"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if string(result.Preview.Bindings["run.inputs.large"]) != "9007199254740993" {
		t.Error("preview rounded a large integer")
	}
	if len(result.Preview.Missing) != 1 || len(result.Preview.Unchecked) != 2 {
		t.Errorf("incorrect missing/unchecked classification: %+v", result.Preview)
	}
}

func TestInsightsPrettyMissingAndZeroMetrics(t *testing.T) {
	for _, tc := range []struct {
		name, stats string
		wantZero    bool
	}{
		{"missing", `{}`, false},
		{"null", `{"error_rate":null,"latency_p50":null,"cost_p50":null}`, false},
		{"zero", `{"error_rate":0,"latency_p50":0,"cost_p50":0}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var detail langsmith.SessionInsightGetJobResponse
			if err := json.Unmarshal([]byte(`{"clusters":[{"name":"Test","stats":`+tc.stats+`}]}`), &detail); err != nil {
				t.Fatal(err)
			}
			out := captureStdout(t, func() { printInsightPretty(&detail) })
			if strings.Contains(out, "0.0%") != tc.wantZero || strings.Contains(out, "$0.0000") != tc.wantZero {
				t.Errorf("incorrect metric rendering: %s", out)
			}
		})
	}
}

func stringifyPreviewBody(value any) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func TestInsightsPreviewRejectsInvalidFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--preview-run", deleteTestProjectID},
		{"--preview-run", "invalid", "--dry-run"},
		{"--preview-run", deleteTestProjectID, "--dry-run", "--config-id", deleteTestProjectID},
	} {
		cmd := newInsightsCreateCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("accepted invalid flags: %v", args)
		}
	}
}
