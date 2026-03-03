package cmd

import (
	"testing"
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
	f := &FilterFlags{MinTokens: 1000}
	result := buildFilterDSL(f)
	expected := `gte(total_tokens, 1000)`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
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
