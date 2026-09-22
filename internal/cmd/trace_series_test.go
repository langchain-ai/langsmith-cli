package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestTraceSeries_RegisteredUnderTrace(t *testing.T) {
	var found bool
	for _, sub := range newTraceCmd().Commands() {
		if sub.Name() == "series" {
			found = true
		}
	}
	if !found {
		t.Fatal("trace missing subcommand \"series\"")
	}
}

func TestParseStride(t *testing.T) {
	cases := []struct {
		in   string
		want chartTimedelta
	}{
		{"30m", chartTimedelta{Minutes: 30}},
		{"2h", chartTimedelta{Hours: 2}},
		{"1d", chartTimedelta{Days: 1}},
		{" 15M ", chartTimedelta{Minutes: 15}},
	}
	for _, tc := range cases {
		got, err := parseStride(tc.in)
		if err != nil {
			t.Fatalf("parseStride(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("parseStride(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestParseStrideRejectsJunk(t *testing.T) {
	// The API takes days/hours/minutes, not a duration string, so a bad unit
	// has to fail here rather than as an opaque 422.
	for _, in := range []string{"", "1w", "h", "0h", "-2h", "abc", "90"} {
		if _, err := parseStride(in); err == nil {
			t.Errorf("parseStride(%q) should have failed", in)
		}
	}
}

// The charts API returns bucket boundaries without a zone, and feedback
// metrics report an object where others report a number. Both have to
// survive the SDK's decoder and the flattening after it.
func TestPreviewResponseDecodesZonelessTimesAndBothValueShapes(t *testing.T) {
	raw := `{"data":[
		{"series_id":"s","timestamp":"2026-09-21T10:31:55","value":0.25,"group":"tool-a"},
		{"series_id":"s","timestamp":"2026-09-21T11:31:55.123456","value":{"avg":0.75,"n":4},"group":""},
		{"series_id":"s","timestamp":"2026-09-21T12:31:55Z","value":null,"group":""}
	]}`
	var res langsmith.ChartPreviewResponse
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(res.Data) != 3 {
		t.Fatalf("got %d points, want 3", len(res.Data))
	}
	want := time.Date(2026, 9, 21, 10, 31, 55, 0, time.UTC)
	if got := res.Data[0].Timestamp.UTC(); !got.Equal(want) {
		t.Errorf("zoneless timestamp decoded as %v, want %v", got, want)
	}
	if got := seriesValue(res.Data[0].Value); got != 0.25 {
		t.Errorf("numeric value flattened to %#v", got)
	}
	if got := formatSeriesValue(seriesValue(res.Data[1].Value)); got != ".75" {
		t.Errorf("feedback object rendered as %q, want .75", got)
	}
	if got := seriesValue(res.Data[2].Value); got != nil {
		t.Errorf("null value flattened to %#v, want nil", got)
	}
}

// Unset flags must be left out of the request: the server parses an empty
// filter string as a filter.
func TestPreviewParamsOmitsUnsetOptionalFields(t *testing.T) {
	q := seriesQuery{
		sessionID: "3fa85f64-5717-4562-b3fc-2c963f66afa6",
		start:     time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
		end:       time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
		stride:    chartTimedelta{Minutes: 30},
		metric:    "error_rate",
	}
	body, err := json.Marshal(previewParams(q))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, absent := range []string{`"filter"`, `"tree_filter"`, `"group_by"`, `"feedback_key"`, `"hours"`} {
		if strings.Contains(s, absent) {
			t.Errorf("unset field %s was sent: %s", absent, s)
		}
	}
	for _, present := range []string{`"minutes":30`, `"metric":"error_rate"`, q.sessionID} {
		if !strings.Contains(s, present) {
			t.Errorf("missing %s in %s", present, s)
		}
	}

	q.filter, q.groupBy, q.maxGroups = `eq(run_type, "tool")`, "name", 5
	body, _ = json.Marshal(previewParams(q))
	s = string(body)
	if !strings.Contains(s, `"group_by":{"attribute":"name","max_groups":5}`) {
		t.Errorf("group_by not rendered as expected: %s", s)
	}
	if !strings.Contains(s, `"filter":"eq(run_type, \"tool\")"`) {
		t.Errorf("filter not rendered: %s", s)
	}
}

func TestFormatSeriesValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, "."},
		{float64(0), "0"},
		{float64(42), "42"},
		{0.00222, ".00222"},
		{5.4213, "5.421"},
		// Feedback metrics report an object per bucket rather than a scalar.
		{map[string]any{"avg": 0.75, "n": float64(4)}, ".75"},
		{map[string]any{"n": float64(4)}, "?"},
	}
	for _, tc := range cases {
		if got := formatSeriesValue(tc.in); got != tc.want {
			t.Errorf("formatSeriesValue(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrintSeriesPrettyStatesTheGridOnce(t *testing.T) {
	t0 := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	points := []chartDataPoint{
		{Group: "tool-a", Timestamp: t0, Value: 1.0},
		{Group: "tool-a", Timestamp: t0.Add(time.Hour), Value: 2.0},
		{Group: "tool-b", Timestamp: t0, Value: 3.0},
		// tool-b has no second bucket, which must render as a gap not a shift.
	}
	var buf bytes.Buffer
	printSeriesPretty(&buf, points, "run_count", "1h")
	out := buf.String()

	if strings.Count(out, "2026-09-21T10:00:00Z") != 1 {
		t.Errorf("grid should appear once in the header, got:\n%s", out)
	}
	if !strings.Contains(out, "buckets=2") {
		t.Errorf("missing bucket count:\n%s", out)
	}
	if !strings.Contains(out, "1 2") {
		t.Errorf("tool-a should hold both buckets:\n%s", out)
	}
	if !strings.Contains(out, "3 .") {
		t.Errorf("tool-b's missing bucket should render as a gap:\n%s", out)
	}
}

func TestPrintSeriesPrettyUsesTheMetricNameWhenUngrouped(t *testing.T) {
	t0 := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	printSeriesPretty(
		&buf, []chartDataPoint{{Timestamp: t0, Value: 7.0}},
		"error_rate", "1h",
	)
	out := buf.String()
	if !strings.Contains(out, "error_rate") {
		t.Errorf("ungrouped row should be named for the metric:\n%s", out)
	}
}

func TestPrintSeriesPrettyOnAnEmptyWindow(t *testing.T) {
	var buf bytes.Buffer
	printSeriesPretty(&buf, nil, "run_count", "1h")
	out := buf.String()
	if !strings.Contains(out, "no data") {
		t.Errorf("expected an empty-window message, got:\n%s", out)
	}
}

func TestPreviewErrorPrefersTheServerDetail(t *testing.T) {
	got := previewError([]byte(`{"detail":["body.chart.series.0.id: Field required"]}`))
	if !strings.Contains(got, "Field required") {
		t.Errorf("detail not surfaced: %q", got)
	}
	if got := previewError([]byte("upstream exploded")); got != "upstream exploded" {
		t.Errorf("non-JSON body should pass through, got %q", got)
	}
}
