package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
)

// ==================== Command structure ====================

func TestInsightsCmd_Subcommands(t *testing.T) {
	cmd := newInsightsCmd()
	expected := map[string]bool{"list": false, "get": false, "create": false, "runs": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; !ok {
			t.Errorf("unexpected insights subcommand %q", sub.Name())
		} else {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("insights missing subcommand %q", name)
		}
	}
}

func TestInsightsCmd_HelpDescribesCreateAndQuery(t *testing.T) {
	cmd := newInsightsCmd()
	const description = "Create and query insight reports for a project"
	if cmd.Short != description {
		t.Errorf("expected Short=%q, got %q", description, cmd.Short)
	}
	if !strings.HasPrefix(cmd.Long, description+".") {
		t.Errorf("expected Long to start with %q, got %q", description+".", cmd.Long)
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
		{"config", "", ""},
		{"wait", "false", ""},
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
	if _, ok := cmd.Flags().Lookup("config").Annotations["cobra_annotation_bash_completion_one_required_flag"]; ok {
		t.Error("--config must remain optional for one-off report creation")
	}
}

func TestLoadInsightConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "insights.json")
	if err := os.WriteFile(path, []byte(`{
		"name": "Reliability review",
		"summary_prompt": "Diagnose {{run.inputs}} and {{run.error}}",
		"model": "anthropic",
		"cluster_model": "claude-thinking",
		"summary_model": "claude-fast",
		"start_time": "2026-09-01T00:00:00Z",
		"end_time": "2026-09-08T12:00:00Z",
		"filter": "eq(is_root, true)",
		"sample": 0.25,
		"partitions": {"environment": "metadata.env"},
		"attribute_schemas": {"failure": {"type": "boolean"}},
		"hierarchy": [4, 12]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := loadInsightConfigFile(path)
	if err != nil {
		t.Fatalf("loadInsightConfigFile: %v", err)
	}
	request, err := config.toSDKParams()
	if err != nil {
		t.Fatalf("toSDKParams: %v", err)
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
	if len(got["hierarchy"].([]any)) != 2 {
		t.Errorf("unexpected hierarchy: %#v", got["hierarchy"])
	}
}

func TestInsightConfigFileDefaultsModel(t *testing.T) {
	request, err := (insightConfigFile{
		Name:          "Report",
		SummaryPrompt: "Summarize {{run.inputs}}",
	}).toSDKParams()
	if err != nil {
		t.Fatalf("toSDKParams: %v", err)
	}
	data, _ := json.Marshal(request)
	if !strings.Contains(string(data), `"model":"openai"`) {
		t.Fatalf("expected default model in request: %s", data)
	}
}

func TestInsightConfigFileValidation(t *testing.T) {
	zero := int64(0)
	sample := float64(0)
	clusterModel := "heavy"
	start := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	invalidModel := createInsightModel("other")
	tests := []struct {
		name   string
		config insightConfigFile
		want   string
	}{
		{
			name:   "name required",
			config: insightConfigFile{SummaryPrompt: "{{run.inputs}}"},
			want:   "name must not be empty",
		},
		{
			name:   "prompt required",
			config: insightConfigFile{Name: "Report"},
			want:   "summary_prompt must not be empty",
		},
		{
			name:   "invalid model",
			config: insightConfigFile{Name: "Report", SummaryPrompt: "{{run.inputs}}", Model: &invalidModel},
			want:   "must be openai or anthropic",
		},
		{
			name:   "models paired",
			config: insightConfigFile{Name: "Report", SummaryPrompt: "{{run.inputs}}", ClusterModel: &clusterModel},
			want:   "must be specified together",
		},
		{
			name:   "end requires start",
			config: insightConfigFile{Name: "Report", SummaryPrompt: "{{run.inputs}}", EndTime: &end},
			want:   "end_time requires start_time",
		},
		{
			name:   "time order",
			config: insightConfigFile{Name: "Report", SummaryPrompt: "{{run.inputs}}", StartTime: &start, EndTime: &end},
			want:   "end_time must be after start_time",
		},
		{
			name:   "positive last hours",
			config: insightConfigFile{Name: "Report", SummaryPrompt: "{{run.inputs}}", LastNHours: &zero},
			want:   "last_n_hours must be greater than 0",
		},
		{
			name:   "positive sample",
			config: insightConfigFile{Name: "Report", SummaryPrompt: "{{run.inputs}}", Sample: &sample},
			want:   "sample must be greater than 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.config.toSDKParams()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestLoadInsightConfigFileRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "insights.json")
	if err := os.WriteFile(path, []byte(`{"name":"Report","summary_prompt":"{{run.inputs}}","unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadInsightConfigFile(path)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestLoadInsightConfigFileRejectsUserContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "insights.json")
	if err := os.WriteFile(path, []byte(`{
		"name":"Report",
		"summary_prompt":"{{run.inputs}}",
		"user_context":{"goal":"Find failures"}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadInsightConfigFile(path)
	if err == nil || !strings.Contains(err.Error(), `unknown field "user_context"`) {
		t.Fatalf("expected user_context to be rejected in manual mode, got %v", err)
	}
}

func TestInsightsCreateCmd_PostsRequestAndPrintsJSON(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	const insightID = "0199321d-e2b4-7000-8000-000000000003"
	configPath := filepath.Join(t.TempDir(), "insights.json")
	if err := os.WriteFile(configPath, []byte(`{
		"name":"Reliability review",
		"summary_prompt":"Diagnose {{run.error}}",
		"last_n_hours":24
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
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
		"--config", configPath,
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
	if !ok || config["summary_prompt"] != "Diagnose {{run.error}}" || config["last_n_hours"] != float64(24) {
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

func TestInsightsCreateCmd_ReportsSavedConfigWhenJobStartFails(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	configPath := filepath.Join(t.TempDir(), "insights.json")
	if err := os.WriteFile(configPath, []byte(`{"name":"Reliability review","summary_prompt":"Diagnose {{run.error}}"}`), 0o600); err != nil {
		t.Fatal(err)
	}
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
		"--config", configPath,
	})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "insight config "+configID+" was saved") {
		t.Fatalf("expected saved config ID in error, got %v", err)
	}
}

func TestInsightsCreateCmd_WaitsForSuccess(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	const insightID = "0199321d-e2b4-7000-8000-000000000003"
	configPath := filepath.Join(t.TempDir(), "insights.json")
	if err := os.WriteFile(configPath, []byte(`{"name":"Reliability review","summary_prompt":"Diagnose {{run.error}}"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	getCalls := 0
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/insights/configs"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": configID, "name": "Reliability review", "description": nil,
				"config": map[string]any{"name": "Reliability review", "summary_prompt": "Diagnose {{run.error}}", "model": "openai"},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/insights"):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": insightID, "name": "Reliability review", "status": "pending", "error": nil})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/insights/"+insightID):
			getCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": insightID, "name": "Reliability review", "status": "success", "error": nil})
		default:
			http.NotFound(w, r)
		}
	})
	defer setupTestEnv(t, server.URL)()
	flagOutputFormat = "json"

	cmd := newInsightsCreateCmd()
	cmd.SetArgs([]string{"--project-id", projectID, "--config", configPath, "--wait"})
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.Execute() })
	if runErr != nil {
		t.Fatalf("execute create --wait: %v", runErr)
	}
	if getCalls != 1 {
		t.Fatalf("expected one status request, got %d", getCalls)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse output %q: %v", out, err)
	}
	if result["status"] != "success" {
		t.Fatalf("expected final success status, got %#v", result)
	}
}

func TestWaitForInsightPollsAndReturnsJobError(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const insightID = "0199321d-e2b4-7000-8000-000000000003"
	getCalls := 0
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		getCalls++
		status := "running"
		errorMessage := any(nil)
		if getCalls == 2 {
			status = "error"
			errorMessage = "model secret unavailable"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": insightID, "name": "Report", "status": status, "error": errorMessage,
		})
	})
	defer setupTestEnv(t, server.URL)()
	c, err := getClient()
	if err != nil {
		t.Fatal(err)
	}
	_, err = waitForInsight(t.Context(), c.SDK, projectID, insightID, time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "model secret unavailable") {
		t.Fatalf("expected job error, got %v", err)
	}
	if getCalls != 2 {
		t.Fatalf("expected two status requests, got %d", getCalls)
	}
}

func createInsightModel(value string) langsmith.CreateRunClusteringJobRequestModel {
	return langsmith.CreateRunClusteringJobRequestModel(value)
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
		{"limit", "20", "n"},
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
