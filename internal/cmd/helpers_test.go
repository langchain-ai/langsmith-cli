package cmd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
)

// ---------- resolveSessionID ----------
//
// These cases all resolve (or error) before any API call, so a nil client is
// safe. The project-name → session-ID lookup path is exercised via the
// command-level tests that stub the client.

func TestResolveSessionID_ProjectIDTakesPrecedence(t *testing.T) {
	// A valid --project-id is returned as-is, even when a project name is set
	// via --project / $LANGSMITH_PROJECT — without any name lookup.
	t.Setenv("LANGSMITH_PROJECT", "some-project-name")
	const id = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

	got, err := resolveSessionID(context.Background(), nil, "another-name", id, "trace get")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != id {
		t.Errorf("expected session id %q returned verbatim, got %q", id, got)
	}
}

func TestResolveSessionID_InvalidProjectID(t *testing.T) {
	_, err := resolveSessionID(context.Background(), nil, "", "not-a-uuid", "trace get")
	if err == nil {
		t.Fatal("expected error for malformed --project-id")
	}
	if !strings.Contains(err.Error(), "--project-id") {
		t.Errorf("error should mention --project-id, got %q", err.Error())
	}
}

func TestResolveSessionID_NeitherProvided(t *testing.T) {
	t.Setenv("LANGSMITH_PROJECT", "")
	_, err := resolveSessionID(context.Background(), nil, "", "", "trace stats")
	if err == nil {
		t.Fatal("expected error when neither --project nor --project-id is provided")
	}
	if !strings.Contains(err.Error(), "trace stats") {
		t.Errorf("error should name the command, got %q", err.Error())
	}
}

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
	runs := []langsmith.RunSchema{
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
	runs := []langsmith.RunSchema{
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
	runs := []langsmith.RunSchema{
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
	runs := []langsmith.RunSchema{
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

// ---------- buildRunSelect ----------

func TestBuildRunSelect_NeitherFlag(t *testing.T) {
	sel := buildRunSelect(false, false)
	if sel != nil {
		t.Errorf("expected nil when neither IO nor feedback requested, got %d fields", len(sel))
	}
}

func TestBuildRunSelect_IOOnly(t *testing.T) {
	sel := buildRunSelect(true, false)
	if sel == nil {
		t.Fatal("expected non-nil when IO requested")
	}

	has := selectSet(sel)

	// Must include IO fields
	for _, f := range []langsmith.RunQueryParamsSelect{
		langsmith.RunQueryParamsSelectInputs,
		langsmith.RunQueryParamsSelectOutputs,
		langsmith.RunQueryParamsSelectError,
	} {
		if !has[f] {
			t.Errorf("missing IO field %q", f)
		}
	}

	// Must NOT include feedback
	if has[langsmith.RunQueryParamsSelectFeedbackStats] {
		t.Error("should not include feedback_stats when only IO requested")
	}

	// Must include base fields
	assertBaseFields(t, has)
}

func TestBuildRunSelect_FeedbackOnly(t *testing.T) {
	sel := buildRunSelect(false, true)
	if sel == nil {
		t.Fatal("expected non-nil when feedback requested")
	}

	has := selectSet(sel)

	// Must include feedback
	if !has[langsmith.RunQueryParamsSelectFeedbackStats] {
		t.Error("missing feedback_stats")
	}

	// Must NOT include IO fields
	for _, f := range []langsmith.RunQueryParamsSelect{
		langsmith.RunQueryParamsSelectInputs,
		langsmith.RunQueryParamsSelectOutputs,
	} {
		if has[f] {
			t.Errorf("should not include %q when only feedback requested", f)
		}
	}

	// Must include base fields
	assertBaseFields(t, has)
}

func TestBuildRunSelect_Both(t *testing.T) {
	sel := buildRunSelect(true, true)
	if sel == nil {
		t.Fatal("expected non-nil when both requested")
	}

	has := selectSet(sel)

	// Must include IO fields
	for _, f := range []langsmith.RunQueryParamsSelect{
		langsmith.RunQueryParamsSelectInputs,
		langsmith.RunQueryParamsSelectOutputs,
		langsmith.RunQueryParamsSelectError,
	} {
		if !has[f] {
			t.Errorf("missing IO field %q", f)
		}
	}

	// Must include feedback
	if !has[langsmith.RunQueryParamsSelectFeedbackStats] {
		t.Error("missing feedback_stats")
	}

	// Must include base fields
	assertBaseFields(t, has)
}

func TestBuildRunSelect_NoDuplicates(t *testing.T) {
	sel := buildRunSelect(true, true)
	seen := make(map[langsmith.RunQueryParamsSelect]bool)
	for _, f := range sel {
		if seen[f] {
			t.Errorf("duplicate select field: %q", f)
		}
		seen[f] = true
	}
}

func TestBuildRunSelect_IncludesMetadataFields(t *testing.T) {
	sel := buildRunSelect(true, false)
	has := selectSet(sel)

	metadataFields := []langsmith.RunQueryParamsSelect{
		langsmith.RunQueryParamsSelectStatus,
		langsmith.RunQueryParamsSelectExtra,
		langsmith.RunQueryParamsSelectPromptTokens,
		langsmith.RunQueryParamsSelectCompletionTokens,
		langsmith.RunQueryParamsSelectTotalTokens,
		langsmith.RunQueryParamsSelectPromptCost,
		langsmith.RunQueryParamsSelectCompletionCost,
		langsmith.RunQueryParamsSelectTotalCost,
		langsmith.RunQueryParamsSelectTags,
	}
	for _, f := range metadataFields {
		if !has[f] {
			t.Errorf("missing metadata field %q (needed so --include-metadata works alongside --include-io)", f)
		}
	}
}

func TestBuildRunSelect_IncludesFirstTokenTimeAndEvents(t *testing.T) {
	// first_token_time and events are native Run fields that --full is
	// documented to return; a prior version of this select list silently
	// dropped both even when --include-io/--include-metadata were requested.
	has := selectSet(buildRunSelect(true, true))
	if !has[langsmith.RunQueryParamsSelectFirstTokenTime] {
		t.Error("missing first_token_time field")
	}
	if !has[langsmith.RunQueryParamsSelectEvents] {
		t.Error("missing events field (should be requested alongside inputs/outputs/error)")
	}
}

// selectSet converts a slice to a set for easy lookup.
func selectSet(sel []langsmith.RunQueryParamsSelect) map[langsmith.RunQueryParamsSelect]bool {
	m := make(map[langsmith.RunQueryParamsSelect]bool, len(sel))
	for _, f := range sel {
		m[f] = true
	}
	return m
}

// assertBaseFields checks that all fields required by ExtractRun's base output are present.
func assertBaseFields(t *testing.T, has map[langsmith.RunQueryParamsSelect]bool) {
	t.Helper()
	baseFields := []langsmith.RunQueryParamsSelect{
		langsmith.RunQueryParamsSelectID,
		langsmith.RunQueryParamsSelectTraceID,
		langsmith.RunQueryParamsSelectName,
		langsmith.RunQueryParamsSelectRunType,
		langsmith.RunQueryParamsSelectParentRunID,
		langsmith.RunQueryParamsSelectStartTime,
		langsmith.RunQueryParamsSelectEndTime,
	}
	for _, f := range baseFields {
		if !has[f] {
			t.Errorf("missing base field %q", f)
		}
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
	runs := []langsmith.RunSchema{
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

// ---------- buildRunSelectV2 ----------

func TestBuildRunSelectV2_IncludesFirstTokenTimeAndEvents(t *testing.T) {
	// Mirrors TestBuildRunSelect_IncludesFirstTokenTimeAndEvents for the v2
	// select builder: first_token_time is a base field always requested,
	// events is requested alongside inputs/outputs/error under --include-io.
	base := buildRunSelectV2(false, false)
	baseHas := map[langsmith.RunQueryV2ParamsSelect]bool{}
	for _, f := range base {
		baseHas[f] = true
	}
	if !baseHas[langsmith.RunQueryV2ParamsSelectFirstTokenTime] {
		t.Error("missing first_token_time field in base v2 select set")
	}
	if baseHas[langsmith.RunQueryV2ParamsSelectEvents] {
		t.Error("events should not be requested without --include-io")
	}

	withIO := buildRunSelectV2(true, false)
	ioHas := map[langsmith.RunQueryV2ParamsSelect]bool{}
	for _, f := range withIO {
		ioHas[f] = true
	}
	if !ioHas[langsmith.RunQueryV2ParamsSelectEvents] {
		t.Error("missing events field when --include-io requested")
	}
}

// ---------- runV2ToSchema ----------

func TestRunV2ToSchema_MapsFirstTokenTimeAndEvents(t *testing.T) {
	firstToken := time.Date(2026, 1, 15, 10, 30, 1, 0, time.UTC)
	v2 := langsmith.Run{
		ID:             "run-123",
		FirstTokenTime: firstToken,
		Events: []langsmith.RunEvent{
			{Name: "new_token", Kwargs: map[string]interface{}{"token": "hi"}},
		},
	}

	out := runV2ToSchema(v2)

	if !out.FirstTokenTime.Equal(firstToken) {
		t.Errorf("expected FirstTokenTime=%v, got %v", firstToken, out.FirstTokenTime)
	}
	if len(out.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(out.Events))
	}
	if out.Events[0]["name"] != "new_token" {
		t.Errorf("expected event name=new_token, got %v", out.Events[0]["name"])
	}
}

// ---------- toV2Params ----------

func TestToV2Params_TranslatesFields(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	p := langsmith.RunQueryParams{
		Trace:     langsmith.F("trace-1"),
		IsRoot:    langsmith.F(true),
		RunType:   langsmith.F(langsmith.RunTypeEnum("llm")),
		Error:     langsmith.F(true),
		StartTime: langsmith.F(start),
		EndTime:   langsmith.F(end),
		Filter:    langsmith.F(`eq(name, "x")`),
		ID:        langsmith.F([]string{"id-1", "id-2"}),
		Limit:     langsmith.F(int64(25)),
		Order:     langsmith.F(langsmith.RunQueryParamsOrderDesc), // dropped
	}
	sel := []langsmith.RunQueryV2ParamsSelect{langsmith.RunQueryV2ParamsSelectID}

	v2 := toV2Params(p, sel)

	if v2.TraceID.Value != "trace-1" {
		t.Errorf("TraceID = %q, want trace-1", v2.TraceID.Value)
	}
	if !v2.IsRoot.Value {
		t.Error("IsRoot = false, want true")
	}
	if v2.RunType.Value != langsmith.RunQueryV2ParamsRunType("LLM") {
		t.Errorf("RunType = %q, want LLM (uppercased)", v2.RunType.Value)
	}
	if !v2.HasError.Value {
		t.Error("HasError = false, want true")
	}
	if !v2.MinStartTime.Value.Equal(start) {
		t.Errorf("MinStartTime = %v, want %v", v2.MinStartTime.Value, start)
	}
	if !v2.MaxStartTime.Value.Equal(end) {
		t.Errorf("MaxStartTime = %v, want %v", v2.MaxStartTime.Value, end)
	}
	if v2.Filter.Value != `eq(name, "x")` {
		t.Errorf("Filter = %q", v2.Filter.Value)
	}
	if len(v2.IDs.Value) != 2 {
		t.Errorf("IDs = %v, want 2 entries", v2.IDs.Value)
	}
	if v2.PageSize.Value != 25 {
		t.Errorf("PageSize = %d, want 25", v2.PageSize.Value)
	}
	if !v2.Selects.Present {
		t.Error("Selects not set")
	}
}

func TestToV2Params_OmitsUnsetFields(t *testing.T) {
	v2 := toV2Params(langsmith.RunQueryParams{}, nil)
	if v2.TraceID.Present || v2.IsRoot.Present || v2.HasError.Present ||
		v2.MinStartTime.Present || v2.Filter.Present || v2.IDs.Present ||
		v2.PageSize.Present || v2.Selects.Present {
		t.Error("expected all fields unset for empty input")
	}
}
