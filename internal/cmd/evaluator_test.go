package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

// testRule is a minimal JSON-serializable struct used by mock server handlers
// to produce responses that the SDK can decode into langsmith.Evaluator.
type testRule struct {
	ID             string         `json:"id"`
	DisplayName    string         `json:"display_name"`
	SamplingRate   float64        `json:"sampling_rate"`
	IsEnabled      bool           `json:"is_enabled"`
	DatasetID      string         `json:"dataset_id,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	CodeEvaluators []testCodeEval `json:"code_evaluators,omitempty"`
	Evaluators     []testLLMEval  `json:"evaluators,omitempty"`
}

type testCodeEval struct {
	Code     string `json:"code"`
	Language string `json:"language"`
}

type testLLMEval struct {
	Structured testLLMStructured `json:"structured"`
}

type testLLMStructured struct {
	HubRef          string            `json:"hub_ref,omitempty"`
	VariableMapping map[string]string `json:"variable_mapping,omitempty"`
}

// ==================== Pure function tests ====================

// ---------- extractPythonFunction ----------

func TestExtractPythonFunction_Simple(t *testing.T) {
	source := `import os

def my_func(run, example):
    score = 1
    return {"score": score}

def other_func():
    pass
`
	result := extractPythonFunction(source, "my_func")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !contains(result, "def my_func(run, example):") {
		t.Error("should contain function definition")
	}
	if !contains(result, `return {"score": score}`) {
		t.Error("should contain return statement")
	}
	if contains(result, "def other_func") {
		t.Error("should not contain other function")
	}
}

func TestExtractPythonFunction_LastFunction(t *testing.T) {
	source := `def first():
    pass

def last_func(x, y):
    return x + y
`
	result := extractPythonFunction(source, "last_func")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !contains(result, "def last_func(x, y):") {
		t.Error("should contain last_func definition")
	}
}

func TestExtractPythonFunction_NotFound(t *testing.T) {
	source := `def other(x):
    return x
`
	result := extractPythonFunction(source, "missing_func")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestExtractPythonFunction_WithIndentedBlocks(t *testing.T) {
	source := `def evaluate(run, example):
    if run.error:
        return {"score": 0}
    else:
        return {"score": 1}

class MyClass:
    pass
`
	result := extractPythonFunction(source, "evaluate")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if contains(result, "class MyClass") {
		t.Error("should not include class definition")
	}
}

func TestExtractPythonFunction_EmptySource(t *testing.T) {
	result := extractPythonFunction("", "anything")
	if result != "" {
		t.Errorf("expected empty for empty source, got %q", result)
	}
}

// ---------- extractJSFunction ----------

func TestExtractJSFunction_FunctionDeclaration(t *testing.T) {
	source := `function myEval(run, example) {
  if (run.error) {
    return { score: 0 };
  }
  return { score: 1 };
}

function other() {
  return 42;
}
`
	result := extractJSFunction(source, "myEval")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !contains(result, "function myEval(run, example)") {
		t.Error("should contain function declaration")
	}
	if contains(result, "function other") {
		t.Error("should not contain other function")
	}
}

func TestExtractJSFunction_ArrowFunction(t *testing.T) {
	source := `const myEval = (run, example) => {
  return { score: 1 };
}
`
	result := extractJSFunction(source, "myEval")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !contains(result, "const myEval") {
		t.Error("should contain const declaration")
	}
}

func TestExtractJSFunction_AsyncFunction(t *testing.T) {
	source := `export async function checkAccuracy(run, example) {
  const result = await check(run);
  return { score: result ? 1 : 0 };
}
`
	result := extractJSFunction(source, "checkAccuracy")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !contains(result, "async function checkAccuracy") {
		t.Error("should contain async function")
	}
}

func TestExtractJSFunction_ExportedConst(t *testing.T) {
	source := `export const myEval = (run, example) => {
  return { score: 1 };
}
`
	result := extractJSFunction(source, "myEval")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestExtractJSFunction_NotFound(t *testing.T) {
	source := `function other() { return 1; }`
	result := extractJSFunction(source, "missing")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestExtractJSFunction_NestedBraces(t *testing.T) {
	source := `function eval(run) {
  if (run.error) {
    return { key: { nested: true } };
  }
  return { score: 1 };
}
`
	result := extractJSFunction(source, "eval")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !contains(result, "return { score: 1 };") {
		t.Error("should contain the full function body")
	}
}

// ---------- detectLanguage ----------

func TestDetectLanguage_Python(t *testing.T) {
	lang, funcName := detectLanguage("eval.py")
	if lang != "python" {
		t.Errorf("expected python, got %q", lang)
	}
	if funcName != "perform_eval" {
		t.Errorf("expected perform_eval, got %q", funcName)
	}
}

func TestDetectLanguage_JavaScript(t *testing.T) {
	for _, ext := range []string{".js", ".ts", ".tsx", ".mjs"} {
		lang, funcName := detectLanguage("eval" + ext)
		if lang != "javascript" {
			t.Errorf("detectLanguage(eval%s): expected javascript, got %q", ext, lang)
		}
		if funcName != "performEval" {
			t.Errorf("detectLanguage(eval%s): expected performEval, got %q", ext, funcName)
		}
	}
}

func TestDetectLanguage_Unknown(t *testing.T) {
	lang, funcName := detectLanguage("eval.rb")
	if lang != "" || funcName != "" {
		t.Errorf("expected empty for .rb, got (%q, %q)", lang, funcName)
	}
}

func TestDetectLanguage_CaseInsensitive(t *testing.T) {
	lang, _ := detectLanguage("eval.PY")
	if lang != "python" {
		t.Errorf("expected python for .PY, got %q", lang)
	}
}

// ---------- renameJSFunction ----------

func TestRenameJSFunction_FunctionDecl(t *testing.T) {
	source := `function myEval(run, example) {
  return { score: 1 };
}
`
	result := renameJSFunction(source, "myEval")
	if !contains(result, "function performEval(") {
		t.Errorf("expected function renamed to performEval, got:\n%s", result)
	}
	if contains(result, "myEval") {
		t.Error("original name should be replaced")
	}
}

func TestRenameJSFunction_AsyncFunctionDecl(t *testing.T) {
	source := `async function myEval(run) {
  return { score: 1 };
}
`
	result := renameJSFunction(source, "myEval")
	if !contains(result, "async function performEval(") {
		t.Errorf("expected async function performEval, got:\n%s", result)
	}
}

func TestRenameJSFunction_ExportFunction(t *testing.T) {
	source := `export function myEval(run) {
  return { score: 1 };
}
`
	result := renameJSFunction(source, "myEval")
	if !contains(result, "function performEval(") {
		t.Errorf("expected export stripped, got:\n%s", result)
	}
	if contains(result, "export") {
		t.Error("export keyword should be removed")
	}
}

func TestRenameJSFunction_ArrowFunction(t *testing.T) {
	source := `const myEval = (run) => {
  return { score: 1 };
}
`
	result := renameJSFunction(source, "myEval")
	if !contains(result, "function performEval(run) {") {
		t.Errorf("expected arrow converted to function decl, got:\n%s", result)
	}
}

func TestRenameJSFunction_AsyncArrowFunction(t *testing.T) {
	source := `export const myEval = async (run, example) => {
  return { score: 1 };
}
`
	result := renameJSFunction(source, "myEval")
	if !contains(result, "async function performEval(run, example) {") {
		t.Errorf("expected async arrow converted, got:\n%s", result)
	}
}

// ---------- findEvaluator ----------

func TestFindEvaluator_MatchByDataset(t *testing.T) {
	rules := []langsmith.Evaluator{
		{ID: "1", DisplayName: "accuracy", DatasetID: "ds-1"},
		{ID: "2", DisplayName: "accuracy", DatasetID: "ds-2"},
	}
	result := findEvaluator(rules, "accuracy", "ds-1", "")
	if result == nil {
		t.Fatal("expected match")
	}
	if result.ID != "1" {
		t.Errorf("expected ID=1, got %q", result.ID)
	}
}

func TestFindEvaluator_MatchByProject(t *testing.T) {
	rules := []langsmith.Evaluator{
		{ID: "1", DisplayName: "accuracy", SessionID: "proj-1"},
		{ID: "2", DisplayName: "accuracy", SessionID: "proj-2"},
	}
	result := findEvaluator(rules, "accuracy", "", "proj-2")
	if result == nil {
		t.Fatal("expected match")
	}
	if result.ID != "2" {
		t.Errorf("expected ID=2, got %q", result.ID)
	}
}

func TestFindEvaluator_NoMatch(t *testing.T) {
	rules := []langsmith.Evaluator{
		{ID: "1", DisplayName: "accuracy", DatasetID: "ds-1"},
	}
	result := findEvaluator(rules, "different-name", "ds-1", "")
	if result != nil {
		t.Error("expected nil for non-matching name")
	}
}

func TestFindEvaluator_EmptyRules(t *testing.T) {
	result := findEvaluator(nil, "accuracy", "ds-1", "")
	if result != nil {
		t.Error("expected nil for empty rules")
	}
}

func TestFindEvaluator_NameMatchButNoTarget(t *testing.T) {
	rules := []langsmith.Evaluator{
		{ID: "1", DisplayName: "accuracy", DatasetID: "ds-1"},
	}
	result := findEvaluator(rules, "accuracy", "ds-other", "")
	if result != nil {
		t.Error("expected nil when target doesn't match")
	}
}

func TestValidateEvaluatorTargetFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dataset string
		project string
		wantErr string
	}{
		{name: "requires one target", wantErr: "must specify"},
		{name: "rejects both targets", dataset: "ds", project: "proj", wantErr: "cannot specify both"},
		{name: "dataset only", dataset: "ds"},
		{name: "project only", project: "proj"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateEvaluatorTargetFlags(tt.dataset, tt.project)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestLoadPromptMessagesFromJSON(t *testing.T) {
	t.Parallel()

	t.Run("pair format", func(t *testing.T) {
		t.Parallel()
		msgs, err := loadPromptMessagesFromJSON("prompt.json", []byte(
			`[["system","You are a judge."],["user","Q: {{input}}"]]`,
		))
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 2 || msgs[0][0] != "system" {
			t.Fatalf("unexpected messages: %#v", msgs)
		}
	})

	t.Run("role content objects", func(t *testing.T) {
		t.Parallel()
		msgs, err := loadPromptMessagesFromJSON("prompt.json", []byte(
			`[{"role":"system","content":"You are a judge."},{"role":"user","content":"Q: {{input}}"}]`,
		))
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 2 || msgs[1][1] != "Q: {{input}}" {
			t.Fatalf("unexpected messages: %#v", msgs)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		t.Parallel()
		_, err := loadPromptMessagesFromJSON("prompt.json", []byte(`{"messages":[]}`))
		if err == nil || !strings.Contains(err.Error(), "expected [[role,content]") {
			t.Fatalf("expected format hint, got %v", err)
		}
	})

	t.Run("rejects single-element pair", func(t *testing.T) {
		t.Parallel()
		_, err := loadPromptMessagesFromJSON("prompt.json", []byte(`[["system"]]`))
		if err == nil || !strings.Contains(err.Error(), "must be [role, content]") {
			t.Fatalf("expected length error, got %v", err)
		}
	})
}

func TestBuildLLMEvaluatorPayload_requiresModelConfig(t *testing.T) {
	t.Parallel()

	_, err := buildLLMEvaluatorPayload(
		"relevance", llmEvaluatorTarget{projectID: "proj-1"},
		1.0, "", "", "prompt.json", "schema.json", "", nil,
	)
	if err == nil || !strings.Contains(err.Error(), "--model-config is required") {
		t.Fatalf("expected model-config error, got %v", err)
	}
}

// ==================== Command structure tests ====================

func TestEvaluatorCmd_Subcommands(t *testing.T) {
	cmd := newEvaluatorCmd()
	expected := map[string]bool{"get": false, "list": false, "upload": false, "create-llm": false, "delete": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("evaluator missing subcommand %q", name)
		}
	}
	for _, sub := range cmd.Commands() {
		if sub.Name() == "create" {
			t.Error("evaluator should not have 'create' subcommand")
		}
	}
}

func TestEvaluatorCmd_UseField(t *testing.T) {
	cmd := newEvaluatorCmd()
	if cmd.Use != "evaluator" {
		t.Errorf("expected Use=evaluator, got %q", cmd.Use)
	}
}

// ---------- Subcommand Use fields ----------

func TestEvaluatorListCmd_UseField(t *testing.T) {
	cmd := newEvaluatorListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

func TestEvaluatorUploadCmd_UseField(t *testing.T) {
	cmd := newEvaluatorUploadCmd()
	if cmd.Use != "upload EVALUATOR_FILE" {
		t.Errorf("expected Use='upload EVALUATOR_FILE', got %q", cmd.Use)
	}
}

func TestEvaluatorDeleteCmd_UseField(t *testing.T) {
	cmd := newEvaluatorDeleteCmd()
	if cmd.Use != "delete NAME" {
		t.Errorf("expected Use='delete NAME', got %q", cmd.Use)
	}
}

// ---------- evaluator list flags ----------

func TestEvaluatorListCmd_Flags(t *testing.T) {
	cmd := newEvaluatorListCmd()
	f := cmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("--output flag not found")
	}
	if f.Shorthand != "o" {
		t.Errorf("expected shorthand 'o', got %q", f.Shorthand)
	}
	if f.DefValue != "" {
		t.Errorf("expected default empty, got %q", f.DefValue)
	}
}

// ---------- evaluator upload flags ----------

func TestEvaluatorUploadCmd_Flags(t *testing.T) {
	cmd := newEvaluatorUploadCmd()
	flags := map[string]string{
		"name":          "",
		"function":      "",
		"dataset":       "",
		"project":       "",
		"sampling-rate": "1",
		"trace-filter":  "",
		"replace":       "false",
		"yes":           "false",
	}
	for name, defVal := range flags {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag --%s not found", name)
			continue
		}
		if f.DefValue != defVal {
			t.Errorf("flag --%s: expected default %q, got %q", name, defVal, f.DefValue)
		}
	}
}

func TestEvaluatorUploadCmd_RequiredFlags(t *testing.T) {
	cmd := newEvaluatorUploadCmd()
	for _, name := range []string{"name", "function"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("flag --%s not found", name)
		}
		ann := f.Annotations
		if ann == nil {
			t.Errorf("flag --%s has no annotations (not marked required)", name)
			continue
		}
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
			t.Errorf("flag --%s not marked as required", name)
		}
	}
}

func TestEvaluatorUploadCmd_ExactArgs(t *testing.T) {
	cmd := newEvaluatorUploadCmd()
	if cmd.Args == nil {
		t.Fatal("expected Args validator")
	}
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"eval.py"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestEvaluatorUploadCmd_InvalidTargetReturnsReportedError(t *testing.T) {
	cmd := newEvaluatorUploadCmd()
	_ = cmd.Flags().Set("dataset", "dataset")
	_ = cmd.Flags().Set("project", "project")

	var runErr error
	out := captureStdout(t, func() {
		runErr = runTestCommand(t, cmd, []string{"evaluator.py"})
	})
	if !IsReportedError(runErr) {
		t.Fatalf("expected reported error, got %v", runErr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse output JSON: %v\noutput: %s", err, out)
	}
	if !contains(result["error"].(string), "cannot specify both") {
		t.Errorf("unexpected error: %v", result["error"])
	}
}

// ---------- evaluator delete flags ----------

func TestEvaluatorDeleteCmd_Flags(t *testing.T) {
	cmd := newEvaluatorDeleteCmd()
	f := cmd.Flags().Lookup("yes")
	if f == nil {
		t.Fatal("--yes flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %q", f.DefValue)
	}
}

func TestEvaluatorDeleteCmd_ExactArgs(t *testing.T) {
	cmd := newEvaluatorDeleteCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-eval"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

// ==================== Execution tests ====================

func TestEvaluatorListCmd_Execute(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]testRule{
				{ID: "eval-1", DisplayName: "accuracy", SamplingRate: 1.0, IsEnabled: true, DatasetID: "ds-1"},
				{ID: "eval-2", DisplayName: "toxicity", SamplingRate: 0.5, IsEnabled: false, SessionID: "proj-1"},
			})
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newEvaluatorListCmd()
		_ = runTestCommand(t, cmd, nil)
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 evaluators, got %d", len(result))
	}
	if result[0]["name"] != "accuracy" {
		t.Errorf("expected name=accuracy, got %v", result[0]["name"])
	}
	if result[0]["id"] != "eval-1" {
		t.Errorf("expected id=eval-1, got %v", result[0]["id"])
	}
	if result[1]["name"] != "toxicity" {
		t.Errorf("expected name=toxicity, got %v", result[1]["name"])
	}
	if result[0]["sampling_rate"] != 1.0 {
		t.Errorf("expected sampling_rate=1.0, got %v", result[0]["sampling_rate"])
	}
	if result[1]["is_enabled"] != false {
		t.Errorf("expected is_enabled=false, got %v", result[1]["is_enabled"])
	}
}

func TestEvaluatorListCmd_Execute_PrettyFormat(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]testRule{
				{ID: "eval-1", DisplayName: "accuracy", SamplingRate: 1.0, IsEnabled: true, DatasetID: "ds-1"},
			})
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "pretty"

	out := captureStdout(t, func() {
		cmd := newEvaluatorListCmd()
		_ = runTestCommand(t, cmd, nil)
	})

	if len(out) > 0 && out[0] == '[' {
		t.Error("pretty format should not produce JSON array")
	}
	if !contains(out, "accuracy") {
		t.Error("pretty output should contain evaluator name")
	}
}

func TestEvaluatorListCmd_Execute_EmptyList(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newEvaluatorListCmd()
		_ = runTestCommand(t, cmd, nil)
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
}

func TestEvaluatorListCmd_VerifiesAPIKeyHeader(t *testing.T) {
	var receivedKey string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	captureStdout(t, func() {
		cmd := newEvaluatorListCmd()
		_ = runTestCommand(t, cmd, nil)
	})

	if receivedKey != "test-api-key" {
		t.Errorf("expected x-api-key=test-api-key, got %q", receivedKey)
	}
}

func TestEvaluatorUploadReplacePatchesExistingCodeEvaluator(t *testing.T) {
	evaluatorFile := t.TempDir() + "/eval.py"
	if err := os.WriteFile(
		evaluatorFile,
		[]byte("def check_accuracy(run, example):\n    return {\"score\": 1}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	var sawDelete bool
	var patchBody map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sessions" && r.Method == "GET":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "project-1", "name": "my-project"},
			})
		case r.URL.Path == "/api/v1/runs/rules" && r.Method == "GET":
			_ = json.NewEncoder(w).Encode([]testRule{
				{
					ID:           "existing-rule",
					DisplayName:  "accuracy",
					SamplingRate: 0.25,
					IsEnabled:    true,
					SessionID:    "project-1",
					CodeEvaluators: []testCodeEval{
						{Code: "def perform_eval(run, example):\n    return {\"score\": 0}", Language: "python"},
					},
				},
			})
		case r.URL.Path == "/api/v1/runs/rules/existing-rule" && r.Method == "PATCH":
			if err := json.NewDecoder(r.Body).Decode(&patchBody); err != nil {
				t.Fatalf("decoding patch body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "existing-rule",
				"display_name": "accuracy",
				"session_id":   "project-1",
			})
		case r.URL.Path == "/api/v1/runs/rules/existing-rule" && r.Method == "DELETE":
			sawDelete = true
			http.Error(w, "delete should not be called", http.StatusInternalServerError)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newEvaluatorUploadCmd()
		_ = cmd.Flags().Set("name", "accuracy")
		_ = cmd.Flags().Set("function", "check_accuracy")
		_ = cmd.Flags().Set("project", "my-project")
		_ = cmd.Flags().Set("sampling-rate", "0.5")
		_ = cmd.Flags().Set("replace", "true")
		_ = cmd.Flags().Set("yes", "true")
		_ = runTestCommand(t, cmd, []string{evaluatorFile})
	})

	if sawDelete {
		t.Fatal("upload --replace should patch the existing evaluator, not delete it")
	}
	if patchBody == nil {
		t.Fatal("expected PATCH body")
	}
	if patchBody["display_name"] != "accuracy" {
		t.Errorf("expected display_name=accuracy, got %v", patchBody["display_name"])
	}
	if patchBody["session_id"] != "project-1" {
		t.Errorf("expected session_id=project-1, got %v", patchBody["session_id"])
	}
	if patchBody["sampling_rate"] != 0.5 {
		t.Errorf("expected sampling_rate=0.5, got %v", patchBody["sampling_rate"])
	}
	evaluators, ok := patchBody["code_evaluators"].([]any)
	if !ok || len(evaluators) != 1 {
		t.Fatalf("expected one code evaluator, got %#v", patchBody["code_evaluators"])
	}
	codeEvaluator, ok := evaluators[0].(map[string]any)
	if !ok {
		t.Fatalf("expected code evaluator object, got %#v", evaluators[0])
	}
	if codeEvaluator["language"] != "python" {
		t.Errorf("expected language=python, got %v", codeEvaluator["language"])
	}
	code, _ := codeEvaluator["code"].(string)
	if !strings.Contains(code, "def perform_eval(") {
		t.Errorf("expected uploaded code to be renamed to perform_eval, got:\n%s", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if result["id"] != "existing-rule" {
		t.Errorf("expected output id=existing-rule, got %v", result["id"])
	}
}

func TestEvaluatorCreateLLMReplacePatchesExistingEvaluator(t *testing.T) {
	modelConfigPath := t.TempDir() + "/model.json"
	if err := os.WriteFile(
		modelConfigPath,
		[]byte(`{"type":"chat","config":{"model":"test-model"}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	var sawDelete bool
	var patchBody map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sessions" && r.Method == "GET":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "project-1", "name": "my-project"},
			})
		case r.URL.Path == "/api/v1/runs/rules" && r.Method == "GET":
			_ = json.NewEncoder(w).Encode([]testRule{
				{
					ID:           "existing-rule",
					DisplayName:  "relevance",
					SamplingRate: 0.25,
					IsEnabled:    true,
					SessionID:    "project-1",
					Evaluators: []testLLMEval{
						{Structured: testLLMStructured{HubRef: "my-org/old:latest"}},
					},
				},
			})
		case r.URL.Path == "/api/v1/runs/rules/existing-rule" && r.Method == "PATCH":
			if err := json.NewDecoder(r.Body).Decode(&patchBody); err != nil {
				t.Fatalf("decoding patch body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "existing-rule",
				"display_name": "relevance",
				"session_id":   "project-1",
			})
		case r.URL.Path == "/api/v1/runs/rules/existing-rule" && r.Method == "DELETE":
			sawDelete = true
			http.Error(w, "delete should not be called", http.StatusInternalServerError)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newEvaluatorCreateLLMCmd()
		_ = cmd.Flags().Set("name", "relevance")
		_ = cmd.Flags().Set("project", "my-project")
		_ = cmd.Flags().Set("hub-ref", "my-org/relevance:latest")
		_ = cmd.Flags().Set("model-config", modelConfigPath)
		_ = cmd.Flags().Set("sampling-rate", "0.5")
		_ = cmd.Flags().Set("replace", "true")
		_ = cmd.Flags().Set("yes", "true")
		_ = runTestCommand(t, cmd, nil)
	})

	if sawDelete {
		t.Fatal("create-llm --replace should patch the existing evaluator, not delete it")
	}
	if patchBody == nil {
		t.Fatal("expected PATCH body")
	}
	if patchBody["display_name"] != "relevance" || patchBody["evaluators"] == nil {
		t.Fatalf("expected LLM evaluator replacement payload, got %#v", patchBody)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if result["id"] != "existing-rule" {
		t.Errorf("expected output id=existing-rule, got %v", result["id"])
	}
}

