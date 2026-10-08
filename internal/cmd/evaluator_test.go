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

// ---------- findRuleForReplace ----------

func TestFindRuleForReplace(t *testing.T) {
	code := []langsmith.CodeEvaluatorTopLevel{{Code: "x"}}
	rules := []langsmith.Evaluator{
		{ID: "renamed", DisplayName: "test", EvaluatorID: "ev-1", EvaluatorName: "ready_for_task_grade", SessionID: "proj-1"},
		{ID: "legacy", DisplayName: "accuracy", CodeEvaluators: code, DatasetID: "ds-1"},
		{ID: "other-target", DisplayName: "accuracy", CodeEvaluators: code, DatasetID: "ds-2"},
		{ID: "webhook", DisplayName: "notify", SessionID: "proj-1"},
		{ID: "dup-a", DisplayName: "dup", EvaluatorID: "ev-2", EvaluatorName: "dup", SessionID: "proj-2"},
		{ID: "dup-b", DisplayName: "dup-rule", EvaluatorID: "ev-3", EvaluatorName: "dup", SessionID: "proj-2"},
	}
	tests := []struct {
		name, match, datasetID, projectID string
		wantID                            string
		wantErr                           bool
	}{
		{name: "evaluator name after a UI rename", match: "ready_for_task_grade", projectID: "proj-1", wantID: "renamed"},
		{name: "rule name", match: "test", projectID: "proj-1", wantID: "renamed"},
		{name: "legacy inline rule on its dataset", match: "accuracy", datasetID: "ds-1", wantID: "legacy"},
		{name: "name on another target", match: "accuracy", datasetID: "ds-other"},
		{name: "non-evaluator rule is never replaced", match: "notify", projectID: "proj-1"},
		{name: "no match", match: "missing", projectID: "proj-1"},
		{name: "ambiguous match", match: "dup", projectID: "proj-2", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findRuleForReplace(rules, tt.match, tt.datasetID, tt.projectID)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "dup-a") || !strings.Contains(err.Error(), "dup-b") {
					t.Fatalf("expected ambiguity error listing both rules, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotID := ""
			if got != nil {
				gotID = got.ID
			}
			if gotID != tt.wantID {
				t.Errorf("expected %q, got %q", tt.wantID, gotID)
			}
		})
	}
}

func TestValidateEvaluatorTargetFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dataset   string
		project   string
		projectID string
		wantErr   string
	}{
		{name: "requires one target", wantErr: "must specify"},
		{name: "rejects dataset and project", dataset: "ds", project: "proj", wantErr: "only one of"},
		{name: "rejects dataset and project-id", dataset: "ds", projectID: "id", wantErr: "only one of"},
		{name: "rejects project and project-id", project: "proj", projectID: "id", wantErr: "only one of"},
		{name: "rejects all three", dataset: "ds", project: "proj", projectID: "id", wantErr: "only one of"},
		{name: "dataset only", dataset: "ds"},
		{name: "project only", project: "proj"},
		{name: "project-id only", projectID: "id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateEvaluatorTargetFlags(tt.dataset, tt.project, tt.projectID)
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
		"relevance", evaluatorTarget{projectID: "proj-1"},
		1.0, "", "", "prompt.json", "schema.json", "", nil,
	)
	if err == nil || !strings.Contains(err.Error(), "--model-config is required") {
		t.Fatalf("expected model-config error, got %v", err)
	}
}

// ==================== Command structure tests ====================

