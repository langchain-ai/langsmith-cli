package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ==================== Command structure ====================

func TestInsightsCmd_Subcommands(t *testing.T) {
	cmd := newInsightsCmd()
	expected := map[string]bool{"create": false, "list": false, "get": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("insights missing subcommand %q", name)
		}
	}
}

// ==================== insights create ====================

func TestInsightsCreateCmd_Flags(t *testing.T) {
	cmd := newInsightsCreateCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"project", "", ""},
		{"project-id", "", ""},
		{"name", "", ""},
		{"summary-prompt", "", ""},
		{"summary-prompt-file", "", ""},
		{"since", "", ""},
		{"before", "", ""},
		{"last-n-hours", "0", ""},
		{"filter", "", ""},
		{"sample", "0", ""},
		{"model", "openai", ""},
		{"cluster-model", "", ""},
		{"summary-model", "", ""},
		{"partitions", "", ""},
		{"attribute-schemas", "", ""},
		{"output", "", "o"},
	}
	for _, tc := range tests {
		f := cmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("flag --%s not found", tc.name)
			continue
		}
		if f.DefValue != tc.defVal {
			t.Errorf("flag --%s: expected default %q, got %q", tc.name, tc.defVal, f.DefValue)
		}
		if f.Shorthand != tc.short {
			t.Errorf("flag --%s: expected shorthand %q, got %q", tc.name, tc.short, f.Shorthand)
		}
	}
	if _, ok := cmd.Flags().Lookup("name").Annotations["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("--name should be required")
	}
}

