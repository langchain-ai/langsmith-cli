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
	if err := OutputJSON(data, ""); err != nil {
		t.Fatalf("OutputJSON: %v", err)
	}
}

func TestOutputJSONToFile(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test.json")

	data := []map[string]any{
		{"id": "1", "name": "first"},
		{"id": "2", "name": "second"},
	}

	if err := OutputJSON(data, fpath); err != nil {
		t.Fatalf("OutputJSON: %v", err)
	}

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

	if err := OutputJSONL(items, fpath); err != nil {
		t.Fatalf("OutputJSONL: %v", err)
	}

	content, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestOutputJSONReturnsWriteError(t *testing.T) {
	err := OutputJSON(map[string]any{"status": "ok"}, t.TempDir())
	if err == nil {
		t.Fatal("expected an error when output path is a directory")
	}
}

func TestOutputJSONLReturnsEncodingError(t *testing.T) {
	err := OutputJSONL([]map[string]any{{"invalid": make(chan int)}}, "")
	if err == nil {
		t.Fatal("expected an error for an unsupported JSON value")
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

	PrintRunsTable(&buf, runs, false, false, "Test Runs")

	output := buf.String()
	if !strings.Contains(output, "Test Runs") {
		t.Error("expected title in output")
	}
	if !strings.Contains(output, "ChatOpenAI") {
		t.Error("expected run name in output")
	}
	if strings.Contains(output, "Feedback") {
		t.Error("did not expect a Feedback column when includeFeedback is false")
	}
}

func TestPrintRunsTable_WithFeedback(t *testing.T) {
	var buf bytes.Buffer

	runs := []map[string]any{
		{
			"start_time": "2024-01-15T10:30:00Z",
			"name":       "ChatOpenAI",
			"run_type":   "llm",
			"trace_id":   "abc123def456789012",
			"run_id":     "run123def456789012",
			"feedback_stats": map[string]map[string]interface{}{
				"correctness": {
					"avg": 0.75,
					"n":   float64(4),
				},
				"category": {
					"n": float64(2),
					"values": map[string]interface{}{
						"good": float64(1),
						"bad":  float64(1),
					},
				},
			},
		},
		{
			"start_time": "2024-01-15T10:31:00Z",
			"name":       "NoFeedbackRun",
			"run_type":   "llm",
			"trace_id":   "abc123def456789099",
			"run_id":     "run123def456789099",
		},
	}

	PrintRunsTable(&buf, runs, false, true, "Test Runs With Feedback")

	output := buf.String()
	if !strings.Contains(output, "Feedback") {
		t.Error("expected a Feedback column header when includeFeedback is true")
	}
	if !strings.Contains(output, "correctness=0.75 (n=4)") {
		t.Errorf("expected numeric feedback summary in output, got: %s", output)
	}
	if !strings.Contains(output, "category{bad=1,good=1}") {
		t.Errorf("expected categorical feedback summary in output, got: %s", output)
	}
	if !strings.Contains(output, "NoFeedbackRun") {
		t.Error("expected run with no feedback_stats to still be printed")
	}
}

func TestFormatFeedbackStats(t *testing.T) {
	tests := []struct {
		name     string
		fb       map[string]map[string]interface{}
		expected string
	}{
		{
			name:     "empty map",
			fb:       map[string]map[string]interface{}{},
			expected: "N/A",
		},
		{
			name: "numeric feedback",
			fb: map[string]map[string]interface{}{
				"correctness": {"avg": 0.5, "n": float64(2)},
			},
			expected: "correctness=0.5 (n=2)",
		},
		{
			name: "categorical feedback",
			fb: map[string]map[string]interface{}{
				"tone": {"n": float64(3), "values": map[string]interface{}{
					"friendly": float64(2),
					"neutral":  float64(1),
				}},
			},
			expected: "tone{friendly=2,neutral=1}",
		},
		{
			name: "multiple keys are sorted and joined",
			fb: map[string]map[string]interface{}{
				"zeta":  {"avg": 1.0, "n": float64(1)},
				"alpha": {"avg": 2.0, "n": float64(1)},
			},
			expected: "alpha=2 (n=1); zeta=1 (n=1)",
		},
		{
			name: "key with no recorded points is skipped",
			fb: map[string]map[string]interface{}{
				"empty": {"n": float64(0)},
			},
			expected: "N/A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFeedbackStats(tt.fb)
			if got != tt.expected {
				t.Errorf("formatFeedbackStats(%v) = %q, want %q", tt.fb, got, tt.expected)
			}
		})
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
