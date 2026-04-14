package output

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputJSON(t *testing.T) {
	// Test stdout output (just ensure it doesn't panic)
	data := map[string]any{
		"id":   "123",
		"name": "test",
	}
	OutputJSON(data, "")
}

func TestOutputJSONToFile(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test.json")

	data := []map[string]any{
		{"id": "1", "name": "first"},
		{"id": "2", "name": "second"},
	}

	OutputJSON(data, fpath)

	content, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), `"name": "first"`) {
		t.Errorf("output file should contain first item, got: %s", content)
	}
}

func TestOutputJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test.jsonl")

	items := []map[string]any{
		{"id": "1", "name": "first"},
		{"id": "2", "name": "second"},
	}

	OutputJSONL(items, fpath)

	content, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms       *int64
		expected string
	}{
		{nil, "N/A"},
		{int64Ptr(5), "5ms"},
		{int64Ptr(999), "999ms"},
		{int64Ptr(1000), "1.00s"},
		{int64Ptr(2500), "2.50s"},
	}

	for _, tt := range tests {
		got := FormatDuration(tt.ms)
		if got != tt.expected {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.ms, got, tt.expected)
		}
	}
}

func TestPrintRunsTable(t *testing.T) {
	var buf bytes.Buffer

	runs := []map[string]any{
		{
			"start_time": "2024-01-15T10:30:00Z",
			"name":       "ChatOpenAI",
			"run_type":   "llm",
			"trace_id":   "abc123def456789012",
			"run_id":     "run123def456789012",
		},
	}

	PrintRunsTable(&buf, runs, false, "Test Runs")

	output := buf.String()
	if !strings.Contains(output, "Test Runs") {
		t.Error("expected title in output")
	}
	if !strings.Contains(output, "ChatOpenAI") {
		t.Error("expected run name in output")
	}
}

func TestOutputTree(t *testing.T) {
	ms := int64(5000)
	runs := []RunTreeData{
		{ID: "root-1", ParentRunID: "", Name: "Agent", RunType: "chain", DurationMs: &ms},
		{ID: "child-1", ParentRunID: "root-1", Name: "LLM Call", RunType: "llm", DurationMs: &ms},
		{ID: "child-2", ParentRunID: "root-1", Name: "Tool Call", RunType: "tool", DurationMs: &ms, HasError: true},
	}

	// Just ensure it doesn't panic
	OutputTree(runs, "")
}

// ---------- applyJQ ----------

func TestApplyJQ_FieldAccess(t *testing.T) {
	data := map[string]any{"name": "alice", "age": 30}
	got := applyJQ(data, ".name")
	if got != "alice" {
		t.Errorf("expected alice, got %v", got)
	}
}

func TestApplyJQ_ArrayIndex(t *testing.T) {
	data := []any{"a", "b", "c"}
	got := applyJQ(data, ".[1]")
	if got != "b" {
		t.Errorf("expected b, got %v", got)
	}
}

func TestApplyJQ_ArrayMap(t *testing.T) {
	data := []any{
		map[string]any{"id": "1", "name": "first"},
		map[string]any{"id": "2", "name": "second"},
	}
	got := applyJQ(data, ".[].name")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(arr) != 2 || arr[0] != "first" || arr[1] != "second" {
		t.Errorf("expected [first second], got %v", arr)
	}
}

func TestApplyJQ_Select(t *testing.T) {
	data := []any{
		map[string]any{"name": "alice", "active": true},
		map[string]any{"name": "bob", "active": false},
		map[string]any{"name": "carol", "active": true},
	}
	got := applyJQ(data, "[.[] | select(.active)]")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 active items, got %d", len(arr))
	}
}

func TestApplyJQ_Length(t *testing.T) {
	data := []any{1, 2, 3, 4, 5}
	got := applyJQ(data, "length")
	// gojq returns int for length
	if got != 5 {
		t.Errorf("expected 5, got %v (%T)", got, got)
	}
}

func TestApplyJQ_Pipe(t *testing.T) {
	data := map[string]any{
		"users": []any{
			map[string]any{"name": "alice"},
			map[string]any{"name": "bob"},
		},
	}
	got := applyJQ(data, ".users | length")
	if got != 2 {
		t.Errorf("expected 2, got %v", got)
	}
}

func TestApplyJQ_Keys(t *testing.T) {
	data := map[string]any{"b": 2, "a": 1, "c": 3}
	got := applyJQ(data, "keys")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(arr) != 3 || arr[0] != "a" {
		t.Errorf("expected sorted keys starting with a, got %v", arr)
	}
}

func TestApplyJQ_InvalidExpr_ReturnsOriginal(t *testing.T) {
	data := map[string]any{"name": "test"}
	got := applyJQ(data, "invalid syntax [[[")
	// Should return original data on parse error
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected original map on error, got %T", got)
	}
	if m["name"] != "test" {
		t.Errorf("expected original data preserved, got %v", got)
	}
}

func TestApplyJQ_NullInput(t *testing.T) {
	got := applyJQ(nil, ".")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestApplyJQ_NestedAccess(t *testing.T) {
	data := map[string]any{
		"meta": map[string]any{
			"pagination": map[string]any{
				"total": 42,
			},
		},
	}
	got := applyJQ(data, ".meta.pagination.total")
	// JSON round-trip turns int to float64
	if got != float64(42) {
		t.Errorf("expected 42, got %v (%T)", got, got)
	}
}

func TestOutputJSON_WithJQExpr(t *testing.T) {
	old := JQExpr
	defer func() { JQExpr = old }()

	JQExpr = ".[].name"

	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "jq_out.json")

	data := []map[string]any{
		{"name": "first", "id": "1"},
		{"name": "second", "id": "2"},
	}
	OutputJSON(data, fpath)

	content, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	s := strings.TrimSpace(string(content))
	// Should contain only the names, not the ids
	if !strings.Contains(s, "first") || !strings.Contains(s, "second") {
		t.Errorf("expected names in output, got: %s", s)
	}
	if strings.Contains(s, "id") {
		t.Errorf("expected jq to filter out id field, got: %s", s)
	}
}

func TestOutputJSON_WithoutJQExpr(t *testing.T) {
	old := JQExpr
	defer func() { JQExpr = old }()

	JQExpr = ""

	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "no_jq_out.json")

	data := map[string]any{"name": "test", "id": "1"}
	OutputJSON(data, fpath)

	content, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "name") || !strings.Contains(s, "id") {
		t.Errorf("expected full object without jq, got: %s", s)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
