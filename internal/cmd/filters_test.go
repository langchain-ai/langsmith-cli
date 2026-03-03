package cmd

import (
	"testing"
	"time"
)

func TestBuildFilterDSL_Empty(t *testing.T) {
	f := &FilterFlags{}
	result := buildFilterDSL(f)
	if result != "" {
		t.Errorf("expected empty filter DSL, got %q", result)
	}
}

func TestBuildFilterDSL_SingleName(t *testing.T) {
	f := &FilterFlags{Name: "ChatOpenAI"}
	result := buildFilterDSL(f)
	expected := `eq(name, "ChatOpenAI")`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildFilterDSL_MultipleTraceIDs(t *testing.T) {
	f := &FilterFlags{TraceIDs: "abc123, def456"}
	result := buildFilterDSL(f)
	expected := `in(trace_id, ["abc123", "def456"])`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildFilterDSL_SingleTraceID(t *testing.T) {
	// Single trace ID should NOT appear in filter DSL (it goes in params.Trace)
	f := &FilterFlags{TraceIDs: "abc123"}
	result := buildFilterDSL(f)
	if result != "" {
		t.Errorf("single trace ID should not be in filter DSL, got %q", result)
	}
}

func TestBuildFilterDSL_LatencyFilters(t *testing.T) {
	f := &FilterFlags{MinLatency: 2.5, MaxLatency: 10.0}
	result := buildFilterDSL(f)
	expected := `and(gte(latency, 2.5), lte(latency, 10))`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildFilterDSL_SingleTag(t *testing.T) {
	f := &FilterFlags{Tags: "production"}
	result := buildFilterDSL(f)
	expected := `has(tags, "production")`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildFilterDSL_MultipleTags(t *testing.T) {
	f := &FilterFlags{Tags: "production, v2"}
	result := buildFilterDSL(f)
	expected := `or(has(tags, "production"), has(tags, "v2"))`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildFilterDSL_TokenFilter(t *testing.T) {
	// MinTokens is filtered client-side, not via server-side DSL
	f := &FilterFlags{MinTokens: 1000}
	result := buildFilterDSL(f)
	if result != "" {
		t.Errorf("expected empty filter DSL for MinTokens (client-side only), got %q", result)
	}
}

func TestBuildFilterDSL_RawFilter(t *testing.T) {
	f := &FilterFlags{RawFilter: `eq(status, "error")`}
	result := buildFilterDSL(f)
	expected := `eq(status, "error")`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildFilterDSL_Combined(t *testing.T) {
	f := &FilterFlags{
		Name:       "ChatOpenAI",
		MinLatency: 2.5,
		Tags:       "prod",
	}
	result := buildFilterDSL(f)
	expected := `and(eq(name, "ChatOpenAI"), gte(latency, 2.5), has(tags, "prod"))`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSplitTrim(t *testing.T) {
	result := splitTrim("  abc , def , ghi  ")
	if len(result) != 3 || result[0] != "abc" || result[1] != "def" || result[2] != "ghi" {
		t.Errorf("expected [abc, def, ghi], got %v", result)
	}
}

func TestSplitTrimEmpty(t *testing.T) {
	result := splitTrim("")
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestResolveProject(t *testing.T) {
	// Flag value takes precedence
	result := ResolveProject("my-project")
	if result != "my-project" {
		t.Errorf("expected my-project, got %s", result)
	}

	// Empty flag falls back to env (tested indirectly)
	_ = ResolveProject("")
	// Will be empty or env value
}

func TestResolveProject_EnvFallback(t *testing.T) {
	t.Setenv("LANGSMITH_PROJECT", "env-project")
	result := ResolveProject("")
	if result != "env-project" {
		t.Errorf("expected env-project, got %s", result)
	}
}

// ---------- BuildRunQueryParams ----------

func TestBuildRunQueryParams_DefaultLimit(t *testing.T) {
	f := &FilterFlags{}
	params := BuildRunQueryParams(f, false, 42)
	if params.Limit.Value != 42 {
		t.Errorf("expected default limit 42, got %d", params.Limit.Value)
	}
}

func TestBuildRunQueryParams_CustomLimit(t *testing.T) {
	f := &FilterFlags{Limit: 10}
	params := BuildRunQueryParams(f, false, 50)
	if params.Limit.Value != 10 {
		t.Errorf("expected limit 10, got %d", params.Limit.Value)
	}
}

func TestBuildRunQueryParams_IsRoot(t *testing.T) {
	f := &FilterFlags{}
	params := BuildRunQueryParams(f, true, 20)
	if !params.IsRoot.Present || params.IsRoot.Value != true {
		t.Error("expected IsRoot=true when isRoot arg is true")
	}

	params2 := BuildRunQueryParams(f, false, 20)
	if params2.IsRoot.Present {
		t.Error("expected IsRoot not set when isRoot arg is false")
	}
}

func TestBuildRunQueryParams_RunType(t *testing.T) {
	f := &FilterFlags{RunType: "llm"}
	params := BuildRunQueryParams(f, false, 20)
	if !params.RunType.Present || string(params.RunType.Value) != "llm" {
		t.Errorf("expected RunType=llm, got %v", params.RunType)
	}
}

func TestBuildRunQueryParams_ErrorFlag(t *testing.T) {
	f := &FilterFlags{ErrorFlag: true}
	params := BuildRunQueryParams(f, false, 20)
	if !params.Error.Present || params.Error.Value != true {
		t.Error("expected Error=true")
	}
}

func TestBuildRunQueryParams_NoErrorFlag(t *testing.T) {
	f := &FilterFlags{NoErrorFlag: true}
	params := BuildRunQueryParams(f, false, 20)
	if !params.Error.Present || params.Error.Value != false {
		t.Error("expected Error=false")
	}
}

func TestBuildRunQueryParams_SingleTraceID(t *testing.T) {
	f := &FilterFlags{TraceIDs: "abc123"}
	params := BuildRunQueryParams(f, false, 20)
	if !params.Trace.Present || params.Trace.Value != "abc123" {
		t.Errorf("expected Trace=abc123, got %v", params.Trace)
	}
	// Should not be in filter DSL
	if params.Filter.Present {
		t.Errorf("single trace ID should not produce filter DSL, got %q", params.Filter.Value)
	}
}

func TestBuildRunQueryParams_MultipleTraceIDs(t *testing.T) {
	f := &FilterFlags{TraceIDs: "abc123,def456"}
	params := BuildRunQueryParams(f, false, 20)
	// Should NOT set Trace for multiple IDs
	if params.Trace.Present {
		t.Error("multiple trace IDs should not set Trace param")
	}
	// Should be in filter DSL
	if !params.Filter.Present {
		t.Fatal("expected Filter to be set for multiple trace IDs")
	}
	expected := `in(trace_id, ["abc123", "def456"])`
	if params.Filter.Value != expected {
		t.Errorf("expected filter %q, got %q", expected, params.Filter.Value)
	}
}

func TestBuildRunQueryParams_LastNMinutes(t *testing.T) {
	before := time.Now().UTC().Add(-31 * time.Minute)
	f := &FilterFlags{LastNMinutes: 30}
	params := BuildRunQueryParams(f, false, 20)
	if !params.StartTime.Present {
		t.Fatal("expected StartTime to be set")
	}
	after := time.Now().UTC().Add(-29 * time.Minute)
	st := params.StartTime.Value
	if st.Before(before) || st.After(after) {
		t.Errorf("expected StartTime ~30 minutes ago, got %v", st)
	}
}

func TestBuildRunQueryParams_Since(t *testing.T) {
	f := &FilterFlags{Since: "2024-01-15T10:00:00Z"}
	params := BuildRunQueryParams(f, false, 20)
	if !params.StartTime.Present {
		t.Fatal("expected StartTime to be set")
	}
	expected := "2024-01-15T10:00:00Z"
	got := params.StartTime.Value.Format("2006-01-02T15:04:05Z07:00")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestBuildRunQueryParams_FilterDSLWithName(t *testing.T) {
	f := &FilterFlags{Name: "ChatOpenAI"}
	params := BuildRunQueryParams(f, false, 20)
	if !params.Filter.Present {
		t.Fatal("expected Filter to be set")
	}
	expected := `eq(name, "ChatOpenAI")`
	if params.Filter.Value != expected {
		t.Errorf("expected %q, got %q", expected, params.Filter.Value)
	}
}

func TestBuildRunQueryParams_NoFilter(t *testing.T) {
	f := &FilterFlags{}
	params := BuildRunQueryParams(f, false, 20)
	if params.Filter.Present {
		t.Errorf("expected no filter, got %q", params.Filter.Value)
	}
}

// ---------- addCommonFilterFlags ----------

func TestAddCommonFilterFlags_AllPresent(t *testing.T) {
	cmd := newRunListCmd()
	flags := []string{
		"trace-ids", "limit", "project", "last-n-minutes", "since",
		"error", "no-error", "name", "min-latency", "max-latency",
		"min-tokens", "tags", "filter", "run-type",
	}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not found on run list command", name)
		}
	}
}

func TestAddCommonFilterFlags_WithoutRunType(t *testing.T) {
	cmd := newTraceListCmd()
	if cmd.Flags().Lookup("run-type") != nil {
		t.Error("trace list should not have --run-type flag")
	}
}

func TestAddCommonFilterFlags_LimitShorthand(t *testing.T) {
	cmd := newRunListCmd()
	f := cmd.Flags().Lookup("limit")
	if f == nil {
		t.Fatal("--limit flag not found")
	}
	if f.Shorthand != "n" {
		t.Errorf("expected shorthand 'n', got %q", f.Shorthand)
	}
}

func TestAddCommonFilterFlags_DefaultValues(t *testing.T) {
	cmd := newRunListCmd()
	defaults := map[string]string{
		"trace-ids":     "",
		"limit":         "0",
		"project":       "",
		"last-n-minutes": "0",
		"since":         "",
		"error":         "false",
		"no-error":      "false",
		"name":          "",
		"min-latency":   "0",
		"max-latency":   "0",
		"min-tokens":    "0",
		"tags":          "",
		"filter":        "",
		"run-type":      "",
	}
	for name, defVal := range defaults {
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

func TestBuildRunQueryParams_SinceWithoutTimezone(t *testing.T) {
	f := &FilterFlags{Since: "2024-01-15T10:00:00"}
	params := BuildRunQueryParams(f, false, 20)
	if !params.StartTime.Present {
		t.Fatal("expected StartTime to be set for non-RFC3339 format")
	}
	expected := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	if !params.StartTime.Value.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, params.StartTime.Value)
	}
}
