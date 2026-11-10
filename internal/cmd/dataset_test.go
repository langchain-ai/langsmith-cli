package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// ==================== Command structure ====================

func TestDatasetCmd_Subcommands(t *testing.T) {
	cmd := newDatasetCmd()
	expected := map[string]bool{
		"list": false, "get": false, "create": false,
		"delete": false, "export": false, "upload": false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("dataset missing subcommand %q", name)
		}
	}
}

func TestDatasetCmd_UseField(t *testing.T) {
	cmd := newDatasetCmd()
	if cmd.Use != "datasets" {
		t.Errorf("expected Use=datasets, got %q", cmd.Use)
	}
}

func TestDatasetCmd_SubcommandCount(t *testing.T) {
	cmd := newDatasetCmd()
	if got := len(cmd.Commands()); got != 6 {
		t.Errorf("expected 6 subcommands, got %d", got)
	}
}

// ==================== dataset list flags ====================

func TestDatasetListCmd_Flags(t *testing.T) {
	cmd := newDatasetListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"limit", "100", "n"},
		{"name-contains", "", ""},
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

func TestDatasetListCmd_UseField(t *testing.T) {
	cmd := newDatasetListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

// datasetPagesServer serves GET /api/v2/datasets as two pages of two datasets
// and records each request's query.
func datasetPagesServer(t *testing.T, queries *[]map[string][]string) string {
	t.Helper()
	pages := map[string]map[string]any{
		"": {
			"items":       []map[string]any{{"id": "11111111-1111-1111-1111-111111111111", "name": "first"}, {"id": "22222222-2222-2222-2222-222222222222", "name": "second"}},
			"next_cursor": "page-2",
		},
		"page-2": {
			"items":       []map[string]any{{"id": "33333333-3333-3333-3333-333333333333", "name": "third"}, {"id": "44444444-4444-4444-4444-444444444444", "name": "fourth"}},
			"next_cursor": nil,
		},
	}
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/datasets" || r.Method != http.MethodGet {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		*queries = append(*queries, r.URL.Query())
		page, ok := pages[r.URL.Query().Get("cursor")]
		if !ok {
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(page)
	})
	return ts.URL
}

func TestDatasetListFollowsCursorUpToLimit(t *testing.T) {
	var queries []map[string][]string
	defer setupTestEnv(t, datasetPagesServer(t, &queries))()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newDatasetListCmd()
		cmd.SetArgs([]string{"--limit", "3", "--name-contains", "set"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decoding output: %v\n%s", err, out)
	}
	var names []string
	for _, ds := range got {
		names = append(names, ds["name"].(string))
	}
	if strings.Join(names, ",") != "first,second,third" {
		t.Errorf("listed %v, want the first three datasets", names)
	}
	if len(queries) != 2 {
		t.Fatalf("made %d requests, want 2", len(queries))
	}
	for i, q := range queries {
		if q["page_size"][0] != "3" || q["name_contains"][0] != "set" {
			t.Errorf("request %d query %v, want page_size=3 and name_contains=set", i, q)
		}
	}
	if q := queries[1]["cursor"]; len(q) != 1 || q[0] != "page-2" {
		t.Errorf("second request cursor %v, want the first page's next_cursor", q)
	}
}

func TestResolveDatasetByNameReadsOnePage(t *testing.T) {
	var queries []map[string][]string
	defer setupTestEnv(t, datasetPagesServer(t, &queries))()

	ds, err := resolveDataset(context.Background(), MustGetClient(), "first")
	if err != nil {
		t.Fatal(err)
	}
	if ds.Name != "first" {
		t.Errorf("resolved %q, want first", ds.Name)
	}
	if len(queries) != 1 || queries[0]["name"][0] != "first" || queries[0]["page_size"][0] != "1" {
		t.Errorf("queries %v, want one request with name=first and page_size=1", queries)
	}
}

// datasetV1OnlyServer answers like a server without GET /api/v2/datasets: a
// 404 there and the v1 list, as one page of two datasets, at /api/v1/datasets.
// It records the path and query of each request.
func datasetV1OnlyServer(t *testing.T, requests *[]string) string {
	t.Helper()
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		*requests = append(*requests, r.URL.Path+"?"+r.URL.RawQuery)
		if r.URL.Path != "/api/v1/datasets" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "11111111-1111-1111-1111-111111111111", "name": "first"},
			{"id": "22222222-2222-2222-2222-222222222222", "name": "second"},
		})
	})
	return ts.URL
}

