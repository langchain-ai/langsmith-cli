package cmd

import (
	"testing"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/output"
)

// ---------- formatTimedelta ----------

func TestFormatTimedelta_Milliseconds(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.001, "1ms"},
		{0.123, "123ms"},
		{0.999, "999ms"},
		{0.0, "0ms"},
		{0.5, "500ms"},
	}
	for _, tc := range tests {
		got := formatTimedelta(tc.input)
		if got != tc.expected {
			t.Errorf("formatTimedelta(%f) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestFormatTimedelta_Seconds(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{1.0, "1.0s"},
		{5.5, "5.5s"},
		{59.9, "59.9s"},
	}
	for _, tc := range tests {
		got := formatTimedelta(tc.input)
		if got != tc.expected {
			t.Errorf("formatTimedelta(%f) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestFormatTimedelta_Minutes(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{60.0, "1m 0s"},
		{90.5, "1m 30s"},
		{125.0, "2m 5s"},
	}
	for _, tc := range tests {
		got := formatTimedelta(tc.input)
		if got != tc.expected {
			t.Errorf("formatTimedelta(%f) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// ---------- formatTimeISO ----------

func TestFormatTimeISO_ValidTime(t *testing.T) {
	tm := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	got := formatTimeISO(tm)
	if got == nil {
		t.Fatal("expected non-nil for valid time")
	}
	s, ok := got.(string)
	if !ok {
		t.Fatal("expected string type")
	}
	if s != "2024-01-15T10:30:00Z" {
		t.Errorf("expected 2024-01-15T10:30:00Z, got %q", s)
	}
}

func TestFormatTimeISO_ZeroTime(t *testing.T) {
	got := formatTimeISO(time.Time{})
	if got != nil {
		t.Errorf("expected nil for zero time, got %v", got)
	}
}

// ---------- formatTimeShort ----------

func TestFormatTimeShort_ValidTime(t *testing.T) {
	tm := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	got := formatTimeShort(tm)
	if got != "2024-03-15 14:30" {
		t.Errorf("expected '2024-03-15 14:30', got %q", got)
	}
}

func TestFormatTimeShort_ZeroTime(t *testing.T) {
	got := formatTimeShort(time.Time{})
	if got != "N/A" {
		t.Errorf("expected 'N/A', got %q", got)
	}
}

// ---------- nilStr ----------

func TestNilStr_Empty(t *testing.T) {
	got := nilStr("")
	if got != nil {
		t.Errorf("expected nil for empty string, got %v", got)
	}
}

func TestNilStr_NonEmpty(t *testing.T) {
	got := nilStr("hello")
	if got != "hello" {
		t.Errorf("expected 'hello', got %v", got)
	}
}

// ---------- nilFloat ----------

func TestNilFloat_Zero(t *testing.T) {
	got := nilFloat(0)
	if got != nil {
		t.Errorf("expected nil for zero, got %v", got)
	}
}

func TestNilFloat_NonZero(t *testing.T) {
	got := nilFloat(3.14)
	if got != 3.14 {
		t.Errorf("expected 3.14, got %v", got)
	}
}

// ---------- runsToTreeData ----------

func TestRunsToTreeData_Empty(t *testing.T) {
	result := runsToTreeData(nil)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

func TestRunsToTreeData_WithDuration(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(2500 * time.Millisecond)
	runs := []langsmith.RunQueryResponseRun{
		{
			ID:          "run-1",
			ParentRunID: "",
			Name:        "agent",
			RunType:     "chain",
			StartTime:   start,
			EndTime:     end,
			Error:       "",
		},
	}

	result := runsToTreeData(runs)
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	td := result[0]
	if td.ID != "run-1" {
		t.Errorf("expected ID run-1, got %q", td.ID)
	}
	if td.Name != "agent" {
		t.Errorf("expected Name agent, got %q", td.Name)
	}
	if td.RunType != "chain" {
		t.Errorf("expected RunType chain, got %q", td.RunType)
	}
	if td.DurationMs == nil {
		t.Fatal("expected DurationMs to be non-nil")
	}
	if *td.DurationMs != 2500 {
		t.Errorf("expected 2500ms, got %d", *td.DurationMs)
	}
	if td.HasError {
		t.Error("expected HasError=false")
	}
}

func TestRunsToTreeData_NoDuration(t *testing.T) {
	runs := []langsmith.RunQueryResponseRun{
		{
			ID:        "run-2",
			Name:      "llm",
			RunType:   "llm",
			StartTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			// EndTime is zero
		},
	}
	result := runsToTreeData(runs)
	if result[0].DurationMs != nil {
		t.Error("expected nil DurationMs when EndTime is zero")
	}
}

func TestRunsToTreeData_WithError(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(100 * time.Millisecond)
	runs := []langsmith.RunQueryResponseRun{
		{
			ID:        "run-3",
			Name:      "tool",
			RunType:   "tool",
			StartTime: start,
			EndTime:   end,
			Error:     "something went wrong",
		},
	}
	result := runsToTreeData(runs)
	if !result[0].HasError {
		t.Error("expected HasError=true when Error is non-empty")
	}
}

func TestRunsToTreeData_ParentRunID(t *testing.T) {
	runs := []langsmith.RunQueryResponseRun{
		{
			ID:          "child-1",
			ParentRunID: "parent-1",
			Name:        "sub-step",
			RunType:     "chain",
		},
	}
	result := runsToTreeData(runs)
	if result[0].ParentRunID != "parent-1" {
		t.Errorf("expected ParentRunID parent-1, got %q", result[0].ParentRunID)
	}
}

// ---------- extractRunsToMaps ----------

func TestExtractRunsToMaps_Empty(t *testing.T) {
	result := extractRunsToMaps(nil, false, false, false)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

func TestExtractRunsToMaps_BasicFields(t *testing.T) {
	runs := []langsmith.RunQueryResponseRun{
		{
			ID:      "r1",
			TraceID: "t1",
			Name:    "test-run",
			RunType: "llm",
		},
	}
	result := extractRunsToMaps(runs, false, false, false)
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	m := result[0]
	if m["run_id"] != "r1" {
		t.Errorf("expected run_id=r1, got %v", m["run_id"])
	}
	if m["trace_id"] != "t1" {
		t.Errorf("expected trace_id=t1, got %v", m["trace_id"])
	}
	if m["name"] != "test-run" {
		t.Errorf("expected name=test-run, got %v", m["name"])
	}
}

// ---------- Ensure output.RunTreeData fields are exercised ----------

func TestRunTreeData_AllFields(t *testing.T) {
	// Just ensure we can construct the struct with all fields (compile-time check mostly)
	ms := int64(100)
	td := output.RunTreeData{
		ID:          "id",
		ParentRunID: "pid",
		Name:        "name",
		RunType:     "chain",
		DurationMs:  &ms,
		HasError:    true,
	}
	if td.ID != "id" || td.ParentRunID != "pid" || td.Name != "name" ||
		td.RunType != "chain" || *td.DurationMs != 100 || !td.HasError {
		t.Error("unexpected RunTreeData field values")
	}
}
