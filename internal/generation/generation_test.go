package generation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTracesFromJSONLFile(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "traces.jsonl")

	lines := []string{
		`{"run_id": "r1", "trace_id": "t1", "name": "root", "run_type": "chain", "parent_run_id": "", "start_time": "2024-01-15T10:30:00Z", "inputs": {"query": "hello"}, "outputs": {"response": "world"}}`,
		`{"run_id": "r2", "trace_id": "t1", "name": "llm", "run_type": "llm", "parent_run_id": "r1", "start_time": "2024-01-15T10:30:01Z", "inputs": {"prompt": "hello"}, "outputs": {"text": "world"}}`,
		`{"run_id": "r3", "trace_id": "t2", "name": "root2", "run_type": "chain", "parent_run_id": "", "start_time": "2024-01-15T11:00:00Z", "inputs": {"query": "hi"}, "outputs": {"response": "there"}}`,
	}

	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	traces, err := LoadTracesFromFile(fpath, "newest")
	if err != nil {
		t.Fatalf("LoadTracesFromFile failed: %v", err)
	}

	if len(traces) != 2 {
		t.Fatalf("expected 2 traces, got %d", len(traces))
	}

	// Newest first: t2 should be first
	if traces[0].TraceID != "t2" {
		t.Errorf("expected first trace to be t2, got %s", traces[0].TraceID)
	}
}

func TestLoadTracesFromDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two JSONL files
	f1 := filepath.Join(tmpDir, "trace1.jsonl")
	_ = os.WriteFile(f1, []byte(`{"run_id": "r1", "trace_id": "t1", "name": "root", "run_type": "chain", "start_time": "2024-01-15T10:30:00Z", "inputs": {"q": "a"}, "outputs": {"r": "b"}}`+"\n"), 0644)

	f2 := filepath.Join(tmpDir, "trace2.jsonl")
	_ = os.WriteFile(f2, []byte(`{"run_id": "r2", "trace_id": "t2", "name": "root2", "run_type": "chain", "start_time": "2024-01-15T11:00:00Z", "inputs": {"q": "c"}, "outputs": {"r": "d"}}`+"\n"), 0644)

	// Non-JSONL file should be ignored
	_ = os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("ignore me"), 0644)

	traces, err := LoadTracesFromDir(tmpDir, "alphabetical")
	if err != nil {
		t.Fatalf("LoadTracesFromDir failed: %v", err)
	}

	if len(traces) != 2 {
		t.Fatalf("expected 2 traces, got %d", len(traces))
	}

	// Alphabetical: t1 before t2
	if traces[0].TraceID != "t1" {
		t.Errorf("expected first trace to be t1, got %s", traces[0].TraceID)
	}
}

func TestGenerateDatasetFinalResponse(t *testing.T) {
	traces := []Trace{
		{
			TraceID: "t1",
			Root: RunData{
				RunID:   "r1",
				TraceID: "t1",
				Name:    "root",
				RunType: "chain",
				Inputs:  map[string]any{"query": "What is Go?"},
				Outputs: map[string]any{"answer": "A programming language"},
			},
			Runs: []RunData{
				{RunID: "r1", TraceID: "t1", Name: "root", RunType: "chain",
					Inputs: map[string]any{"query": "What is Go?"}, Outputs: map[string]any{"answer": "A programming language"}},
			},
		},
	}

	dataset := GenerateDataset(traces, "final_response", "", nil, nil, nil, false, nil)

	if len(dataset) != 1 {
		t.Fatalf("expected 1 example, got %d", len(dataset))
	}

	ex := dataset[0]
	if ex["trace_id"] != "t1" {
		t.Errorf("expected trace_id=t1, got %v", ex["trace_id"])
	}

	outputs, ok := ex["outputs"].(map[string]any)
	if !ok {
		t.Fatal("expected outputs to be a map")
	}
	if outputs["expected_response"] != "A programming language" {
		t.Errorf("expected response='A programming language', got %v", outputs["expected_response"])
	}
}

