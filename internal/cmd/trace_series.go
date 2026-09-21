package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// `trace stats` returns one aggregate per window, so the only way to see a
// shape over time was two windows via --compare-since. This bucketed form
// comes from the charts API, which is the only place a stride exists.
//
// The langsmith-go SDK has no charts service yet, so this speaks to the
// endpoint through the raw client, as `thread` does for its own uncovered
// route.
const chartPreviewPath = "/api/v1/charts/preview"

// chartMetrics is for the help text and a friendly error; the server remains
// the authority, so an unknown value is still passed through rather than
// rejected here.
var chartMetrics = []string{
	"run_count", "error_rate", "streaming_rate",
	"latency_p50", "latency_p99", "latency_avg",
	"first_token_p50", "first_token_p99",
	"total_tokens", "prompt_tokens", "completion_tokens", "median_tokens",
	"prompt_tokens_p50", "completion_tokens_p50",
	"tokens_p99", "prompt_tokens_p99", "completion_tokens_p99",
	"total_cost", "prompt_cost", "completion_cost", "cost_p50", "cost_p99",
	"feedback_score_avg",
}

type chartTimedelta struct {
	Days    int `json:"days,omitempty"`
	Hours   int `json:"hours,omitempty"`
	Minutes int `json:"minutes,omitempty"`
}

type chartBucketInfo struct {
	StartTime string         `json:"start_time"`
	EndTime   string         `json:"end_time"`
	Stride    chartTimedelta `json:"stride"`
	Timezone  string         `json:"timezone"`
}

type chartSeriesFilters struct {
	Filter      string   `json:"filter,omitempty"`
	TraceFilter string   `json:"trace_filter,omitempty"`
	TreeFilter  string   `json:"tree_filter,omitempty"`
	Session     []string `json:"session"`
}

type chartGroupBy struct {
	Attribute string `json:"attribute"`
	Path      string `json:"path,omitempty"`
	MaxGroups int    `json:"max_groups,omitempty"`
}

type chartSeries struct {
	// The API requires an id on a preview series even though nothing is
	// created; any stable string satisfies it.
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Metric      string             `json:"metric"`
	FeedbackKey string             `json:"feedback_key,omitempty"`
	Filters     chartSeriesFilters `json:"filters"`
	GroupBy     *chartGroupBy      `json:"group_by,omitempty"`
}

type chartPreviewRequest struct {
	BucketInfo chartBucketInfo `json:"bucket_info"`
	Chart      struct {
		Series []chartSeries `json:"series"`
	} `json:"chart"`
}

// bucketTime tolerates a timestamp without a zone: the charts API returns
// bucket boundaries as naive local-to-UTC strings, so RFC3339 alone fails to
// decode the response.
type bucketTime struct{ time.Time }

func (b *bucketTime) UnmarshalJSON(raw []byte) error {
	s := strings.Trim(string(raw), `"`)
	if s == "" || s == "null" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339Nano, "2006-01-02T15:04:05.999999", "2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			b.Time = t.UTC()
			return nil
		}
	}
	return fmt.Errorf("unrecognised bucket timestamp %q", s)
}

type chartDataPoint struct {
	SeriesID  string     `json:"series_id"`
	Timestamp bucketTime `json:"timestamp"`
	Value     any        `json:"value"`
	Group     string     `json:"group"`
}

type chartPreviewResponse struct {
	Data []chartDataPoint `json:"data"`
}

