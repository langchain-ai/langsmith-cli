package cmd

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

// runStats is the internal representation used for display.
type runStats struct {
	RunCount         int64          `json:"run_count"`
	LatencyP50       float64        `json:"latency_p50"`
	LatencyP99       float64        `json:"latency_p99"`
	TotalTokens      int64          `json:"total_tokens"`
	PromptTokens     int64          `json:"prompt_tokens"`
	CompletionTokens int64          `json:"completion_tokens"`
	TotalCost        any            `json:"total_cost"`
	ErrorRate        float64        `json:"error_rate"`
	FeedbackStats    map[string]any `json:"feedback_stats"`
}

func newTraceStatsCmd() *cobra.Command {
	var (
		project     string
		projectID   string
		since       string
		before      string
		lastNMin    int
		cmpSince    string
		cmpBefore   string
		cmpLastNMin int
		filter      string
		version     string
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Aggregate stats for traces in a project (token usage, latency, costs, feedback)",
		Long: `Fetch aggregate stats for root traces in a project.

Returns run count, latency percentiles, token usage, costs, error rate, and
the top feedback keys with their score distributions. Useful for spotting
trends, discovering available feedback keys, and understanding score ranges
before building evaluators.

Optionally pass --compare-since/--compare-before (or --compare-last-n-minutes)
to fetch a second time window side-by-side for trend comparison.

Examples:
  langsmith trace stats --project my-app
  langsmith trace stats --project my-app --last-n-minutes 120
  langsmith trace stats --project my-app --since 2025-01-10 --compare-since 2025-01-03 --compare-before 2025-01-10
  langsmith trace stats --project my-app --filter 'eq(status, "error")'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustGetClient()
			ctx := context.Background()

			sessionID, err := resolveSessionID(ctx, c, project, projectID, "trace stats")
			if err != nil {
				return err
			}

			primary, err := fetchRunStats(ctx, c, sessionID, since, before, lastNMin, filter)
			if err != nil {
				return fmt.Errorf("fetching stats: %w", err)
			}

			hasCompare := cmpSince != "" || cmpBefore != "" || cmpLastNMin > 0
			var compare *runStats
			if hasCompare {
				s, err := fetchRunStats(ctx, c, sessionID, cmpSince, cmpBefore, cmpLastNMin, filter)
				if err != nil {
					return fmt.Errorf("fetching comparison stats: %w", err)
				}
				compare = &s
			}

			fmt_ := GetFormat()
			if fmt_ == "pretty" {
				printStatsPretty(&primary, compare, hasCompare)
			} else {
				result := map[string]any{"stats": &primary}
				if hasCompare {
					result["compare"] = compare
				}
				output.OutputJSON(result, outputFile)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project (session) UUID; skips the name lookup. Takes precedence over --project / $LANGSMITH_PROJECT")
	cmd.Flags().StringVar(&since, "since", "", "Start of time window (RFC3339 or YYYY-MM-DD; default: 7 days ago)")
	cmd.Flags().StringVar(&before, "before", "", "End of time window (RFC3339 or YYYY-MM-DD; default: now)")
	cmd.Flags().IntVar(&lastNMin, "last-n-minutes", 0, "Shorthand: window = last N minutes (overrides --since)")
	cmd.Flags().StringVar(&cmpSince, "compare-since", "", "Comparison window start (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&cmpBefore, "compare-before", "", "Comparison window end (RFC3339 or YYYY-MM-DD; default: same as --since)")
	cmd.Flags().IntVar(&cmpLastNMin, "compare-last-n-minutes", 0, "Shorthand: comparison window = N minutes before the primary window starts")
	cmd.Flags().StringVar(&filter, "filter", "", "LangSmith filter DSL (applied to both windows if comparing)")
	// --version is accepted for interface consistency but is a no-op here: SDK
	// v0.18.2 exposes no v2 stats method (only Runs.Stats). The stats endpoint
	// routes to SmithDB server-side, so v1/v2 selection is not a client concern.
	cmd.Flags().StringVar(&version, "version", "", `Query API version: "" (v1, default) or "v2" (SmithDB)`)
	cmd.Flags().StringVar(&outputFile, "output", "", "Write JSON output to file instead of stdout")
	cmd.MarkFlagsMutuallyExclusive("project", "project-id")
	return cmd
}

// fetchRunStats calls the SDK Runs.Stats endpoint and maps the result to runStats.
func fetchRunStats(ctx context.Context, c *client.Client, sessionID, since, before string, lastNMin int, filter string) (runStats, error) {
	params := langsmith.RunStatsParams{
		RunStatsQueryParams: langsmith.RunStatsQueryParams{
			Session:   langsmith.F([]string{sessionID}),
			IsRoot:    langsmith.F(true),
			StartTime: langsmith.F(resolveStartTime(since, lastNMin)),
			Select: langsmith.F([]langsmith.RunStatsQueryParamsSelect{
				langsmith.RunStatsQueryParamsSelectRunCount,
				langsmith.RunStatsQueryParamsSelectLatencyP50,
				langsmith.RunStatsQueryParamsSelectLatencyP99,
				langsmith.RunStatsQueryParamsSelectTotalTokens,
				langsmith.RunStatsQueryParamsSelectPromptTokens,
				langsmith.RunStatsQueryParamsSelectCompletionTokens,
				// total_cost is intentionally excluded: the API returns it as a JSON number
				// (e.g. 8.2e-6) but the SDK models it as string. The type mismatch causes the
				// union discriminator to pick RunStatsResponseMap instead of RunStatsResponseRunStats,
				// yielding all-zero results. Excluding it keeps the flat-object response decodable.
				langsmith.RunStatsQueryParamsSelectErrorRate,
				langsmith.RunStatsQueryParamsSelectFeedbackStats,
			}),
		},
	}
	if before != "" {
		if t, err := parseFlexTime(before); err == nil {
			params.RunStatsQueryParams.EndTime = langsmith.F(t)
		}
	}
	if filter != "" {
		params.RunStatsQueryParams.Filter = langsmith.F(filter)
	}

	res, err := c.SDK.Runs.Stats(ctx, params)
	if err != nil {
		return runStats{}, err
	}

	switch s := (*res).(type) {
	case langsmith.RunStatsResponseRunStats:
		return toRunStats(s.RunCount, s.LatencyP50, s.LatencyP99, s.TotalTokens, s.PromptTokens, s.CompletionTokens, s.TotalCost, s.ErrorRate, s.FeedbackStats), nil
	case langsmith.RunStatsResponseMap:
		// group_by response — extract the first (and only) entry when no grouping was requested.
		for _, item := range s {
			return toRunStats(item.RunCount, item.LatencyP50, item.LatencyP99, item.TotalTokens, item.PromptTokens, item.CompletionTokens, item.TotalCost, item.ErrorRate, item.FeedbackStats), nil
		}
		return runStats{}, nil
	default:
		return runStats{}, fmt.Errorf("unhandled stats response type: %T", *res)
	}
}

func toRunStats(runCount int64, latencyP50, latencyP99 float64, totalTokens, promptTokens, completionTokens int64, totalCost float64, errorRate float64, feedbackStats map[string]interface{}) runStats {
	fs := make(map[string]any, len(feedbackStats))
	for k, v := range feedbackStats {
		fs[k] = v
	}
	var cost any
	if totalCost > 0 {
		cost = totalCost
	}
	return runStats{
		RunCount:         runCount,
		LatencyP50:       latencyP50,
		LatencyP99:       latencyP99,
		TotalTokens:      totalTokens,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalCost:        cost,
		ErrorRate:        errorRate,
		FeedbackStats:    fs,
	}
}

// parseFlexTime parses RFC3339 or YYYY-MM-DD.
func parseFlexTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

func printStatsPretty(primary, compare *runStats, hasCompare bool) {
	if primary == nil {
		fmt.Println("No stats returned.")
		return
	}
	p := primary
	c := compare

	// ── Overview ──────────────────────────────────────────────────────────────
	if hasCompare && c != nil {
		cols := []string{"Metric", "Primary", "Comparison", "Delta"}
		rows := [][]string{
			{"Traces", fmt.Sprintf("%d", p.RunCount), fmt.Sprintf("%d", c.RunCount), fmtDeltaInt(p.RunCount, c.RunCount)},
			{"Error rate", fmtPct(p.ErrorRate), fmtPct(c.ErrorRate), fmtDeltaPct(p.ErrorRate, c.ErrorRate)},
			{"Latency p50 (s)", fmtF2(p.LatencyP50), fmtF2(c.LatencyP50), fmtDeltaF(p.LatencyP50, c.LatencyP50)},
			{"Latency p99 (s)", fmtF2(p.LatencyP99), fmtF2(c.LatencyP99), fmtDeltaF(p.LatencyP99, c.LatencyP99)},
			{"Total tokens", fmt.Sprintf("%d", p.TotalTokens), fmt.Sprintf("%d", c.TotalTokens), fmtDeltaInt(p.TotalTokens, c.TotalTokens)},
			{"Prompt tokens", fmt.Sprintf("%d", p.PromptTokens), fmt.Sprintf("%d", c.PromptTokens), fmtDeltaInt(p.PromptTokens, c.PromptTokens)},
			{"Completion tokens", fmt.Sprintf("%d", p.CompletionTokens), fmt.Sprintf("%d", c.CompletionTokens), fmtDeltaInt(p.CompletionTokens, c.CompletionTokens)},
			{"Total cost", fmtOptFloat(p.TotalCost), fmtOptFloat(c.TotalCost), ""},
		}
		output.OutputTable(cols, rows, "Overview")
	} else {
		cols := []string{"Metric", "Value"}
		rows := [][]string{
			{"Traces", fmt.Sprintf("%d", p.RunCount)},
			{"Error rate", fmtPct(p.ErrorRate)},
			{"Latency p50 (s)", fmtF2(p.LatencyP50)},
			{"Latency p99 (s)", fmtF2(p.LatencyP99)},
			{"Total tokens", fmt.Sprintf("%d", p.TotalTokens)},
			{"Prompt tokens", fmt.Sprintf("%d", p.PromptTokens)},
			{"Completion tokens", fmt.Sprintf("%d", p.CompletionTokens)},
			{"Total cost", fmtOptFloat(p.TotalCost)},
		}
		output.OutputTable(cols, rows, "Overview")
	}

	// ── Feedback keys ─────────────────────────────────────────────────────────
	if len(p.FeedbackStats) == 0 {
		fmt.Println("No feedback stats available for this window.")
		return
	}

	type kv struct {
		key   string
		stats map[string]any
	}
	var keys []kv
	for k, v := range p.FeedbackStats {
		if m, ok := v.(map[string]any); ok {
			keys = append(keys, kv{k, m})
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].key < keys[j].key })

	if hasCompare && c != nil {
		cols := []string{"Feedback key", "n (primary)", "avg (primary)", "n (compare)", "avg (compare)"}
		var rows [][]string
		for _, kv := range keys {
			pN, pAvg := extractFeedbackStat(kv.stats)
			cN, cAvg := "-", "-"
			if cv, ok := c.FeedbackStats[kv.key]; ok {
				if cm, ok := cv.(map[string]any); ok {
					cN, cAvg = extractFeedbackStat(cm)
				}
			}
			rows = append(rows, []string{kv.key, pN, pAvg, cN, cAvg})
		}
		output.OutputTable(cols, rows, "Feedback Keys")
	} else {
		cols := []string{"Feedback key", "n", "avg", "stdev", "values (top 5)"}
		var rows [][]string
		for _, kv := range keys {
			n, avg := extractFeedbackStat(kv.stats)
			stdev := fmtOptFloat(kv.stats["stdev"])
			vals := formatTopValues(kv.stats["values"], 5)
			rows = append(rows, []string{kv.key, n, avg, stdev, vals})
		}
		output.OutputTable(cols, rows, "Feedback Keys")
	}
}

func extractFeedbackStat(m map[string]any) (n, avg string) {
	n = fmtOptFloat(m["n"])
	avg = fmtOptFloat(m["avg"])
	return
}

func fmtOptFloat(v any) string {
	if v == nil {
		return "-"
	}
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return "-"
		}
		return strconv.FormatFloat(x, 'f', 3, 64)
	case int64:
		return strconv.FormatInt(x, 10)
	case string:
		return x
	}
	return fmt.Sprintf("%v", v)
}

func fmtPct(f float64) string {
	if math.IsNaN(f) {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", f*100)
}

func fmtF2(f float64) string {
	if math.IsNaN(f) || f == 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f", f)
}

func fmtDeltaInt(a, b int64) string {
	d := a - b
	if d > 0 {
		return fmt.Sprintf("+%d", d)
	}
	return fmt.Sprintf("%d", d)
}

func fmtDeltaF(a, b float64) string {
	if math.IsNaN(a) || math.IsNaN(b) {
		return "-"
	}
	d := a - b
	if d > 0 {
		return fmt.Sprintf("+%.2f", d)
	}
	return fmt.Sprintf("%.2f", d)
}

func fmtDeltaPct(a, b float64) string {
	if math.IsNaN(a) || math.IsNaN(b) {
		return "-"
	}
	d := (a - b) * 100
	if d > 0 {
		return fmt.Sprintf("+%.1f%%", d)
	}
	return fmt.Sprintf("%.1f%%", d)
}

func formatTopValues(v any, n int) string {
	if v == nil {
		return "-"
	}
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return "-"
	}
	type pair struct {
		k string
		c float64
	}
	var pairs []pair
	for k, cnt := range m {
		f, _ := toFloat64(cnt)
		pairs = append(pairs, pair{k, f})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].c > pairs[j].c })
	if len(pairs) > n {
		pairs = pairs[:n]
	}
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = fmt.Sprintf("%s:%.0f", p.k, p.c)
	}
	return strings.Join(parts, " ")
}

func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	}
	return 0, false
}