func TestEvaluatorCmd_Subcommands(t *testing.T) {
	cmd := newEvaluatorCmd()
	expected := map[string]bool{"get": false, "list": false, "upload": false, "create-llm": false, "delete": false, "rule": false}
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
	if cmd.Use != "delete EVALUATOR_ID" {
		t.Errorf("expected Use='delete EVALUATOR_ID', got %q", cmd.Use)
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

func TestEvaluatorUploadCmd_InvalidTargetReturnsError(t *testing.T) {
	cmd := newEvaluatorUploadCmd()
	_ = cmd.Flags().Set("dataset", "dataset")
	_ = cmd.Flags().Set("project", "project")

	runErr := runTestCommand(t, cmd, []string{"evaluator.py"})
	if runErr == nil || !contains(runErr.Error(), "only one of") {
		t.Fatalf("unexpected error: %v", runErr)
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
	if result["rule_id"] != "existing-rule" {
		t.Errorf("expected output rule_id=existing-rule, got %v", result["rule_id"])
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
	if result["rule_id"] != "existing-rule" {
		t.Errorf("expected output rule_id=existing-rule, got %v", result["rule_id"])
	}
}

// ==================== evaluators and rules ====================
const (
	testEvaluatorID  = "c065b9d1-a6cb-4f2c-be93-46ed1ac6d8d0"
	testEvaluatorID2 = "5b1d7f0e-2f4a-4c55-9a0e-7d6c1e2b3a41"
	testRuleID       = "9f5c80d7-0983-438b-b7e9-4064fc5510ab"
	testRuleID2      = "0e8f2a6c-1b3d-4e5f-8a9b-0c1d2e3f4a5b"
	testSessionID    = "3d2c1b0a-9f8e-4d7c-b6a5-4f3e2d1c0b9a"
)

// serveEvaluatorPage answers GET /platform/evaluators with evaluators on the
// first page and an empty page after it, which ends auto-paging.
func serveEvaluatorPage(w http.ResponseWriter, r *http.Request, evaluators []map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	if off := r.URL.Query().Get("offset"); off != "" && off != "0" {
		evaluators = []map[string]any{}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"evaluators": evaluators, "total": len(evaluators)})
}

func readyForTaskGrade() map[string]any {
	return map[string]any{
		"id":            testEvaluatorID,
		"name":          "ready_for_task_grade",
		"type":          "llm",
		"feedback_keys": []string{"ready"},
		"llm_evaluator": map[string]any{"prompt_repo_handle": "ready-for-task"},
		"run_rules": []map[string]any{
			{"id": testRuleID, "session_id": testSessionID, "session_name": "vanta-agent"},
		},
	}
}

func TestEvaluatorList_ShowsEvaluatorNameAndAttachedRules(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/platform/evaluators" && r.Method == http.MethodGet {
			serveEvaluatorPage(w, r, []map[string]any{readyForTaskGrade()})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		if err := runTestCommand(t, newEvaluatorListCmd(), nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 evaluator, got %d", len(result))
	}
	if result[0]["name"] != "ready_for_task_grade" || result[0]["id"] != testEvaluatorID {
		t.Errorf("expected evaluator identity, got %v", result[0])
	}
	rules, _ := result[0]["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("expected 1 attached rule, got %v", result[0]["rules"])
	}
	rule := rules[0].(map[string]any)
	if rule["rule_id"] != testRuleID || rule["project"] != "vanta-agent" {
		t.Errorf("expected rule to point at vanta-agent, got %v", rule)
	}
}

func TestEvaluatorGet_ByNameRequiresExactMatch(t *testing.T) {
	var fetchedID string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/platform/evaluators" && r.Method == http.MethodGet:
			// The API's name filter is a substring match.
			serveEvaluatorPage(w, r, []map[string]any{
				{"id": testEvaluatorID2, "name": "ready_for_task_grade_v2", "type": "llm"},
				readyForTaskGrade(),
			})
		case strings.HasPrefix(r.URL.Path, "/api/v1/platform/evaluators/") && r.Method == http.MethodGet:
			fetchedID = strings.TrimPrefix(r.URL.Path, "/api/v1/platform/evaluators/")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(readyForTaskGrade())
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		if err := runTestCommand(t, newEvaluatorGetCmd(), []string{"ready_for_task_grade"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if fetchedID != testEvaluatorID {
		t.Errorf("expected to fetch %s, fetched %q", testEvaluatorID, fetchedID)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if result["name"] != "ready_for_task_grade" {
		t.Errorf("expected name=ready_for_task_grade, got %v", result["name"])
	}
	llm, _ := result["llm_evaluator"].(map[string]any)
	if llm["prompt_repo_handle"] != "ready-for-task" {
		t.Errorf("expected llm_evaluator detail, got %v", result["llm_evaluator"])
	}
}

func TestEvaluatorGet_RuleNamePointsToItsEvaluator(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/platform/evaluators":
			serveEvaluatorPage(w, r, nil)
		case "/api/v1/runs/rules":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": testRuleID, "display_name": "test", "session_id": testSessionID,
				"evaluator_id": testEvaluatorID, "evaluator_name": "ready_for_task_grade",
			}})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	err := runTestCommand(t, newEvaluatorGetCmd(), []string{"test"})
	if err == nil || !strings.Contains(err.Error(), "ready_for_task_grade") || !strings.Contains(err.Error(), "rule name") {
		t.Fatalf("expected error naming the rule's evaluator, got %v", err)
	}
}

func TestEvaluatorGet_DuplicateNamesAsksForID(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/platform/evaluators" {
			serveEvaluatorPage(w, r, []map[string]any{
				{"id": testEvaluatorID, "name": "accuracy", "type": "code"},
				{"id": testEvaluatorID2, "name": "accuracy", "type": "llm"},
			})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	err := runTestCommand(t, newEvaluatorGetCmd(), []string{"accuracy"})
	if err == nil || !strings.Contains(err.Error(), testEvaluatorID) || !strings.Contains(err.Error(), testEvaluatorID2) {
		t.Fatalf("expected error listing both IDs, got %v", err)
	}
}

func TestEvaluatorDelete_RejectsNameWithoutCallingAPI(t *testing.T) {
	var calls int
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	cmd := newEvaluatorDeleteCmd()
	_ = cmd.Flags().Set("yes", "true")
	err := runTestCommand(t, cmd, []string{"accuracy"})
	if err == nil || !strings.Contains(err.Error(), "evaluator rule delete") {
		t.Fatalf("expected error pointing to rule delete, got %v", err)
	}
	if calls != 0 {
		t.Errorf("expected no API calls, got %d", calls)
	}
}

func TestEvaluatorDelete_WithRulesRequiresDeleteRules(t *testing.T) {
	var sawDelete bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			sawDelete = true
		}
		if r.URL.Path == "/api/v1/platform/evaluators/"+testEvaluatorID && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(readyForTaskGrade())
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	cmd := newEvaluatorDeleteCmd()
	_ = cmd.Flags().Set("yes", "true")
	err := runTestCommand(t, cmd, []string{testEvaluatorID})
	if err == nil || !strings.Contains(err.Error(), "--delete-rules") || !strings.Contains(err.Error(), "vanta-agent") {
		t.Fatalf("expected --delete-rules error naming the project, got %v", err)
	}
	if sawDelete {
		t.Error("expected no DELETE request")
	}
}

func TestEvaluatorDelete_DeletesEvaluatorAndRules(t *testing.T) {
	var deleteQuery string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/platform/evaluators/"+testEvaluatorID {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(readyForTaskGrade())
		case http.MethodDelete:
			deleteQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusNoContent)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newEvaluatorDeleteCmd()
		_ = cmd.Flags().Set("delete-rules", "true")
		_ = cmd.Flags().Set("yes", "true")
		if err := runTestCommand(t, cmd, []string{testEvaluatorID}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if deleteQuery != "delete_run_rules=true" {
		t.Errorf("expected delete_run_rules=true, got %q", deleteQuery)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if result["status"] != "deleted" || result["rules_deleted"] != float64(1) {
		t.Errorf("unexpected output: %v", result)
	}
}

func TestEvaluatorRuleList_ShowsRuleAndEvaluatorNames(t *testing.T) {
	var sessionFilter string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == http.MethodGet {
			sessionFilter = r.URL.Query().Get("session_id")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": testRuleID, "display_name": "test", "sampling_rate": 1.0, "is_enabled": true,
				"session_id": testSessionID, "session_name": "vanta-agent",
				"evaluator_id": testEvaluatorID, "evaluator_name": "ready_for_task_grade",
				"evaluator_version": 3,
			}})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newEvaluatorRuleListCmd()
		_ = cmd.Flags().Set("project-id", testSessionID)
		if err := runTestCommand(t, cmd, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if sessionFilter != testSessionID {
		t.Errorf("expected session_id filter %s, got %q", testSessionID, sessionFilter)
	}
	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(result))
	}
	got := result[0]
	if got["rule_id"] != testRuleID || got["rule_name"] != "test" {
		t.Errorf("expected rule identity, got %v", got)
	}
	if got["evaluator_id"] != testEvaluatorID || got["evaluator_name"] != "ready_for_task_grade" {
		t.Errorf("expected evaluator identity, got %v", got)
	}
	if _, ok := got["evaluator_version"]; ok {
		t.Error("evaluator_version is a rule format version and should not be shown")
	}
}

