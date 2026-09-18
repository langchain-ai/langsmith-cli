package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func insightsTestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "analysis.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInsightsJSONValidation(t *testing.T) {
	for _, content := range []string{`null`, `[]`, `{} {}`, `{"sample":"20"}`, `{"validate_model_secrets":false}`, `{"is_scheduled":true}`, `{"attribute_schemas":{"x":{"unknown":true}}}`, "{" + strings.Repeat(" ", 1024*1024) + "}"} {
		var config insightsFileConfig
		if err := readInsightsJSON(insightsTestFile(t, content), &config); err == nil {
			t.Fatalf("accepted invalid file of length %d", len(content))
		}
	}
	for _, change := range []func(*insightsCreateOptions){
		func(o *insightsCreateOptions) { o.categories = map[string]string{} },
		func(o *insightsCreateOptions) { o.categories = map[string]string{"": "x"} },
		func(o *insightsCreateOptions) { o.categories = map[string]string{"x": " "} },
		func(o *insightsCreateOptions) {
			o.categories = map[string]string{}
			for i := 0; i < 11; i++ {
				o.categories[fmt.Sprint(i)] = "description"
			}
		},
		func(o *insightsCreateOptions) {
			o.attributes = map[string]insightsAttribute{"x": {Type: "integer", Description: "x"}}
		},
		func(o *insightsCreateOptions) { o.clusterModel = "https://untrusted.example" },
		func(o *insightsCreateOptions) { o.configID = "invalid" },
	} {
		o := insightsCreateOptions{model: "openai", sample: 5, lastNHours: 24}
		change(&o)
		if _, err := insightsCreateParams(o); err == nil {
			t.Fatal("accepted invalid analysis settings")
		}
	}
}

func TestInsightsCreateFileDryRunAndApply(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["partitions"].(map[string]any)["Refunds"] != "Refund requests" || body["cluster_model"] != "anthropic" {
			t.Errorf("analysis fields not forwarded")
		}
		if body["attribute_schemas"].(map[string]any)["resolved"].(map[string]any)["type"] != "boolean" {
			t.Error("attribute missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"job","name":"analysis","status":"queued"}`))
	})
	defer setupTestEnv(t, ts.URL)()
	file := insightsTestFile(t, `{"model":"openai","sample":5,"last_n_hours":24,"cluster_model":"anthropic","partitions":{"Refunds":"Refund requests"},"attribute_schemas":{"resolved":{"type":"boolean","description":"Request resolved","filter_by":true}}}`)
	for _, dry := range []bool{true, false} {
		cmd := newInsightsCreateCmd()
		args := []string{"--project-id", deleteTestProjectID, "--file", file}
		if dry {
			args = append(args, "--dry-run")
		}
		cmd.SetArgs(args)
		var err error
		out := captureStdout(t, func() { err = cmd.Execute() })
		if err != nil {
			t.Fatal(err)
		}
		if dry && (calls != 0 || !strings.Contains(out, `"dry_run"`)) {
			t.Fatalf("dry run created job or omitted status: %s", out)
		}
	}
	if calls != 1 {
		t.Fatalf("want one POST, got %d", calls)
	}
}

func TestInsightsCreateSavedConfigAndConflicts(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body) != 1 || body["config_id"] != deleteTestProjectID {
			t.Errorf("saved config must not carry overrides: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"job","status":"queued"}`))
	})
	defer setupTestEnv(t, ts.URL)()
	cmd := newInsightsCreateCmd()
	cmd.SetArgs([]string{"--project-id", deleteTestProjectID, "--config-id", deleteTestProjectID})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--config-id", deleteTestProjectID, "--sample", "5"},
		{"--file", "unused.json", "--model", "openai"},
		{"--config-id", " "},
		{"--model", "openai", "--sample", "0", "--last-n-hours", "24"},
	} {
		cmd := newInsightsCreateCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted conflicting args: %v", args)
		}
	}
	if calls != 1 {
		t.Fatal("invalid requests reached service")
	}
}
