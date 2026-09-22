package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/langchain-ai/langsmith-go/shared"
	"github.com/spf13/cobra"
)

func newChartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chart",
		Short: "Compute charts over runs without saving them",
	}
	cmd.AddCommand(newChartPreviewCmd())
	return cmd
}

func newChartPreviewCmd() *cobra.Command {
	var body, outputFile string

	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Compute a chart's bucketed data without creating the chart",
		Long: `Send a chart definition to the charts preview API and print the data it
computes. Nothing is saved.

--body is the request body, inline or as @file. It takes the API's own shape:
a bucket_info with the window and stride, and a chart with one or more series.
Each series picks a metric and carries the same filter DSL the other commands
use, and can split by name, run_type, tag, or a metadata path.

Pretty output states the bucket grid once and prints one row per series and
group, with "." for a bucket that has no data. --format json prints the API
response unchanged.

Example:
  langsmith chart preview --body '{
    "bucket_info": {
      "start_time": "2026-01-01T00:00:00Z",
      "end_time": "2026-01-02T00:00:00Z",
      "stride": {"hours": 1}
    },
    "chart": {"series": [{
      "id": "0b3a86e5-4e04-4d4b-9b2c-0f6f2b1c3d4e",
      "name": "tool errors",
      "metric": "error_rate",
      "filters": {
        "session": ["<project-uuid>"],
        "filter": "eq(run_type, \"tool\")"
      },
      "group_by": {"attribute": "name", "max_groups": 5}
    }]}
  }'

  langsmith chart preview --body @chart.json --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := loadJSONArg(body)
			if err != nil {
				return fmt.Errorf("--body: %w", err)
			}
			c := MustGetClient()
			res, err := chartPreview(context.Background(), c.SDK, raw)
			if err != nil {
				return err
			}
			if GetFormat() == "pretty" {
				printChartPretty(os.Stdout, raw, res)
				return nil
			}
			return output.OutputJSON(json.RawMessage(res.JSON.RawJSON()), outputFile)
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "Request body as JSON, or @path to a JSON file (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

// chartPreview sends the caller's body as-is: the SDK's param types cannot be
// populated from JSON, so the body replaces the empty params on the wire.
func chartPreview(
	ctx context.Context, sdk *langsmith.Client, body []byte,
) (*langsmith.ChartPreviewResponse, error) {
	res, err := sdk.Charts.Preview(
		ctx, langsmith.ChartPreviewParams{},
		option.WithRequestBody("application/json", body),
	)
	if err != nil {
		var apiErr *langsmith.Error
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("chart preview: HTTP %d: %s",
				apiErr.StatusCode, apiErrorDetail([]byte(apiErr.JSON.RawJSON())))
		}
		return nil, fmt.Errorf("chart preview: %w", err)
	}
	return res, nil
}

// apiErrorDetail prefers the server's validation detail, which names the
// offending field, over the whole error body.
func apiErrorDetail(body []byte) string {
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

// seriesNames maps each series id in the request to its name, so pretty rows
// read as the caller named them rather than as ids.
func seriesNames(body []byte) map[string]string {
	var req struct {
		Chart struct {
			Series []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"series"`
		} `json:"chart"`
	}
	names := map[string]string{}
	if json.Unmarshal(body, &req) != nil {
		return names
	}
	for _, s := range req.Chart.Series {
		if s.Name != "" {
			names[s.ID] = s.Name
		}
	}
	return names
}

// printChartPretty states the bucket grid once and gives one row per series
// and group, so a long series stays readable instead of repeating a
// timestamp on every point.
func printChartPretty(w io.Writer, body []byte, res *langsmith.ChartPreviewResponse) {
	if len(res.Data) == 0 {
		fmt.Fprintln(w, "no data in this window")
		return
	}
	names := seriesNames(body)
	stamps := map[time.Time]bool{}
	rows := map[string]map[time.Time]any{}
	for _, d := range res.Data {
		t := d.Timestamp.UTC()
		stamps[t] = true
		label := names[d.SeriesID]
		if label == "" {
			label = d.SeriesID
		}
		if d.Group != "" {
			label += "[" + d.Group + "]"
		}
		if rows[label] == nil {
			rows[label] = map[time.Time]any{}
		}
		rows[label][t] = chartValue(d.Value)
	}
	grid := make([]time.Time, 0, len(stamps))
	for t := range stamps {
		grid = append(grid, t)
	}
	sort.Slice(grid, func(i, j int) bool { return grid[i].Before(grid[j]) })

	fmt.Fprintf(w, "# buckets=%d t0=%s stride=%s  (\".\" = no data)\n",
		len(grid), grid[0].Format(time.RFC3339), bucketStride(grid))

	labels := make([]string, 0, len(rows))
	for l := range rows {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, l := range labels {
		cells := make([]string, 0, len(grid))
		for _, t := range grid {
			cells = append(cells, formatChartValue(rows[l][t]))
		}
		fmt.Fprintf(tw, "%s\t%s\n", l, strings.Join(cells, " "))
	}
	_ = tw.Flush()
}

// bucketStride is the most common gap between buckets. The API aligns buckets
// to the stride, so the first and last are usually partial and their gaps
// understate it.
func bucketStride(grid []time.Time) string {
	if len(grid) < 2 {
		return "-"
	}
	counts := map[time.Duration]int{}
	var best time.Duration
	for i := 1; i < len(grid); i++ {
		gap := grid[i].Sub(grid[i-1])
		counts[gap]++
		if counts[gap] > counts[best] || (counts[gap] == counts[best] && gap > best) {
			best = gap
		}
	}
	return best.String()
}

// chartValue flattens the SDK's number-or-object union: most metrics report
// a number per bucket, feedback metrics an object.
func chartValue(v langsmith.ChartPreviewResponseDataValueUnion) any {
	switch n := v.(type) {
	case nil:
		return nil
	case shared.UnionFloat:
		return float64(n)
	case langsmith.ChartPreviewResponseDataValueMap:
		return map[string]any(n)
	default:
		return v
	}
}

func formatChartValue(v any) string {
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
			return formatChartValue(avg)
		}
		return "?"
	default:
		return fmt.Sprintf("%v", v)
	}
}