func TestEvaluatorGet_DuplicateNamesAlsoPointsToRuleEvaluator(t *testing.T) {
	const ruleEvaluatorID = "7e590565-6032-4117-8087-659a58ac172d"
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/platform/evaluators":
			serveEvaluatorPage(w, r, []map[string]any{
				{"id": testEvaluatorID, "name": "test", "type": "code"},
				{"id": testEvaluatorID2, "name": "test", "type": "code"},
			})
		case "/api/v1/runs/rules":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": testRuleID, "display_name": "test", "evaluator_id": testEvaluatorID, "evaluator_name": "test"},
				{"id": testRuleID2, "display_name": "test", "evaluator_id": ruleEvaluatorID, "evaluator_name": "ready_for_task_grade"},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	err := runTestCommand(t, newEvaluatorGetCmd(), []string{"test"})
	if err == nil {
		t.Fatal("expected an ambiguity error")
	}
	msg := err.Error()
	for _, want := range []string{testEvaluatorID, testEvaluatorID2, "ready_for_task_grade", ruleEvaluatorID} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected error to mention %q, got %v", want, msg)
		}
	}
	if strings.Count(msg, testEvaluatorID) != 1 {
		t.Errorf("expected evaluators named %q not to be repeated as rule hints, got %v", "test", msg)
	}
}