func TestDatasetListFallsBackToV1(t *testing.T) {
	var requests []string
	defer setupTestEnv(t, datasetV1OnlyServer(t, &requests))()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newDatasetListCmd()
		cmd.SetArgs([]string{"--limit", "2", "--name-contains", "set"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decoding output: %v\n%s", err, out)
	}
	if len(got) != 2 || got[0]["name"] != "first" || got[1]["name"] != "second" {
		t.Errorf("listed %v, want first and second", got)
	}
	if len(requests) != 2 || !strings.HasPrefix(requests[0], "/api/v2/datasets?") ||
		!strings.HasPrefix(requests[1], "/api/v1/datasets?") ||
		!strings.Contains(requests[1], "limit=2") || !strings.Contains(requests[1], "name_contains=set") {
		t.Errorf("requests %v, want v2 then v1 with limit=2 and name_contains=set", requests)
	}
}

func TestResolveDatasetByNameFallsBackToV1(t *testing.T) {
	var requests []string
	defer setupTestEnv(t, datasetV1OnlyServer(t, &requests))()

	ds, err := resolveDataset(context.Background(), MustGetClient(), "first")
	if err != nil {
		t.Fatal(err)
	}
	if ds.Name != "first" {
		t.Errorf("resolved %q, want first", ds.Name)
	}
	if len(requests) != 2 || !strings.Contains(requests[1], "name=first") || !strings.Contains(requests[1], "limit=1") {
		t.Errorf("requests %v, want the v1 lookup with name=first and limit=1", requests)
	}
}

func TestResolveDatasetReportsOtherErrors(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/datasets" {
			t.Errorf("fell back to v1 after a non-404 error")
		}
		http.Error(w, `{"detail":"boom"}`, http.StatusInternalServerError)
	})
	defer setupTestEnv(t, ts.URL)()

	if _, err := resolveDataset(context.Background(), MustGetClient(), "first"); err == nil {
		t.Fatal("want an error")
	}
}

// ==================== dataset get flags ====================

func TestDatasetGetCmd_Flags(t *testing.T) {
	cmd := newDatasetGetCmd()
	f := cmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("--output flag not found")
	}
	if f.Shorthand != "o" {
		t.Errorf("expected shorthand 'o', got %q", f.Shorthand)
	}
}

func TestDatasetGetCmd_ExactArgs(t *testing.T) {
	cmd := newDatasetGetCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-dataset"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestDatasetGetCmd_UseField(t *testing.T) {
	cmd := newDatasetGetCmd()
	if cmd.Use != "get NAME_OR_ID" {
		t.Errorf("expected Use='get NAME_OR_ID', got %q", cmd.Use)
	}
}

// ==================== dataset create flags ====================

func TestDatasetCreateCmd_Flags(t *testing.T) {
	cmd := newDatasetCreateCmd()
	flags := map[string]string{
		"name":        "",
		"description": "",
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

func TestDatasetCreateCmd_RequiredName(t *testing.T) {
	cmd := newDatasetCreateCmd()
	f := cmd.Flags().Lookup("name")
	if f == nil {
		t.Fatal("--name flag not found")
	}
	ann := f.Annotations
	if ann == nil {
		t.Fatal("--name has no annotations (not marked required)")
	}
	if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("--name not marked as required")
	}
}

func TestDatasetCreateCmd_UseField(t *testing.T) {
	cmd := newDatasetCreateCmd()
	if cmd.Use != "create" {
		t.Errorf("expected Use=create, got %q", cmd.Use)
	}
}

// ==================== dataset delete flags ====================

func TestDatasetDeleteCmd_Flags(t *testing.T) {
	cmd := newDatasetDeleteCmd()
	f := cmd.Flags().Lookup("yes")
	if f == nil {
		t.Fatal("--yes flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %q", f.DefValue)
	}
}

func TestDatasetDeleteCmd_ExactArgs(t *testing.T) {
	cmd := newDatasetDeleteCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-dataset"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestDatasetDeleteCmd_UseField(t *testing.T) {
	cmd := newDatasetDeleteCmd()
	if cmd.Use != "delete NAME_OR_ID" {
		t.Errorf("expected Use='delete NAME_OR_ID', got %q", cmd.Use)
	}
}

// ==================== dataset export flags ====================

func TestDatasetExportCmd_Flags(t *testing.T) {
	cmd := newDatasetExportCmd()
	f := cmd.Flags().Lookup("limit")
	if f == nil {
		t.Fatal("--limit flag not found")
	}
	if f.DefValue != "100" {
		t.Errorf("expected default 100, got %q", f.DefValue)
	}
	if f.Shorthand != "n" {
		t.Errorf("expected shorthand 'n', got %q", f.Shorthand)
	}
}

func TestDatasetExportCmd_ExactArgs(t *testing.T) {
	cmd := newDatasetExportCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"ds"}); err == nil {
		t.Error("expected error for 1 arg")
	}
	if err := cmd.Args(cmd, []string{"ds", "out.json"}); err != nil {
		t.Errorf("expected no error for 2 args, got %v", err)
	}
}

func TestDatasetExportCmd_UseField(t *testing.T) {
	cmd := newDatasetExportCmd()
	if cmd.Use != "export NAME_OR_ID OUTPUT_FILE" {
		t.Errorf("expected Use='export NAME_OR_ID OUTPUT_FILE', got %q", cmd.Use)
	}
}

// ==================== dataset upload flags ====================

func TestDatasetUploadCmd_Flags(t *testing.T) {
	cmd := newDatasetUploadCmd()
	flags := map[string]string{
		"name":        "",
		"description": "",
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

func TestDatasetUploadCmd_RequiredName(t *testing.T) {
	cmd := newDatasetUploadCmd()
	f := cmd.Flags().Lookup("name")
	ann := f.Annotations
	if ann == nil {
		t.Fatal("--name has no annotations (not marked required)")
	}
	if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("--name not marked as required")
	}
}

func TestDatasetUploadCmd_ExactArgs(t *testing.T) {
	cmd := newDatasetUploadCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"data.json"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestDatasetUploadCmd_UseField(t *testing.T) {
	cmd := newDatasetUploadCmd()
	if cmd.Use != "upload FILE_PATH" {
		t.Errorf("expected Use='upload FILE_PATH', got %q", cmd.Use)
	}
}
