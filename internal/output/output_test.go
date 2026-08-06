package output

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
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

const hostileTitle = "Runs\x1b]52;c;cHduZWQ=\x07"

func TestOutputTableSanitizesTitleColumnsAndCells(t *testing.T) {
	out := captureStdout(t, func() {
		OutputTable(
			[]string{"Name\x1b[2K", "Value"},
			[][]string{{"row\rone", "line\ntwo"}},
			hostileTitle,
		)
	})
	assertNoControlChars(t, out)
	if !strings.Contains(out, "cHduZWQ=") {
		t.Errorf("expected escape payload to remain visible as text: %q", out)
	}
}

func TestOutputTableUnderlineCountsRunes(t *testing.T) {
	out := captureStdout(t, func() {
		OutputTable([]string{"A"}, [][]string{{"b"}}, "Café")
	})
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected a title and an underline: %q", out)
	}
	if got, want := utf8.RuneCountInString(lines[1]), 4; got != want {
		t.Errorf("underline is %d runes, want %d: %q", got, want, lines[1])
	}
}

func TestOutputTreeSanitizesLabels(t *testing.T) {
	runs := []RunTreeData{
		{ID: "root", Name: "parent\x1b[2K", RunType: "chain\r"},
		{ID: "child", ParentRunID: "root", Name: "kid\nfaked", RunType: "llm", HasError: true},
	}
	out := captureStdout(t, func() {
		OutputTree(runs, "root")
	})
	assertNoControlChars(t, out)
	if strings.Count(out, "\n") != 2 {
		t.Errorf("expected exactly one line per run, got %q", out)
	}
}

func TestPrintRunsTableSanitizesValues(t *testing.T) {
	var buf bytes.Buffer
	runs := []map[string]any{{
		"name":     "run\x1b]52;c;cHduZWQ=\x07",
		"run_type": "chain\r",
		"trace_id": "t\x1b1",
		"run_id":   "r\n1",
		"status":   "ok\x1b",
	}}
	PrintRunsTable(&buf, runs, true, hostileTitle)
	assertNoControlChars(t, buf.String())
}

func TestPrintRunsTableTruncatesNameByRunes(t *testing.T) {
	var buf bytes.Buffer
	name := strings.Repeat("é", 50)
	PrintRunsTable(&buf, []map[string]any{{"name": name}}, false, "")
	out := buf.String()
	if !utf8.ValidString(out) {
		t.Fatalf("truncation split a character: %q", out)
	}
	if strings.Contains(out, strings.Repeat("é", 41)) {
		t.Errorf("name was not truncated to 40 runes: %q", out)
	}
	if !strings.Contains(out, strings.Repeat("é", 40)) {
		t.Errorf("expected 40 runes of the name: %q", out)
	}
}

func TestPrintRunsTableFormatsStartTime(t *testing.T) {
	var buf bytes.Buffer
	PrintRunsTable(&buf, []map[string]any{{"start_time": "2024-01-02T03:04:05.123456+00:00"}}, false, "")
	if !strings.Contains(buf.String(), "03:04:05") {
		t.Errorf("expected clock time from start_time: %q", buf.String())
	}
}

func TestPrintRunsTableRejectsUnparseableStartTime(t *testing.T) {
	var buf bytes.Buffer
	PrintRunsTable(&buf, []map[string]any{{"start_time": "éééééééééééé"}}, false, "")
	out := buf.String()
	assertNoControlChars(t, out)
	if !strings.Contains(out, "N/A") {
		t.Errorf("expected N/A for an unparseable start_time: %q", out)
	}
}
