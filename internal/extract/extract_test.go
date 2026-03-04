package extract

import (
	"testing"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestExtractRunBase(t *testing.T) {
	start := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 10, 30, 5, 0, time.UTC)

	run := langsmith.RunQueryResponseRun{
		ID:          "run-123",
		TraceID:     "trace-456",
		Name:        "ChatOpenAI",
		RunType:     "llm",
		ParentRunID: "parent-789",
		StartTime:   start,
		EndTime:     end,
	}

	result := ExtractRun(run, false, false, false)

	if result["run_id"] != "run-123" {
		t.Errorf("expected run_id=run-123, got %v", result["run_id"])
	}
	if result["trace_id"] != "trace-456" {
		t.Errorf("expected trace_id=trace-456, got %v", result["trace_id"])
	}
	if result["name"] != "ChatOpenAI" {
		t.Errorf("expected name=ChatOpenAI, got %v", result["name"])
	}
	if result["run_type"] != "llm" {
		t.Errorf("expected run_type=llm, got %v", result["run_type"])
	}
	if result["parent_run_id"] != "parent-789" {
		t.Errorf("expected parent_run_id=parent-789, got %v", result["parent_run_id"])
	}

	// Should NOT have metadata or IO fields
	if _, ok := result["status"]; ok {
		t.Error("base extraction should not include status")
	}
	if _, ok := result["inputs"]; ok {
		t.Error("base extraction should not include inputs")
	}
}

func TestExtractRunWithMetadata(t *testing.T) {
	start := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 10, 30, 5, 0, time.UTC)

	run := langsmith.RunQueryResponseRun{
		ID:               "run-123",
		TraceID:          "trace-456",
		Name:             "ChatOpenAI",
		RunType:          "llm",
		StartTime:        start,
		EndTime:          end,
		Status:           "success",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		PromptCost:       "0.001",
		CompletionCost:   "0.0005",
		TotalCost:        "0.0015",
		Tags:             []string{"production", "v2"},
		Extra:            map[string]any{"metadata": map[string]any{"model": "gpt-4"}},
	}

	result := ExtractRun(run, true, false, false)

	if result["status"] != "success" {
		t.Errorf("expected status=success, got %v", result["status"])
	}

	durationMs, ok := result["duration_ms"].(int64)
	if !ok || durationMs != 5000 {
		t.Errorf("expected duration_ms=5000, got %v", result["duration_ms"])
	}

	tokenUsage := result["token_usage"].(map[string]any)
	if tokenUsage["prompt_tokens"] != int64(100) {
		t.Errorf("expected prompt_tokens=100, got %v", tokenUsage["prompt_tokens"])
	}
	if tokenUsage["total_tokens"] != int64(150) {
		t.Errorf("expected total_tokens=150, got %v", tokenUsage["total_tokens"])
	}

	costs := result["costs"].(map[string]any)
	if costs["total_cost"] != 0.0015 {
		t.Errorf("expected total_cost=0.0015, got %v", costs["total_cost"])
	}

	tags := result["tags"].([]string)
	if len(tags) != 2 || tags[0] != "production" {
		t.Errorf("expected tags=[production, v2], got %v", tags)
	}

	meta := result["custom_metadata"].(map[string]any)
	if meta["model"] != "gpt-4" {
		t.Errorf("expected custom_metadata.model=gpt-4, got %v", meta["model"])
	}
}

func TestExtractRunWithIO(t *testing.T) {
	run := langsmith.RunQueryResponseRun{
		ID:        "run-123",
		TraceID:   "trace-456",
		Name:      "ChatOpenAI",
		RunType:   "llm",
		StartTime: time.Now(),
		Inputs:    map[string]any{"query": "hello"},
		Outputs:   map[string]any{"response": "world"},
		Error:     "some error",
	}

	result := ExtractRun(run, false, true, false)

	inputs := result["inputs"].(map[string]any)
	if inputs["query"] != "hello" {
		t.Errorf("expected inputs.query=hello, got %v", inputs["query"])
	}

	outputs := result["outputs"].(map[string]any)
	if outputs["response"] != "world" {
		t.Errorf("expected outputs.response=world, got %v", outputs["response"])
	}

	if result["error"] != "some error" {
		t.Errorf("expected error='some error', got %v", result["error"])
	}
}

func TestExtractRunNilParent(t *testing.T) {
	run := langsmith.RunQueryResponseRun{
		ID:        "run-123",
		TraceID:   "trace-456",
		Name:      "root",
		RunType:   "chain",
		StartTime: time.Now(),
	}

	result := ExtractRun(run, false, false, false)

	if result["parent_run_id"] != nil {
		t.Errorf("expected parent_run_id=nil, got %v", result["parent_run_id"])
	}
}

func TestExtractRunWithFeedback(t *testing.T) {
	run := langsmith.RunQueryResponseRun{
		ID:        "run-123",
		TraceID:   "trace-456",
		Name:      "ChatOpenAI",
		RunType:   "llm",
		StartTime: time.Now(),
		FeedbackStats: map[string]map[string]interface{}{
			"correctness": {"avg": 0.8, "count": float64(5)},
			"helpfulness": {"avg": 1.0, "count": float64(3)},
		},
	}

	result := ExtractRun(run, false, false, true)

	fs, ok := result["feedback_stats"]
	if !ok {
		t.Fatal("expected feedback_stats key to be present")
	}
	stats, ok := fs.(map[string]map[string]interface{})
	if !ok {
		t.Fatalf("expected feedback_stats to be map, got %T", fs)
	}
	if stats["correctness"]["avg"] != 0.8 {
		t.Errorf("expected correctness avg=0.8, got %v", stats["correctness"]["avg"])
	}
	if stats["helpfulness"]["avg"] != 1.0 {
		t.Errorf("expected helpfulness avg=1.0, got %v", stats["helpfulness"]["avg"])
	}
}

func TestExtractRunWithFeedbackEmpty(t *testing.T) {
	run := langsmith.RunQueryResponseRun{
		ID:        "run-123",
		TraceID:   "trace-456",
		Name:      "ChatOpenAI",
		RunType:   "llm",
		StartTime: time.Now(),
	}

	result := ExtractRun(run, false, false, true)

	if result["feedback_stats"] != nil {
		t.Errorf("expected feedback_stats=nil for run with no feedback, got %v", result["feedback_stats"])
	}
}

func TestExtractRunWithoutFeedbackFlag(t *testing.T) {
	run := langsmith.RunQueryResponseRun{
		ID:        "run-123",
		TraceID:   "trace-456",
		Name:      "ChatOpenAI",
		RunType:   "llm",
		StartTime: time.Now(),
		FeedbackStats: map[string]map[string]interface{}{
			"correctness": {"avg": 0.8},
		},
	}

	result := ExtractRun(run, false, false, false)

	if _, ok := result["feedback_stats"]; ok {
		t.Error("feedback_stats should not be present when includeFeedback=false")
	}
}

func TestFormatDurationHuman(t *testing.T) {
	tests := []struct {
		ms       int64
		expected string
	}{
		{5, "5ms"},
		{500, "500ms"},
		{999, "999ms"},
		{1000, "1.00s"},
		{2500, "2.50s"},
		{60000, "60.00s"},
	}

	for _, tt := range tests {
		got := FormatDurationHuman(tt.ms)
		if got != tt.expected {
			t.Errorf("FormatDurationHuman(%d) = %q, want %q", tt.ms, got, tt.expected)
		}
	}
}
