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
	runs := []map[string]any{
		{
			"start_time": "2024-01-15T10:30:00Z",
			"name":       "ChatOpenAI",
			"run_type":   "llm",
			"trace_id":   "abc123def456789012",
			"run_id":     "run123def456789012",
			"feedback_stats": map[string]map[string]any{
				"correctness": {"avg": 0.8, "n": 5},
				"helpfulness": {"avg": 1.0, "n": 2},
			},
		},
		{
			"start_time": "2024-01-15T10:31:00Z",
			"name":       "ToolCall",
			"run_type":   "tool",
			"trace_id":   "abc123def456789012",
			"run_id":     "run456def456789012",
		},
	}

	t.Run("without feedback flag", func(t *testing.T) {
		var buf bytes.Buffer
		PrintRunsTable(&buf, runs, false, false, "Test Runs")
		output := buf.String()

		if !strings.Contains(output, "Test Runs") {
			t.Error("expected title in output")
		}
		if !strings.Contains(output, "ChatOpenAI") {
			t.Error("expected run name in output")
		}
		if strings.Contains(output, "Feedback") {
			t.Error("expected no Feedback column when includeFeedback is false")
		}
	})

	t.Run("with feedback flag", func(t *testing.T) {
		var buf bytes.Buffer
		PrintRunsTable(&buf, runs, false, true, "Test Runs")
		output := buf.String()

		if !strings.Contains(output, "FEEDBACK") && !strings.Contains(output, "Feedback") {
			t.Error("expected Feedback column when includeFeedback is true")
		}
		if !strings.Contains(output, "correctness: 0.80 (5)") {
			t.Errorf("expected formatted correctness feedback stats in output, got:\n%s", output)
		}
		if !strings.Contains(output, "helpfulness: 1.00 (2)") {
			t.Errorf("expected formatted helpfulness feedback stats in output, got:\n%s", output)
		}
		if !strings.Contains(output, "N/A") {
			t.Error("expected N/A for run without feedback stats")
		}
	})
}

func TestFormatFeedbackStats(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "nil",
			input:    nil,
			expected: "N/A",
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: "N/A",
		},
		{
			name: "single key with avg and n",
			input: map[string]map[string]any{
				"correctness": {"avg": 0.8, "n": 5},
			},
			expected: "correctness: 0.80 (5)",
		},
		{
			name: "multiple keys sorted alphabetically",
			input: map[string]map[string]any{
				"helpfulness": {"avg": 1.0, "n": 2},
				"accuracy":    {"avg": 0.95, "n": 10},
			},
			expected: "accuracy: 0.95 (10), helpfulness: 1.00 (2)",
		},
		{
			name: "feedback stats with count instead of n",
			input: map[string]map[string]interface{}{
				"relevance": {"avg": 0.75, "count": float64(4)},
			},
			expected: "relevance: 0.75 (4)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFeedbackStats(tt.input)
			if got != tt.expected {
				t.Errorf("FormatFeedbackStats() = %q, want %q", got, tt.expected)
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
