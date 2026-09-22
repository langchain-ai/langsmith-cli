package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInsightsConfigCmd_Subcommands(t *testing.T) {
	cmd := newInsightsConfigCmd()
	expected := map[string]bool{"list": false, "update": false, "delete": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("insights config missing subcommand %q", name)
		}
	}
}

func TestInsightsRunCmd_PostsConfigIDOnly(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	const jobID = "0199321d-e2b4-7000-8000-000000000003"
	var requestBody map[string]any
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/sessions/"+projectID+"/insights" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode run request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": jobID, "name": "Reliability review", "status": "pending", "error": nil,
		})
	})
	defer setupTestEnv(t, server.URL)()
	flagOutputFormat = "json"

	cmd := newInsightsRunCmd()
	cmd.SetArgs([]string{"--project-id", projectID, configID})
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.Execute() })
	if runErr != nil {
		t.Fatalf("execute run: %v", runErr)
	}
	if len(requestBody) != 1 || requestBody["config_id"] != configID {
		t.Fatalf("run request should contain only config_id, got %#v", requestBody)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse output %q: %v", out, err)
	}
	if result["id"] != jobID || result["config_id"] != configID || result["status"] != "pending" {
		t.Errorf("unexpected output: %#v", result)
	}
}

func TestInsightsConfigListCmd_ListsCompleteConfigs(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/sessions/"+projectID+"/insights/configs" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("include_prebuilts") != "true" {
			t.Errorf("expected include_prebuilts=true, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configs": []any{insightConfigListFixture(configID, "0 9 * * 1")},
		})
	})
	defer setupTestEnv(t, server.URL)()
	flagOutputFormat = "json"

	cmd := newInsightsConfigListCmd()
	cmd.SetArgs([]string{"--project-id", projectID, "--include-prebuilt"})
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.Execute() })
	if runErr != nil {
		t.Fatalf("execute config list: %v", runErr)
	}
	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse output %q: %v", out, err)
	}
	if len(result) != 1 || result[0]["id"] != configID || result[0]["schedule_cron"] != "0 9 * * 1" {
		t.Fatalf("unexpected list output: %#v", result)
	}
	config, ok := result[0]["config"].(map[string]any)
	if !ok || config["summary_prompt"] != "Diagnose {{run.error}}" {
		t.Errorf("list output omitted config definition: %#v", result[0])
	}
}

func TestInsightsConfigUpdateCmd_UpdatesDefinitionAndClearsSchedule(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	configPath := writeInsightConfigFile(t, `{
		"name":"Updated reliability review",
		"description":"",
		"summary_prompt":"Find failures in {{run.error}}",
		"last_n_hours":48,
		"schedule_cron":null
	}`)
	var requestBody map[string]any
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/sessions/"+projectID+"/insights/configs/"+configID {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode update request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		response := insightConfigListFixture(configID, nil)
		delete(response, "prebuilt")
		response["name"] = "Updated reliability review"
		response["description"] = ""
		response["config"].(map[string]any)["name"] = "Updated reliability review"
		response["config"].(map[string]any)["summary_prompt"] = "Find failures in {{run.error}}"
		response["config"].(map[string]any)["last_n_hours"] = 48
		_ = json.NewEncoder(w).Encode(response)
	})
	defer setupTestEnv(t, server.URL)()
	flagOutputFormat = "json"

	cmd := newInsightsConfigUpdateCmd()
	cmd.SetArgs([]string{configID, "--project-id", projectID, "--config", configPath})
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.Execute() })
	if runErr != nil {
		t.Fatalf("execute config update: %v", runErr)
	}
	if value, ok := requestBody["schedule_cron"]; !ok || value != nil {
		t.Errorf("expected explicit null schedule_cron, got %#v", requestBody)
	}
	if value, ok := requestBody["description"]; !ok || value != "" {
		t.Errorf("expected empty description, got %#v", requestBody)
	}
	config, ok := requestBody["config"].(map[string]any)
	if !ok || config["summary_prompt"] != "Find failures in {{run.error}}" || config["last_n_hours"] != float64(48) {
		t.Errorf("unexpected config update: %#v", requestBody)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse output %q: %v", out, err)
	}
	if result["id"] != configID || result["schedule_cron"] != nil {
		t.Errorf("unexpected update output: %#v", result)
	}
}