func newTraceSeriesCmd() *cobra.Command {
	var (
		project     string
		projectID   string
		since       string
		before      string
		lastNMin    int
		stride      string
		metric      string
		feedbackKey string
		filter      string
		treeFilter  string
		groupBy     string
		groupPath   string
		maxGroups   int
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "series",
		Short: "Bucketed timeseries for one metric over a project",
		Long: `Chart one metric over time for a project, bucketed by --stride.

Where ` + "`trace stats`" + ` gives a single aggregate for a window, this returns a
value per bucket, so you can see when something changed rather than only that
it differs. Filters use the same LangSmith DSL, and --group-by splits the
series by tool, run name, tag, or a metadata path.

Metrics:
  ` + strings.Join(chartMetrics, ", ") + `

Examples:
  langsmith trace series --project my-app --metric error_rate --stride 1h
  langsmith trace series --project my-app --metric latency_p99 --stride 15m --last-n-minutes 720
  langsmith trace series --project my-app --metric error_rate --stride 30m \
    --filter 'eq(run_type, "tool")' --group-by name
  langsmith trace series --project my-app --metric feedback_score_avg \
    --feedback-key correctness --stride 1d --since 2026-01-01`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustGetClient()
			ctx := context.Background()

			sessionID, err := resolveSessionID(ctx, c, project, projectID, "trace series")
			if err != nil {
				return err
			}
			delta, err := parseStride(stride)
			if err != nil {
				return err
			}
			start := resolveStartTime(since, lastNMin)
			end := time.Now().UTC()
			if before != "" {
				if end, err = parseFlexTime(before); err != nil {
					return fmt.Errorf("invalid --before timestamp: %s", before)
				}
			}
			if !end.After(start) {
				return fmt.Errorf("--before must be after --since")
			}

			req := chartPreviewRequest{BucketInfo: chartBucketInfo{
				StartTime: start.UTC().Format(time.RFC3339),
				EndTime:   end.UTC().Format(time.RFC3339),
				Stride:    delta,
				Timezone:  "UTC",
			}}
			series := chartSeries{
				ID:          "series-1",
				Name:        metric,
				Metric:      metric,
				FeedbackKey: feedbackKey,
				Filters: chartSeriesFilters{
					Filter:     filter,
					TreeFilter: treeFilter,
					Session:    []string{sessionID},
				},
			}
			if groupBy != "" {
				series.GroupBy = &chartGroupBy{
					Attribute: groupBy, Path: groupPath, MaxGroups: maxGroups,
				}
			}
			req.Chart.Series = []chartSeries{series}

			points, err := fetchChartPreview(ctx, c, req)
			if err != nil {
				return err
			}
			if GetFormat() == "pretty" {
				printSeriesPretty(os.Stdout, points, metric, stride)
				return nil
			}
			if err := output.OutputJSON(
				map[string]any{"data": points}, outputFile,
			); err != nil {
				ExitErrorf("%v", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project (session) UUID; skips the name lookup. Takes precedence over --project / $LANGSMITH_PROJECT")
	cmd.Flags().StringVar(&metric, "metric", "run_count", "Metric to chart (see the list above)")
	cmd.Flags().StringVar(&stride, "stride", "1h", "Bucket width: 30m, 2h, 1d")
	cmd.Flags().StringVar(&since, "since", "", "Start of time window (RFC3339 or YYYY-MM-DD; default: 7 days ago)")
	cmd.Flags().StringVar(&before, "before", "", "End of time window (RFC3339 or YYYY-MM-DD; default: now)")
	cmd.Flags().IntVar(&lastNMin, "last-n-minutes", 0, "Shorthand: window = last N minutes (overrides --since)")
	cmd.Flags().StringVar(&filter, "filter", "", "LangSmith filter DSL applied to each run")
	cmd.Flags().StringVar(&treeFilter, "tree-filter", "", "LangSmith filter DSL matching any run in the trace tree")
	cmd.Flags().StringVar(&feedbackKey, "feedback-key", "", "Feedback key, required by the feedback metrics")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "Split the series by: name, run_type, tag, metadata")
	cmd.Flags().StringVar(&groupPath, "group-path", "", "Metadata key, when --group-by is metadata")
	cmd.Flags().IntVar(&maxGroups, "max-groups", 5, "Maximum groups to return when grouping (1-20)")
	cmd.Flags().StringVar(&outputFile, "output", "", "Write JSON output to file instead of stdout")
	cmd.MarkFlagsMutuallyExclusive("project", "project-id")
	return cmd
}

// parseStride accepts the compact forms the flag help advertises. The API
// takes days/hours/minutes rather than a duration string, and rejects a
// stride under one minute.
func parseStride(s string) (chartTimedelta, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return chartTimedelta{}, fmt.Errorf("--stride is required")
	}
	unit := s[len(s)-1]
	var n int
	if _, err := fmt.Sscanf(s[:len(s)-1], "%d", &n); err != nil || n <= 0 {
		return chartTimedelta{}, fmt.Errorf(
			"invalid --stride %q: use a positive number with m, h, or d (30m, 2h, 1d)", s,
		)
	}
	switch unit {
	case 'm':
		return chartTimedelta{Minutes: n}, nil
	case 'h':
		return chartTimedelta{Hours: n}, nil
	case 'd':
		return chartTimedelta{Days: n}, nil
	default:
		return chartTimedelta{}, fmt.Errorf(
			"invalid --stride %q: use m, h, or d (30m, 2h, 1d)", s,
		)
	}
}

func fetchChartPreview(
	ctx context.Context, c *client.Client, req chartPreviewRequest,
) ([]chartDataPoint, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	status, _, _, body, err := c.RawDo(
		ctx, http.MethodPost, chartPreviewPath, bytes.NewReader(raw), nil,
	)
	if err != nil {
		return nil, fmt.Errorf("fetching series: %w", err)
	}
	if status >= 400 {
		return nil, fmt.Errorf("fetching series: HTTP %d: %s", status, previewError(body))
	}
	var out chartPreviewResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decoding series: %w", err)
	}
	return out.Data, nil
}