func TestBuildInsightCreateRequest(t *testing.T) {
	opts := insightCreateOptions{
		name:             "Reliability review",
		summaryPrompt:    "Diagnose {{run.inputs}} and {{run.error}}",
		since:            "2026-09-01",
		before:           "2026-09-08T12:00:00Z",
		filter:           "eq(is_root, true)",
		sample:           0.25,
		sampleSet:        true,
		model:            "anthropic",
		clusterModel:     "claude-thinking",
		summaryModel:     "claude-fast",
		partitions:       `{"environment":"metadata.env"}`,
		attributeSchemas: `{"failure":{"type":"boolean"}}`,
	}

	request, err := buildInsightCreateRequest(opts)
	if err != nil {
		t.Fatalf("buildInsightCreateRequest: %v", err)
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	wantValues := map[string]any{
		"name":           "Reliability review",
		"summary_prompt": "Diagnose {{run.inputs}} and {{run.error}}",
		"filter":         "eq(is_root, true)",
		"sample":         0.25,
		"model":          "anthropic",
		"cluster_model":  "claude-thinking",
		"summary_model":  "claude-fast",
	}
	for key, want := range wantValues {
		if got[key] != want {
			t.Errorf("%s: expected %#v, got %#v", key, want, got[key])
		}
	}
	if got["start_time"] != "2026-09-01T00:00:00Z" {
		t.Errorf("unexpected start_time: %#v", got["start_time"])
	}
	if got["end_time"] != "2026-09-08T12:00:00Z" {
		t.Errorf("unexpected end_time: %#v", got["end_time"])
	}
	if got["partitions"].(map[string]any)["environment"] != "metadata.env" {
		t.Errorf("unexpected partitions: %#v", got["partitions"])
	}
	if got["attribute_schemas"].(map[string]any)["failure"] == nil {
		t.Errorf("unexpected attribute_schemas: %#v", got["attribute_schemas"])
	}
}

func TestBuildInsightCreateRequest_LastNHours(t *testing.T) {
	request, err := buildInsightCreateRequest(insightCreateOptions{
		summaryPrompt: "Summarize {{run.inputs}}",
		model:         "openai",
		lastNHours:    24,
		lastNHoursSet: true,
	})
	if err != nil {
		t.Fatalf("buildInsightCreateRequest: %v", err)
	}
	data, _ := json.Marshal(request)
	if !strings.Contains(string(data), `"last_n_hours":24`) {
		t.Fatalf("expected last_n_hours in request: %s", data)
	}
}

func TestBuildInsightCreateRequest_Validation(t *testing.T) {
	tests := []struct {
		name string
		opts insightCreateOptions
		want string
	}{
		{
			name: "prompt required",
			opts: insightCreateOptions{model: "openai"},
			want: "one of --summary-prompt or --summary-prompt-file is required",
		},
		{
			name: "invalid model",
			opts: insightCreateOptions{summaryPrompt: "{{run.inputs}}", model: "other"},
			want: "must be openai or anthropic",
		},
		{
			name: "models paired",
			opts: insightCreateOptions{summaryPrompt: "{{run.inputs}}", model: "openai", clusterModel: "heavy"},
			want: "must be specified together",
		},
		{
			name: "before requires since",
			opts: insightCreateOptions{summaryPrompt: "{{run.inputs}}", model: "openai", before: "2026-09-08"},
			want: "--before requires --since",
		},
		{
			name: "time order",
			opts: insightCreateOptions{summaryPrompt: "{{run.inputs}}", model: "openai", since: "2026-09-08", before: "2026-09-01"},
			want: "--before must be after --since",
		},
		{
			name: "positive last hours",
			opts: insightCreateOptions{summaryPrompt: "{{run.inputs}}", model: "openai", lastNHoursSet: true},
			want: "--last-n-hours must be greater than 0",
		},
		{
			name: "positive sample",
			opts: insightCreateOptions{summaryPrompt: "{{run.inputs}}", model: "openai", sampleSet: true},
			want: "--sample must be greater than 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildInsightCreateRequest(tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestBuildInsightCreateRequest_ReadsFiles(t *testing.T) {
	dir := t.TempDir()
	promptPath := filepath.Join(dir, "summary.txt")
	attributesPath := filepath.Join(dir, "attributes.json")
	if err := os.WriteFile(promptPath, []byte("Classify {{run.outputs}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attributesPath, []byte(`{"topic":{"type":"string"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	request, err := buildInsightCreateRequest(insightCreateOptions{
		summaryPromptFile: promptPath,
		attributeSchemas:  "@" + attributesPath,
		model:             "openai",
	})
	if err != nil {
		t.Fatalf("buildInsightCreateRequest: %v", err)
	}
	data, _ := json.Marshal(request)
	if !strings.Contains(string(data), `"summary_prompt":"Classify {{run.outputs}}\n"`) {
		t.Fatalf("expected file prompt in request: %s", data)
	}
	if !strings.Contains(string(data), `"attribute_schemas":{"topic":{"type":"string"}}`) {
		t.Fatalf("expected file attributes in request: %s", data)
	}
}

func TestInsightsCreateCmd_PostsRequestAndPrintsJSON(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	const insightID = "0199321d-e2b4-7000-8000-000000000003"
	var configRequestBody map[string]any
	var jobRequestBody map[string]any
	var requestPaths []string
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		requestPaths = append(requestPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/sessions/" + projectID + "/insights/configs":
			if err := json.NewDecoder(r.Body).Decode(&configRequestBody); err != nil {
				t.Errorf("decode config request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": configID, "name": "Reliability review", "description": nil,
				"config": map[string]any{
					"name": "Reliability review", "hierarchy": nil, "partitions": nil,
					"sample": nil, "summary_prompt": "Diagnose {{run.error}}",
					"filter": nil, "attribute_schemas": nil, "model": "openai",
				},
				"schedule_cron": nil,
			})
		case "/api/v1/sessions/" + projectID + "/insights":
			if err := json.NewDecoder(r.Body).Decode(&jobRequestBody); err != nil {
				t.Errorf("decode job request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": insightID, "name": "Reliability review", "status": "pending", "error": nil,
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer setupTestEnv(t, server.URL)()
	flagOutputFormat = "json"

	cmd := newInsightsCreateCmd()
	cmd.SetArgs([]string{
		"--project-id", projectID,
		"--name", "Reliability review",
		"--summary-prompt", "Diagnose {{run.error}}",
	})
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.Execute()
	})
	if runErr != nil {
		t.Fatalf("execute create: %v", runErr)
	}
	if len(requestPaths) != 2 || !strings.HasSuffix(requestPaths[0], "/insights/configs") || !strings.HasSuffix(requestPaths[1], "/insights") {
		t.Fatalf("unexpected request order: %#v", requestPaths)
	}
	if configRequestBody["name"] != "Reliability review" {
		t.Errorf("unexpected config request name: %#v", configRequestBody)
	}
	config, ok := configRequestBody["config"].(map[string]any)
	if !ok || config["summary_prompt"] != "Diagnose {{run.error}}" {
		t.Errorf("unexpected config request: %#v", configRequestBody)
	}
	if len(jobRequestBody) != 1 || jobRequestBody["config_id"] != configID {
		t.Errorf("unexpected job request: %#v", jobRequestBody)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse output %q: %v", out, err)
	}
	if result["id"] != insightID || result["config_id"] != configID || result["project_id"] != projectID || result["status"] != "pending" {
		t.Errorf("unexpected output: %#v", result)
	}
}

func TestInsightsCreateCmd_RejectsBlankName(t *testing.T) {
	cmd := newInsightsCreateCmd()
	cmd.SetArgs([]string{
		"--project-id", "0199321d-e2b4-7000-8000-000000000001",
		"--name", "   ",
		"--summary-prompt", "Diagnose {{run.error}}",
	})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--name must not be empty") {
		t.Fatalf("expected blank name error, got %v", err)
	}
}

func TestInsightsCreateCmd_ReportsSavedConfigWhenJobStartFails(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/insights/configs") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": configID, "name": "Reliability review", "description": nil,
				"config": map[string]any{
					"name": "Reliability review", "hierarchy": nil, "partitions": nil,
					"sample": nil, "summary_prompt": "Diagnose {{run.error}}",
					"filter": nil, "attribute_schemas": nil, "model": "openai",
				},
			})
			return
		}
		http.Error(w, `{"detail":"failed to start"}`, http.StatusInternalServerError)
	})
	defer setupTestEnv(t, server.URL)()

	cmd := newInsightsCreateCmd()
	cmd.SetArgs([]string{
		"--project-id", projectID,
		"--name", "Reliability review",
		"--summary-prompt", "Diagnose {{run.error}}",
	})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "insight config "+configID+" was saved") {
		t.Fatalf("expected saved config ID in error, got %v", err)
	}
}