func TestInsightsConfigUpdateCmd_OmittedMetadataIsNotPatched(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	configPath := writeInsightConfigFile(t, `{
		"name":"Reliability review",
		"summary_prompt":"Find failures in {{run.error}}"
	}`)
	var requestBody map[string]any
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode update request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		response := insightConfigListFixture(configID, "0 9 * * 1")
		delete(response, "prebuilt")
		_ = json.NewEncoder(w).Encode(response)
	})
	defer setupTestEnv(t, server.URL)()
	flagOutputFormat = "json"

	cmd := newInsightsConfigUpdateCmd()
	cmd.SetArgs([]string{configID, "--project-id", projectID, "--config", configPath})
	var runErr error
	_ = captureStdout(t, func() { runErr = cmd.Execute() })
	if runErr != nil {
		t.Fatalf("execute config update: %v", runErr)
	}
	if _, ok := requestBody["schedule_cron"]; ok {
		t.Errorf("omitted schedule_cron should not be patched: %#v", requestBody)
	}
	if _, ok := requestBody["description"]; ok {
		t.Errorf("omitted description should not be patched: %#v", requestBody)
	}
}

func TestInsightsConfigDeleteCmd_DeletesByID(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	var called bool
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/sessions/"+projectID+"/insights/configs/"+configID {
			http.NotFound(w, r)
			return
		}
		called = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": configID, "message": "deleted"})
	})
	defer setupTestEnv(t, server.URL)()
	flagOutputFormat = "json"

	cmd := newInsightsConfigDeleteCmd()
	cmd.SetArgs([]string{configID, "--project-id", projectID, "--yes"})
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.Execute() })
	if runErr != nil {
		t.Fatalf("execute config delete: %v", runErr)
	}
	if !called || !strings.Contains(out, configID) || !strings.Contains(out, "deleted") {
		t.Errorf("unexpected delete result: called=%t output=%q", called, out)
	}
}

func TestInsightsConfigDeleteCmd_WarnsAboutAssociatedJobsAndReports(t *testing.T) {
	const projectID = "0199321d-e2b4-7000-8000-000000000001"
	const configID = "0199321d-e2b4-7000-8000-000000000002"
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/sessions/"+projectID+"/insights/configs/"+configID {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": configID, "message": "deleted"})
	})
	defer setupTestEnv(t, server.URL)()
	flagOutputFormat = "json"

	cmd := newInsightsConfigDeleteCmd()
	cmd.SetArgs([]string{configID, "--project-id", projectID})
	cmd.SetIn(strings.NewReader("yes\n"))
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	var runErr error
	_ = captureStdout(t, func() { runErr = cmd.Execute() })
	if runErr != nil {
		t.Fatalf("execute config delete: %v", runErr)
	}
	if warning := stderr.String(); !strings.Contains(warning, "all associated jobs and reports") {
		t.Fatalf("delete warning does not describe cascading deletion: %q", warning)
	}
}

func writeInsightConfigFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "insights.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func insightConfigListFixture(configID string, schedule any) map[string]any {
	return map[string]any{
		"id":            configID,
		"name":          "Reliability review",
		"description":   "Find recurring failures",
		"schedule_cron": schedule,
		"prebuilt":      false,
		"config": map[string]any{
			"name":              "Reliability review",
			"last_n_hours":      24,
			"start_time":        nil,
			"end_time":          nil,
			"hierarchy":         nil,
			"partitions":        nil,
			"sample":            nil,
			"summary_prompt":    "Diagnose {{run.error}}",
			"filter":            "eq(is_root, true)",
			"attribute_schemas": nil,
			"user_context":      nil,
			"model":             "openai",
			"cluster_model":     nil,
			"summary_model":     nil,
		},
	}
}
