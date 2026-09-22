package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
)

func TestChartPreview_Registered(t *testing.T) {
	cmd, _, err := newChartCmd().Find([]string{"preview"})
	if err != nil || cmd.Name() != "preview" {
		t.Fatalf("chart missing subcommand \"preview\": %v", err)
	}
}

func newChartTestServer(t *testing.T, status int, resp string, gotBody *[]byte) *langsmith.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/charts/preview" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		*gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, resp)
	}))
	t.Cleanup(srv.Close)
	return langsmith.NewClient(
		option.WithBaseURL(srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0),
	)
}

// The command is a pass-through: whatever the caller wrote is what the API
// receives, including fields the CLI knows nothing about.
func TestChartPreviewSendsTheBodyUnchanged(t *testing.T) {
	var got []byte
	sdk := newChartTestServer(t, http.StatusOK, `{"data":[]}`, &got)
	body := []byte(`{"bucket_info":{"stride":{"hours":1}},"chart":{"series":[{"id":"x","name":"n","metric":"run_count","future_field":true}]}}`)

	if _, err := chartPreview(context.Background(), sdk, body); err != nil {
		t.Fatal(err)
	}
	var want, sent any
	_ = json.Unmarshal(body, &want)
	if err := json.Unmarshal(got, &sent); err != nil {
		t.Fatalf("server got invalid JSON %q: %v", got, err)
	}
	wantJSON, _ := json.Marshal(want)
	sentJSON, _ := json.Marshal(sent)
	if !bytes.Equal(wantJSON, sentJSON) {
		t.Errorf("body changed in transit:\n sent %s\n want %s", sentJSON, wantJSON)
	}
}

func TestChartPreviewSurfacesTheServerDetail(t *testing.T) {
	var got []byte
	sdk := newChartTestServer(t, http.StatusUnprocessableEntity,
		`{"detail":["body.chart.series.0.id: Field required"]}`, &got)
	_, err := chartPreview(context.Background(), sdk, []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "HTTP 422") ||
		!strings.Contains(err.Error(), "Field required") {
		t.Errorf("want the 422 detail surfaced, got %v", err)
	}
}

func TestAPIErrorDetailFallsBackToTheBody(t *testing.T) {
	if got := apiErrorDetail([]byte("upstream exploded")); got != "upstream exploded" {
		t.Errorf("non-JSON body should pass through, got %q", got)
	}
}

func decodePreview(t *testing.T, raw string) *langsmith.ChartPreviewResponse {
	t.Helper()
	var res langsmith.ChartPreviewResponse
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &res
}

// The charts API returns bucket boundaries without a zone, and feedback
// metrics report an object where others report a number.
func TestPreviewResponseDecodesZonelessTimesAndBothValueShapes(t *testing.T) {
	res := decodePreview(t, `{"data":[
		{"series_id":"s","timestamp":"2026-09-21T10:31:55","value":0.25,"group":""},
		{"series_id":"s","timestamp":"2026-09-21T11:31:55.123456","value":{"avg":0.75,"n":4},"group":""},
		{"series_id":"s","timestamp":"2026-09-21T12:31:55Z","value":null,"group":""}
	]}`)
	want := time.Date(2026, 9, 21, 10, 31, 55, 0, time.UTC)
	if got := res.Data[0].Timestamp.UTC(); !got.Equal(want) {
		t.Errorf("zoneless timestamp decoded as %v, want %v", got, want)
	}
	if got := chartValue(res.Data[0].Value); got != 0.25 {
		t.Errorf("numeric value flattened to %#v", got)
	}
	if got := formatChartValue(chartValue(res.Data[1].Value)); got != ".75" {
		t.Errorf("feedback object rendered as %q, want .75", got)
	}
	if got := chartValue(res.Data[2].Value); got != nil {
		t.Errorf("null value flattened to %#v, want nil", got)
	}
}

func TestFormatChartValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, "."},
		{float64(0), "0"},
		{float64(42), "42"},
		{0.00222, ".00222"},
		{5.4213, "5.421"},
		{map[string]any{"avg": 0.75, "n": float64(4)}, ".75"},
		{map[string]any{"n": float64(4)}, "?"},
	}
	for _, tc := range cases {
		if got := formatChartValue(tc.in); got != tc.want {
			t.Errorf("formatChartValue(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrintChartPrettyStatesTheGridOnceAndNamesRowsFromTheRequest(t *testing.T) {
	body := []byte(`{"chart":{"series":[{"id":"s1","name":"errors"},{"id":"s2","name":""}]}}`)
	res := decodePreview(t, `{"data":[
		{"series_id":"s1","timestamp":"2026-09-21T10:00:00Z","value":1,"group":"tool-a"},
		{"series_id":"s1","timestamp":"2026-09-21T11:00:00Z","value":2,"group":"tool-a"},
		{"series_id":"s1","timestamp":"2026-09-21T10:00:00Z","value":3,"group":"tool-b"},
		{"series_id":"s2","timestamp":"2026-09-21T11:00:00Z","value":4,"group":""}
	]}`)
	var buf bytes.Buffer
	printChartPretty(&buf, body, res)
	out := buf.String()

	if strings.Count(out, "2026-09-21T10:00:00Z") != 1 {
		t.Errorf("grid should appear once in the header, got:\n%s", out)
	}
	if !strings.Contains(out, "buckets=2") || !strings.Contains(out, "stride=1h0m0s") {
		t.Errorf("header should give bucket count and stride:\n%s", out)
	}
	for _, want := range []string{"errors[tool-a]", "errors[tool-b]", "1 2", "3 ."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	// A series with no name falls back to its id, and its missing first
	// bucket renders as a gap rather than shifting the row.
	if !strings.Contains(out, "s2") || !strings.Contains(out, ". 4") {
		t.Errorf("unnamed series should fall back to its id with a gap:\n%s", out)
	}
}

// Buckets are aligned to the stride, so a window starting mid-bucket opens
// with a short one: 07:00, 08:00, then every 2h. The first gap is not the
// stride.
func TestBucketStrideIgnoresPartialEdgeBuckets(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 9, 22, h, 0, 0, 0, time.UTC) }
	grid := []time.Time{at(7), at(8), at(10), at(12), at(14), at(16), at(18)}
	if got := bucketStride(grid); got != "2h0m0s" {
		t.Errorf("bucketStride = %s, want 2h0m0s", got)
	}
	if got := bucketStride(grid[:1]); got != "-" {
		t.Errorf("single bucket should have no stride, got %s", got)
	}
}

func TestPrintChartPrettyOnAnEmptyWindow(t *testing.T) {
	var buf bytes.Buffer
	printChartPretty(&buf, nil, decodePreview(t, `{"data":[]}`))
	if !strings.Contains(buf.String(), "no data") {
		t.Errorf("expected an empty-window message, got:\n%s", buf.String())
	}
}
