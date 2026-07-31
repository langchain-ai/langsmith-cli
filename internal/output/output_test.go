package output

import (
	"bytes"
	"errors"
	"io"
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

func TestOutputJSON_ReturnsMarshalError(t *testing.T) {
	err := OutputJSON(map[string]any{"unsupported": make(chan int)}, "")
	if err == nil || !strings.Contains(err.Error(), "encoding JSON") {
		t.Fatalf("error = %v, want JSON encoding error", err)
	}
}

func TestOutputJSON_ReturnsFileError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "output.json")
	err := OutputJSON(map[string]any{"ok": true}, path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %v, want path %q", err, path)
	}
}

func TestPrintOutput_ReturnsFileError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "output.json")
	err := PrintOutput(map[string]any{"ok": true}, "pretty", path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %v, want path %q", err, path)
	}
}

func TestWriteBytes_ReturnsShortWrite(t *testing.T) {
	err := writeBytes(shortWriter{}, []byte("complete output"))
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("error = %v, want io.ErrShortWrite", err)
	}
}

func TestOutputJSONL_ReturnsWriteErrorWithoutSuccess(t *testing.T) {
	writeErr := errors.New("disk full")
	writer := &errorWriteCloser{writeErr: writeErr}
	var stderr bytes.Buffer
	err := outputJSONL(
		[]map[string]any{{"id": "1"}},
		"output.jsonl",
		io.Discard,
		&stderr,
		func(string) (io.WriteCloser, error) { return writer, nil },
	)
	if !errors.Is(err, writeErr) {
		t.Fatalf("error = %v, want %v", err, writeErr)
	}
	if stderr.Len() != 0 {
		t.Errorf("failure must not print success status: %q", stderr.String())
	}
	if !writer.closed {
		t.Error("writer was not closed after write failure")
	}
}

func TestOutputJSONL_ReturnsCloseErrorWithoutSuccess(t *testing.T) {
	closeErr := errors.New("close failed")
	writer := &errorWriteCloser{closeErr: closeErr}
	var stderr bytes.Buffer
	err := outputJSONL(
		[]map[string]any{{"id": "1"}},
		"output.jsonl",
		io.Discard,
		&stderr,
		func(string) (io.WriteCloser, error) { return writer, nil },
	)
	if !errors.Is(err, closeErr) {
		t.Fatalf("error = %v, want %v", err, closeErr)
	}
	if stderr.Len() != 0 {
		t.Errorf("close failure must not print success status: %q", stderr.String())
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	return len(p) - 1, nil
}

type errorWriteCloser struct {
	bytes.Buffer
	writeErr error
	closeErr error
	closed   bool
}

func (w *errorWriteCloser) Write(p []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.Buffer.Write(p)
}

func (w *errorWriteCloser) Close() error {
	w.closed = true
	return w.closeErr
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