func TestInsightsCmd_UseField(t *testing.T) {
	cmd := newInsightsCmd()
	if cmd.Use != "insights" {
		t.Errorf("expected Use=insights, got %q", cmd.Use)
	}
}

// ==================== insights list flags ====================

func TestInsightsListCmd_Flags(t *testing.T) {
	cmd := newInsightsListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"project", "", ""},
		{"limit", "0", "n"},
		{"output", "", "o"},
	}
	for _, tc := range tests {
		f := cmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("flag --%s not found", tc.name)
			continue
		}
		if f.DefValue != tc.defVal {
			t.Errorf("flag --%s: expected default %q, got %q", tc.name, tc.defVal, f.DefValue)
		}
		if tc.short != "" && f.Shorthand != tc.short {
			t.Errorf("flag --%s: expected shorthand %q, got %q", tc.name, tc.short, f.Shorthand)
		}
	}
}

func TestInsightsListCmd_ProjectNotCobraRequired(t *testing.T) {
	cmd := newInsightsListCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	ann := f.Annotations
	if ann != nil {
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; ok {
			t.Error("--project should not be marked as cobra-required; use ResolveProject instead")
		}
	}
}

func TestInsightsListCmd_ProjectEnvFallback(t *testing.T) {
	t.Setenv("LANGSMITH_PROJECT", "env-project")
	result := ResolveProject("")
	if result != "env-project" {
		t.Errorf("expected ResolveProject to return env-project, got %q", result)
	}
}

func TestInsightsListCmd_ProjectFlagHelpMentionsEnv(t *testing.T) {
	cmd := newInsightsListCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	if f.Usage != "Project name [env: LANGSMITH_PROJECT]" {
		t.Errorf("expected project flag usage to mention env var, got %q", f.Usage)
	}
}

// ==================== insights get flags ====================

func TestInsightsGetCmd_Flags(t *testing.T) {
	cmd := newInsightsGetCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"project", "", ""},
		{"output", "", "o"},
	}
	for _, tc := range tests {
		f := cmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("flag --%s not found", tc.name)
			continue
		}
		if f.DefValue != tc.defVal {
			t.Errorf("flag --%s: expected default %q, got %q", tc.name, tc.defVal, f.DefValue)
		}
		if tc.short != "" && f.Shorthand != tc.short {
			t.Errorf("flag --%s: expected shorthand %q, got %q", tc.name, tc.short, f.Shorthand)
		}
	}
}

func TestInsightsGetCmd_ExactArgs(t *testing.T) {
	cmd := newInsightsGetCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"insight-123"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestInsightsGetCmd_ProjectNotCobraRequired(t *testing.T) {
	cmd := newInsightsGetCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	ann := f.Annotations
	if ann != nil {
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; ok {
			t.Error("--project should not be marked as cobra-required; use ResolveProject instead")
		}
	}
}

func TestInsightsGetCmd_ProjectFlagHelpMentionsEnv(t *testing.T) {
	cmd := newInsightsGetCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	if f.Usage != "Project name [env: LANGSMITH_PROJECT]" {
		t.Errorf("expected project flag usage to mention env var, got %q", f.Usage)
	}
}

// ==================== Helper functions ====================

func TestFormatShape_Nil(t *testing.T) {
	if got := formatShape(nil); got != "N/A" {
		t.Errorf("expected N/A for nil shape, got %q", got)
	}
}

func TestFormatShape_Empty(t *testing.T) {
	if got := formatShape(map[string]int64{}); got != "N/A" {
		t.Errorf("expected N/A for empty shape, got %q", got)
	}
}

func TestFormatShape_WithData(t *testing.T) {
	shape := map[string]int64{"Tooling": 19, "Integrations": 53}
	got := formatShape(shape)
	expected := "Integrations:53, Tooling:19"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatInsightTime_Zero(t *testing.T) {
	if got := formatInsightTime(time.Time{}); got != "N/A" {
		t.Errorf("expected N/A for zero time, got %q", got)
	}
}

func TestFormatInsightTime_Valid(t *testing.T) {
	ts := time.Date(2026, 3, 17, 12, 58, 12, 0, time.UTC)
	got := formatInsightTime(ts)
	expected := "2026-03-17 12:58"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