// ==================== evaluator get ====================

func TestEvaluatorGetCmd_UseField(t *testing.T) {
	cmd := newEvaluatorGetCmd()
	if cmd.Use != "get [NAME]" {
		t.Errorf("expected Use='get [NAME]', got %q", cmd.Use)
	}
}

func TestEvaluatorGetCmd_Args(t *testing.T) {
	cmd := newEvaluatorGetCmd()
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("expected no error for 0 args, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"accuracy"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestEvaluatorGetCmd_SessionIDFlag(t *testing.T) {
	cmd := newEvaluatorGetCmd()
	f := cmd.Flags().Lookup("session-id")
	if f == nil {
		t.Fatal("--session-id flag not found")
	}
	if f.DefValue != "" {
		t.Errorf("expected default empty, got %q", f.DefValue)
	}
}

func TestEvaluatorGetCmd_Execute_CodeEvaluator(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]testRule{
				{
					ID:           "eval-1",
					DisplayName:  "accuracy",
					SamplingRate: 1.0,
					IsEnabled:    true,
					DatasetID:    "ds-1",
					CodeEvaluators: []testCodeEval{
						{Code: "def perform_eval(run, example):\n  return {}", Language: "python"},
					},
				},
			})
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newEvaluatorGetCmd()
		_ = runTestCommand(t, cmd, []string{"accuracy"})
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if result["id"] != "eval-1" {
		t.Errorf("expected id=eval-1, got %v", result["id"])
	}
	if result["type"] != "code" {
		t.Errorf("expected type=code, got %v", result["type"])
	}
	if result["language"] != "python" {
		t.Errorf("expected language=python, got %v", result["language"])
	}
	if result["code"] == nil || result["code"] == "" {
		t.Error("expected code to be populated")
	}
}

