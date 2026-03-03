package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

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
	// Should contain the closing brace of the function
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
	rules := []evaluatorRule{
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
	rules := []evaluatorRule{
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
	rules := []evaluatorRule{
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
	rules := []evaluatorRule{
		{ID: "1", DisplayName: "accuracy", DatasetID: "ds-1"},
	}
	// Name matches but target dataset doesn't
	result := findEvaluator(rules, "accuracy", "ds-other", "")
	if result != nil {
		t.Error("expected nil when target doesn't match")
	}
}

// ==================== Command structure tests ====================

func TestEvaluatorCmd_Subcommands(t *testing.T) {
	cmd := newEvaluatorCmd()
	expected := map[string]bool{"list": false, "upload": false, "delete": false}
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
	// name and function are required
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
	// Should reject 0 args
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	// Should accept 1 arg
	if err := cmd.Args(cmd, []string{"eval.py"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
	// Should reject 2 args
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
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

// ==================== Execution tests (evaluator list uses RawGet) ====================

func TestEvaluatorListCmd_Execute(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/runs/rules" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]evaluatorRule{
				{
					ID:           "eval-1",
					DisplayName:  "accuracy",
					SamplingRate: 1.0,
					IsEnabled:    true,
					DatasetID:    "ds-1",
				},
				{
					ID:           "eval-2",
					DisplayName:  "toxicity",
					SamplingRate: 0.5,
					IsEnabled:    false,
					SessionID:    "proj-1",
				},
			})
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newEvaluatorListCmd()
		cmd.Run(cmd, nil)
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
	// Check sampling_rate
	if result[0]["sampling_rate"] != 1.0 {
		t.Errorf("expected sampling_rate=1.0, got %v", result[0]["sampling_rate"])
	}
	if result[1]["is_enabled"] != false {
		t.Errorf("expected is_enabled=false, got %v", result[1]["is_enabled"])
	}
}

func TestEvaluatorListCmd_Execute_PrettyFormat(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/runs/rules" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]evaluatorRule{
				{
					ID:           "eval-1",
					DisplayName:  "accuracy",
					SamplingRate: 1.0,
					IsEnabled:    true,
					DatasetID:    "ds-1",
				},
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
		cmd.Run(cmd, nil)
	})

	// Pretty format should produce a table, not JSON
	if len(out) > 0 && out[0] == '[' {
		t.Error("pretty format should not produce JSON array")
	}
	// Should contain the evaluator name somewhere
	if !contains(out, "accuracy") {
		t.Error("pretty output should contain evaluator name")
	}
}

func TestEvaluatorListCmd_Execute_EmptyList(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/runs/rules" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
			return
		}
		http.Error(w, "not found", 404)
	})

	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newEvaluatorListCmd()
		cmd.Run(cmd, nil)
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	// Should produce empty JSON array (or null)
	// Either is acceptable since the handler returns empty slice
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
		cmd.Run(cmd, nil)
	})

	if receivedKey != "test-api-key" {
		t.Errorf("expected x-api-key=test-api-key, got %q", receivedKey)
	}
}

// ==================== helper ====================

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
