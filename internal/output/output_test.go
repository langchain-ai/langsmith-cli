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

func int64Ptr(v int64) *int64 {
	return &v
}