func TestEvaluatorRuleList_EvaluatorFilterWithTarget(t *testing.T) {
	var evaluatorFilter string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == http.MethodGet {
			evaluatorFilter = r.URL.Query().Get("evaluator_id")
			// The API applies session_id and ignores evaluator_id when both are set.
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": testRuleID, "display_name": "test", "session_id": testSessionID, "evaluator_id": testEvaluatorID, "evaluator_name": "ready_for_task_grade"},
				{"id": testRuleID2, "display_name": "other", "session_id": testSessionID, "evaluator_id": testEvaluatorID2, "evaluator_name": "other"},
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
		_ = cmd.Flags().Set("project-id", testSessionID)
		_ = cmd.Flags().Set("evaluator-id", testEvaluatorID)
		if err := runTestCommand(t, cmd, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if evaluatorFilter != testEvaluatorID {
		t.Errorf("expected evaluator_id filter %s, got %q", testEvaluatorID, evaluatorFilter)
	}
	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if len(result) != 1 || result[0]["rule_id"] != testRuleID {
		t.Fatalf("expected only rule %s, got %v", testRuleID, result)
	}
}

func TestEvaluatorRuleGet_ShowsCodeEvaluator(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" && r.URL.Query().Get("id") == testRuleID {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": testRuleID, "display_name": "accuracy", "dataset_id": "ds-1",
				"evaluator_id": testEvaluatorID, "evaluator_name": "accuracy_v2",
				"code_evaluators": []map[string]any{{"code": "def perform_eval(run, example):\n  return {}", "language": "python"}},
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

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if result["type"] != "code" || result["language"] != "python" || result["code"] == "" {
		t.Errorf("expected code evaluator detail, got %v", result)
	}
	if result["evaluator_name"] != "accuracy_v2" {
		t.Errorf("expected evaluator_name=accuracy_v2, got %v", result["evaluator_name"])
	}
}

func TestEvaluatorRuleDelete_NameNeedsTarget(t *testing.T) {
	var calls int
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	cmd := newEvaluatorRuleDeleteCmd()
	_ = cmd.Flags().Set("yes", "true")
	err := runTestCommand(t, cmd, []string{"accuracy"})
	if err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("expected error asking for a target, got %v", err)
	}
	if calls != 0 {
		t.Errorf("expected no API calls, got %d", calls)
	}
}

func TestEvaluatorRuleDelete_RefusesAmbiguousName(t *testing.T) {
	var sawDelete bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			sawDelete = true
		}
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": testRuleID, "display_name": "accuracy", "session_id": testSessionID, "evaluator_id": testEvaluatorID},
				{"id": testRuleID2, "display_name": "accuracy", "session_id": testSessionID, "evaluator_id": testEvaluatorID2},
			})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	cmd := newEvaluatorRuleDeleteCmd()
	_ = cmd.Flags().Set("project-id", testSessionID)
	_ = cmd.Flags().Set("yes", "true")
	err := runTestCommand(t, cmd, []string{"accuracy"})
	if err == nil || !strings.Contains(err.Error(), testRuleID) || !strings.Contains(err.Error(), testRuleID2) {
		t.Fatalf("expected error listing both rule IDs, got %v", err)
	}
	if sawDelete {
		t.Error("expected no DELETE request")
	}
}