func TestGenerateDatasetTrajectory(t *testing.T) {
	traces := []Trace{
		{
			TraceID: "t1",
			Root: RunData{
				RunID:   "r1",
				TraceID: "t1",
				Name:    "agent",
				RunType: "chain",
				Inputs:  map[string]any{"query": "find me info"},
			},
			Runs: []RunData{
				{RunID: "r1", TraceID: "t1", Name: "agent", RunType: "chain",
					Inputs: map[string]any{"query": "find me info"}},
				{RunID: "r2", TraceID: "t1", Name: "search", RunType: "tool",
					ParentRunID: "r1", StartTime: "2024-01-15T10:30:01Z"},
				{RunID: "r3", TraceID: "t1", Name: "read", RunType: "tool",
					ParentRunID: "r1", StartTime: "2024-01-15T10:30:02Z"},
			},
		},
	}

	dataset := GenerateDataset(traces, "trajectory", "", nil, nil, nil, false, nil)

	if len(dataset) != 1 {
		t.Fatalf("expected 1 example, got %d", len(dataset))
	}

	outputs, ok := dataset[0]["outputs"].(map[string]any)
	if !ok {
		t.Fatal("expected outputs to be a map")
	}

	tools, ok := outputs["expected_trajectory"].([]string)
	if !ok {
		t.Fatal("expected expected_trajectory to be []string")
	}

	if len(tools) != 2 || tools[0] != "search" || tools[1] != "read" {
		t.Errorf("expected tools=[search, read], got %v", tools)
	}
}

func TestGenerateDatasetSingleStep(t *testing.T) {
	traces := []Trace{
		{
			TraceID: "t1",
			Root:    RunData{RunID: "r1", TraceID: "t1", Name: "root", RunType: "chain"},
			Runs: []RunData{
				{RunID: "r1", TraceID: "t1", Name: "root", RunType: "chain",
					Inputs: map[string]any{"q": "a"}, Outputs: map[string]any{"r": "b"}},
				{RunID: "r2", TraceID: "t1", Name: "MyLLM", RunType: "llm",
					ParentRunID: "r1", StartTime: "2024-01-15T10:30:01Z",
					Inputs: map[string]any{"prompt": "hello"}, Outputs: map[string]any{"text": "world"}},
			},
		},
	}

	// Filter by run name
	dataset := GenerateDataset(traces, "single_step", "MyLLM", nil, nil, nil, false, nil)

	if len(dataset) != 1 {
		t.Fatalf("expected 1 example, got %d", len(dataset))
	}

	if dataset[0]["node_name"] != "MyLLM" {
		t.Errorf("expected node_name=MyLLM, got %v", dataset[0]["node_name"])
	}
}

func TestExportToFile(t *testing.T) {
	tmpDir := t.TempDir()

	dataset := []map[string]any{
		{"trace_id": "t1", "inputs": map[string]any{"q": "hello"}, "outputs": map[string]any{"a": "world"}},
	}

	// Test JSON export
	jsonPath := filepath.Join(tmpDir, "output.json")
	ExportToFile(dataset, jsonPath)

	content, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	var loaded []map[string]any
	if err := json.Unmarshal(content, &loaded); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	if len(loaded) != 1 {
		t.Errorf("expected 1 example in output, got %d", len(loaded))
	}

	// Test CSV export
	csvPath := filepath.Join(tmpDir, "output.csv")
	ExportToFile(dataset, csvPath)

	csvContent, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("failed to read CSV output: %v", err)
	}

	if len(csvContent) == 0 {
		t.Error("CSV output should not be empty")
	}
}

func TestExtractFromMessages(t *testing.T) {
	messages := []any{
		map[string]any{"type": "human", "content": "What is Go?"},
		map[string]any{"type": "ai", "content": "A programming language"},
	}

	// Test human extraction
	result := ExtractFromMessages(messages, "human")
	if result != "What is Go?" {
		t.Errorf("expected 'What is Go?', got %q", result)
	}

	// Test AI extraction
	result = ExtractFromMessages(messages, "ai")
	if result != "A programming language" {
		t.Errorf("expected 'A programming language', got %q", result)
	}
}

func TestExtractValue(t *testing.T) {
	data := map[string]any{
		"query": "hello",
		"extra": "stuff",
	}

	// Test user-specified fields
	result := ExtractValue(data, []string{"query"}, nil, "", true)
	if result != "hello" {
		t.Errorf("expected 'hello', got %v", result)
	}

	// Test common fields
	result = ExtractValue(data, nil, []string{"query"}, "", true)
	if result != "hello" {
		t.Errorf("expected 'hello', got %v", result)
	}
}

func TestExtractDocuments(t *testing.T) {
	outputs := map[string]any{
		"documents": []any{
			map[string]any{"page_content": "doc 1"},
			map[string]any{"page_content": "doc 2"},
		},
	}

	chunks := ExtractDocuments(outputs)
	if len(chunks) != 2 || chunks[0] != "doc 1" {
		t.Errorf("expected [doc 1, doc 2], got %v", chunks)
	}
}