func TestEvaluatorGetCmd_Execute_LLMEvaluator(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]testRule{
				{
					ID:           "eval-2",
					DisplayName:  "relevance",
					SamplingRate: 0.5,
					IsEnabled:    true,
					SessionID:    "proj-1",
					Evaluators: []testLLMEval{
						{Structured: testLLMStructured{
							HubRef:          "myorg/relevance:latest",
							VariableMapping: map[string]string{"input": "question"},
						}},
					},
				},
			})
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newEvaluatorGetCmd()
		_ = runTestCommand(t, cmd, []string{"relevance"})
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if result["type"] != "llm" {
		t.Errorf("expected type=llm, got %v", result["type"])
	}
	if result["hub_ref"] != "myorg/relevance:latest" {
		t.Errorf("expected hub_ref set, got %v", result["hub_ref"])
	}
	varMap, _ := result["variable_mapping"].(map[string]any)
	if varMap["input"] != "question" {
		t.Errorf("expected variable_mapping.input=question, got %v", varMap["input"])
	}
}

func TestEvaluatorGetCmd_Execute_NotFound(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	var runErr error
	out := captureStdout(t, func() {
		cmd := newEvaluatorGetCmd()
		runErr = runTestCommand(t, cmd, []string{"nonexistent"})
	})
	if !IsReportedError(runErr) {
		t.Fatalf("expected reported error, got %v", runErr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if result["error"] == nil {
		t.Error("expected error for not found evaluator")
	}
}

func TestEvaluatorGetCmd_Execute_FilterBySessionID(t *testing.T) {
	allRules := []testRule{
		{ID: "eval-1", DisplayName: "accuracy", SessionID: "session-abc", SamplingRate: 1.0, IsEnabled: true},
		{ID: "eval-2", DisplayName: "toxicity", SessionID: "session-xyz", SamplingRate: 0.5, IsEnabled: true},
		{ID: "eval-3", DisplayName: "relevance", SessionID: "session-abc", SamplingRate: 1.0, IsEnabled: true},
	}

	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			// Simulate server-side filtering by session_id query param.
			sid := r.URL.Query().Get("session_id")
			var filtered []testRule
			for _, rule := range allRules {
				if sid == "" || rule.SessionID == sid {
					filtered = append(filtered, rule)
				}
			}
			_ = json.NewEncoder(w).Encode(filtered)
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newEvaluatorGetCmd()
		_ = cmd.Flags().Set("session-id", "session-abc")
		_ = runTestCommand(t, cmd, nil)
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 evaluators for session-abc, got %d", len(result))
	}
	for _, r := range result {
		if r["session_id"] != "session-abc" {
			t.Errorf("expected session_id=session-abc, got %v", r["session_id"])
		}
	}
}

func TestEvaluatorGetCmd_Execute_FilterByNameAndSessionID(t *testing.T) {
	allRules := []testRule{
		{ID: "eval-1", DisplayName: "accuracy", SessionID: "session-abc", SamplingRate: 1.0, IsEnabled: true},
		{ID: "eval-2", DisplayName: "accuracy", SessionID: "session-xyz", SamplingRate: 0.5, IsEnabled: true},
	}

	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			sid := r.URL.Query().Get("session_id")
			var filtered []testRule
			for _, rule := range allRules {
				if sid == "" || rule.SessionID == sid {
					filtered = append(filtered, rule)
				}
			}
			_ = json.NewEncoder(w).Encode(filtered)
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newEvaluatorGetCmd()
		_ = cmd.Flags().Set("session-id", "session-abc")
		_ = runTestCommand(t, cmd, []string{"accuracy"})
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("expected single object: %v\noutput: %s", err, out)
	}
	if result["id"] != "eval-1" {
		t.Errorf("expected eval-1, got %v", result["id"])
	}
}

func TestEvaluatorGetCmd_Execute_MultipleMatches(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]testRule{
				{ID: "eval-1", DisplayName: "accuracy", DatasetID: "ds-1"},
				{ID: "eval-2", DisplayName: "accuracy", SessionID: "proj-1"},
			})
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newEvaluatorGetCmd()
		_ = runTestCommand(t, cmd, []string{"accuracy"})
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("expected JSON array for multiple matches: %v\noutput: %s", err, out)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
}

// ==================== helper ====================

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