func TestEvaluatorRuleDelete_DeletesSingleMatch(t *testing.T) {
	var deletedPath string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/runs/rules" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": testRuleID, "display_name": "accuracy", "session_id": testSessionID, "evaluator_id": testEvaluatorID},
				{"id": testRuleID2, "display_name": "toxicity", "session_id": testSessionID},
			})
		case r.Method == http.MethodDelete:
			deletedPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newEvaluatorRuleDeleteCmd()
		_ = cmd.Flags().Set("project-id", testSessionID)
		_ = cmd.Flags().Set("yes", "true")
		if err := runTestCommand(t, cmd, []string{"accuracy"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if deletedPath != "/api/v1/runs/rules/"+testRuleID {
		t.Errorf("expected DELETE of rule %s, got %q", testRuleID, deletedPath)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if result["rule_id"] != testRuleID || result["evaluator_id"] != testEvaluatorID {
		t.Errorf("unexpected output: %v", result)
	}
}

func TestEvaluatorRuleList_HidesNonEvaluatorRulesUnlessAll(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runs/rules" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": testRuleID, "display_name": "test", "evaluator_id": testEvaluatorID, "evaluator_name": "ready_for_task_grade"},
				{"id": testRuleID2, "display_name": "legacy", "code_evaluators": []map[string]any{{"code": "x", "language": "python"}}},
				{"id": "1a2b3c4d-0000-4000-8000-000000000001", "display_name": "notify", "webhooks": []map[string]any{{"url": "https://example.com"}}},
			})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	listNames := func(all bool) []string {
		out := captureStdout(t, func() {
			cmd := newEvaluatorRuleListCmd()
			if all {
				_ = cmd.Flags().Set("all", "true")
			}
			if err := runTestCommand(t, cmd, nil); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
		var result []map[string]any
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
		}
		var names []string
		for _, r := range result {
			names = append(names, r["rule_name"].(string))
		}
		return names
	}

	if got := strings.Join(listNames(false), ","); got != "test,legacy" {
		t.Errorf("expected evaluator and legacy inline rules only, got %s", got)
	}
	if got := strings.Join(listNames(true), ","); got != "test,legacy,notify" {
		t.Errorf("expected --all to include the webhook rule, got %s", got)
	}
}

