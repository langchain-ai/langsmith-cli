package cmd

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const (
	testExampleDatasetID = "11111111-1111-1111-1111-111111111111"
	testExampleID        = "22222222-2222-2222-2222-222222222222"
)

// ==================== Command structure ====================

func TestExampleCmd_Subcommands(t *testing.T) {
	cmd := newExampleCmd()
	expected := map[string]bool{"list": false, "create": false, "delete": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("example missing subcommand %q", name)
		}
	}
}

func TestExampleCmd_UseField(t *testing.T) {
	cmd := newExampleCmd()
	if cmd.Use != "example" {
		t.Errorf("expected Use=example, got %q", cmd.Use)
	}
}

// ==================== Subcommand Use fields ====================

func TestExampleListCmd_UseField(t *testing.T) {
	cmd := newExampleListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

func TestExampleCreateCmd_UseField(t *testing.T) {
	cmd := newExampleCreateCmd()
	if cmd.Use != "create" {
		t.Errorf("expected Use=create, got %q", cmd.Use)
	}
}

func TestExampleDeleteCmd_UseField(t *testing.T) {
	cmd := newExampleDeleteCmd()
	if cmd.Use != "delete EXAMPLE_ID" {
		t.Errorf("expected Use='delete EXAMPLE_ID', got %q", cmd.Use)
	}
}

// ==================== example list flags ====================

func TestExampleListCmd_Flags(t *testing.T) {
	cmd := newExampleListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"dataset", "", ""},
		{"limit", "20", "n"},
		{"offset", "0", ""},
		{"split", "", ""},
		{"output", "", "o"},
	}
	for _, tc := range tests {
		f := cmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("flag --%s not found", tc.name)
			continue
		}
		if f.DefValue != tc.defVal {
			t.Errorf("flag --%s: expected default %q, got %q", tc.name, tc.defVal, f.DefValue)
		}
		if tc.short != "" && f.Shorthand != tc.short {
			t.Errorf("flag --%s: expected shorthand %q, got %q", tc.name, tc.short, f.Shorthand)
		}
	}
}

func TestExampleListCmd_RequiredDataset(t *testing.T) {
	cmd := newExampleListCmd()
	f := cmd.Flags().Lookup("dataset")
	if f == nil {
		t.Fatal("--dataset not found")
	}
	ann := f.Annotations
	if ann == nil {
		t.Fatal("--dataset has no annotations (not marked required)")
	}
	if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("--dataset not marked as required")
	}
}

// ==================== example create flags ====================

func TestExampleCreateCmd_Flags(t *testing.T) {
	cmd := newExampleCreateCmd()
	flags := map[string]string{
		"dataset":  "",
		"inputs":   "",
		"outputs":  "",
		"metadata": "",
		"split":    "",
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

func TestExampleCreateCmd_RequiredFlags(t *testing.T) {
	cmd := newExampleCreateCmd()
	for _, name := range []string{"dataset", "inputs"} {
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

// ==================== example delete flags ====================

func TestExampleDeleteCmd_Flags(t *testing.T) {
	cmd := newExampleDeleteCmd()
	f := cmd.Flags().Lookup("yes")
	if f == nil {
		t.Fatal("--yes flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %q", f.DefValue)
	}
}

func TestExampleDeleteCmd_ExactArgs(t *testing.T) {
	cmd := newExampleDeleteCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"ex-id"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestExampleList_UsesNativeSplitFilterAndOutput(t *testing.T) {
	var exampleQuery url.Values
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/datasets":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": testExampleDatasetID, "name": "my-dataset",
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/examples":
			exampleQuery = r.URL.Query()
			if exampleQuery.Get("offset") != "" && exampleQuery.Get("offset") != "0" {
				_ = json.NewEncoder(w).Encode([]any{})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":         testExampleID,
				"dataset_id": testExampleDatasetID,
				"name":       "example",
				"inputs":     map[string]any{"question": "hello"},
				"metadata":   map[string]any{"split": "metadata-value"},
				"split":      []string{"test", "validation"},
			}})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer setupTestEnv(t, srv.URL)()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newExampleListCmd()
		cmd.SetArgs([]string{"--dataset", "my-dataset", "--split", "test"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if got := exampleQuery["splits"]; len(got) != 1 || got[0] != "test" {
		t.Fatalf("splits query = %v, want [test]", got)
	}
	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if len(result) != 1 {
		t.Fatalf("got %d examples, want 1", len(result))
	}
	splits, ok := result[0]["split"].([]any)
	if !ok || len(splits) != 2 || splits[0] != "test" || splits[1] != "validation" {
		t.Errorf("native split output = %#v, want [test validation]", result[0]["split"])
	}
	metadata, _ := result[0]["metadata"].(map[string]any)
	if metadata["split"] != "metadata-value" {
		t.Errorf("metadata was changed or discarded: %#v", metadata)
	}
}

func TestExampleCreate_UsesNativeSplitWithoutChangingMetadata(t *testing.T) {
	var createBody map[string]any
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/datasets":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": testExampleDatasetID, "name": "my-dataset",
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/examples":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("decode create request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": testExampleID, "dataset_id": testExampleDatasetID,
				"name": "example", "inputs": map[string]any{"question": "hello"},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer setupTestEnv(t, srv.URL)()

	captureStdout(t, func() {
		cmd := newExampleCreateCmd()
		cmd.SetArgs([]string{
			"--dataset", "my-dataset",
			"--inputs", `{"question":"hello"}`,
			"--metadata", `{"split":"metadata-value","source":"fixture"}`,
			"--split", "test",
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if createBody["split"] != "test" {
		t.Errorf("native split = %#v, want test", createBody["split"])
	}
	metadata, _ := createBody["metadata"].(map[string]any)
	if metadata["split"] != "metadata-value" || metadata["source"] != "fixture" {
		t.Errorf("metadata was changed: %#v", metadata)
	}
}

func TestFormatExampleSplit(t *testing.T) {
	if got := formatExampleSplit([]any{"train", "validation"}); got != "train, validation" {
		t.Errorf("formatExampleSplit() = %q", got)
	}
	if got := formatExampleSplit("test"); got != "test" {
		t.Errorf("formatExampleSplit() = %q", got)
	}
	if got := formatExampleSplit(nil); got != "N/A" {
		t.Errorf("formatExampleSplit() = %q", got)
	}
	if strings.Contains(formatExampleSplit([]any{1, false}), "1") {
		t.Error("non-string split members should be ignored")
	}
}
