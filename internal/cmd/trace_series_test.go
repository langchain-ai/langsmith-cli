package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
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

// The charts API returns bucket boundaries without a zone, so decoding only
// RFC3339 failed every response.
func TestBucketTimeAcceptsAZonelessTimestamp(t *testing.T) {
	for _, raw := range []string{
		`"2026-09-21T10:31:55"`,
		`"2026-09-21T10:31:55Z"`,
		`"2026-09-21T10:31:55.123456"`,
	} {
		var b bucketTime
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			t.Fatalf("unmarshal %s: %v", raw, err)
		}
		if b.Year() != 2026 || b.Location() != time.UTC {
			t.Errorf("unmarshal %s gave %v", raw, b.Time)
		}
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
		{Group: "tool-a", Timestamp: bucketTime{t0}, Value: 1.0},
		{Group: "tool-a", Timestamp: bucketTime{t0.Add(time.Hour)}, Value: 2.0},
		{Group: "tool-b", Timestamp: bucketTime{t0}, Value: 3.0},
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
		&buf, []chartDataPoint{{Timestamp: bucketTime{t0}, Value: 7.0}},
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