func TestEvaluatorRuleDelete_RefusesNonEvaluatorRule(t *testing.T) {
	var sawDelete bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			sawDelete = true
		}
		if r.URL.Path == "/api/v1/runs/rules" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": testRuleID, "display_name": "notify", "session_id": testSessionID, "webhooks": []map[string]any{{"url": "https://example.com"}}},
			})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	cmd := newEvaluatorRuleDeleteCmd()
	_ = cmd.Flags().Set("yes", "true")
	err := runTestCommand(t, cmd, []string{testRuleID})
	if err == nil || !strings.Contains(err.Error(), "not an evaluator rule") {
		t.Fatalf("expected refusal for a webhook rule, got %v", err)
	}
	if sawDelete {
		t.Error("expected no DELETE request")
	}
}

func TestEvaluatorUploadReplaceMatchesRenamedEvaluator(t *testing.T) {
	evaluatorFile := t.TempDir() + "/eval.py"
	if err := os.WriteFile(evaluatorFile, []byte("def grade(run, example):\n    return {\"score\": 1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var patchedPath string
	var patchBody map[string]any
	var sawPost bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/runs/rules" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": testRuleID, "display_name": "test", "session_id": testSessionID,
				"evaluator_id": testEvaluatorID, "evaluator_name": "ready_for_task_grade",
			}})
		case r.URL.Path == "/api/v1/runs/rules" && r.Method == http.MethodPost:
			sawPost = true
			http.Error(w, "should not create", http.StatusInternalServerError)
		case r.Method == http.MethodPatch:
			patchedPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&patchBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": testRuleID, "evaluator_id": testEvaluatorID})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	captureStdout(t, func() {
		cmd := newEvaluatorUploadCmd()
		_ = cmd.Flags().Set("name", "ready_for_task_grade")
		_ = cmd.Flags().Set("function", "grade")
		_ = cmd.Flags().Set("project-id", testSessionID)
		_ = cmd.Flags().Set("replace", "true")
		_ = cmd.Flags().Set("yes", "true")
		if err := runTestCommand(t, cmd, []string{evaluatorFile}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if sawPost {
		t.Fatal("expected --replace to update the existing evaluator, not create a second one")
	}
	if patchedPath != "/api/v1/runs/rules/"+testRuleID {
		t.Fatalf("expected PATCH of rule %s, got %q", testRuleID, patchedPath)
	}
	if patchBody["display_name"] != "test" {
		t.Errorf("expected the rule to keep its name, got %v", patchBody["display_name"])
	}
}

func TestEvaluatorUploadWithoutReplaceRefusesRenamedEvaluator(t *testing.T) {
	evaluatorFile := t.TempDir() + "/eval.py"
	if err := os.WriteFile(evaluatorFile, []byte("def grade(run, example):\n    return {\"score\": 1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var sawWrite bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			sawWrite = true
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id": testRuleID, "display_name": "test", "session_id": testSessionID,
			"evaluator_id": testEvaluatorID, "evaluator_name": "ready_for_task_grade",
		}})
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	cmd := newEvaluatorUploadCmd()
	_ = cmd.Flags().Set("name", "ready_for_task_grade")
	_ = cmd.Flags().Set("function", "grade")
	_ = cmd.Flags().Set("project-id", testSessionID)
	err := runTestCommand(t, cmd, []string{evaluatorFile})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists error, got %v", err)
	}
	if sawWrite {
		t.Error("expected no create or update request")
	}
}

// ==================== helper ====================

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