func previewError(body []byte) string {
	var env struct {
		Detail any `json:"detail"`
	}
	if json.Unmarshal(body, &env) == nil && env.Detail != nil {
		return fmt.Sprintf("%v", env.Detail)
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return msg
}

// printSeriesPretty states the bucket grid once and gives one row per group,
// so a long series stays readable and diffable instead of repeating a
// timestamp on every point.
func printSeriesPretty(w io.Writer, points []chartDataPoint, metric, stride string) {
	if len(points) == 0 {
		fmt.Fprintln(w, "no data in this window")
		return
	}
	stamps := map[time.Time]bool{}
	byGroup := map[string]map[time.Time]any{}
	for _, p := range points {
		stamps[p.Timestamp.Time] = true
		name := p.Group
		if name == "" {
			name = metric
		}
		if byGroup[name] == nil {
			byGroup[name] = map[time.Time]any{}
		}
		byGroup[name][p.Timestamp.Time] = p.Value
	}
	grid := make([]time.Time, 0, len(stamps))
	for t := range stamps {
		grid = append(grid, t)
	}
	sort.Slice(grid, func(i, j int) bool { return grid[i].Before(grid[j]) })

	fmt.Fprintf(w, "# metric=%s stride=%s buckets=%d t0=%s  (\".\" = no data)\n",
		metric, stride, len(grid), grid[0].UTC().Format(time.RFC3339))

	names := make([]string, 0, len(byGroup))
	for name := range byGroup {
		names = append(names, name)
	}
	sort.Strings(names)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, name := range names {
		cells := make([]string, 0, len(grid))
		for _, t := range grid {
			cells = append(cells, formatSeriesValue(byGroup[name][t]))
		}
		fmt.Fprintf(tw, "%s\t%s\n", name, strings.Join(cells, " "))
	}
	_ = tw.Flush()
}

func formatSeriesValue(v any) string {
	switch n := v.(type) {
	case nil:
		return "."
	case float64:
		if n == float64(int64(n)) {
			return fmt.Sprintf("%d", int64(n))
		}
		if n < 1 && n > -1 {
			return strings.TrimPrefix(fmt.Sprintf("%.3g", n), "0")
		}
		return fmt.Sprintf("%.4g", n)
	case map[string]any:
		// Feedback metrics report an object per bucket rather than a scalar.
		if avg, ok := n["avg"].(float64); ok {
			return formatSeriesValue(avg)
		}
		return "?"
	default:
		return fmt.Sprintf("%v", v)
	}
}
