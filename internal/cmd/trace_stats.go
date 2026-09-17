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

// runStats is the internal representation used for display. Token and cost
// fields are per-window sums; the p50/p99 fields are per-trace percentiles.
type runStats struct {
	RunCount         int64          `json:"run_count"`
	LatencyP50       float64        `json:"latency_p50"`
	LatencyP99       float64        `json:"latency_p99"`
	TotalTokens      int64          `json:"total_tokens"`
	PromptTokens     int64          `json:"prompt_tokens"`
	CompletionTokens int64          `json:"completion_tokens"`
	MedianTokens     int64          `json:"median_tokens"`
	TokensP99        int64          `json:"tokens_p99"`
	TotalCost        float64        `json:"total_cost"`
	PromptCost       float64        `json:"prompt_cost"`
	CompletionCost   float64        `json:"completion_cost"`
	CostP50          float64        `json:"cost_p50"`
	CostP99          float64        `json:"cost_p99"`
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
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Aggregate stats for traces in a project (token usage, latency, costs, feedback)",
		Long: `Fetch aggregate stats for root traces in a project.

Returns run count, error rate, latency percentiles, token and cost totals
(split into prompt/completion), per-trace token and cost percentiles, and the
top feedback keys with their score distributions. Useful for spotting trends,
discovering available feedback keys, and understanding score ranges before
building evaluators.

Token and cost totals are sums over the window; the p50/p99 figures are
per-trace distributions.

Optionally pass --compare-since/--compare-before (or --compare-last-n-minutes)
to fetch a second time window side-by-side for trend comparison.

Examples:
  langsmith trace stats --project my-app
  langsmith trace stats --project my-app --last-n-minutes 120
  langsmith trace stats --project my-app --since <YYYY-MM-DD> --compare-since <YYYY-MM-DD> --compare-before <YYYY-MM-DD>
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
				if err := output.OutputJSON(result, outputFile); err != nil {
					ExitCommandError(err)
				}
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
			Select:    langsmith.F(traceStatsSelect()),
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
		return statsFromResponse(s), nil
	case langsmith.RunStatsResponseMap:
		// group_by response — extract the first (and only) entry when no grouping was requested.
		for _, item := range s {
			return statsFromMapItem(item), nil
		}
		return runStats{}, nil
	default:
		return runStats{}, fmt.Errorf("unhandled stats response type: %T", *res)
	}
}

// traceStatsSelect is the metric set requested for every trace stats call.
func traceStatsSelect() []langsmith.RunStatsQueryParamsSelect {
	return []langsmith.RunStatsQueryParamsSelect{
		langsmith.RunStatsQueryParamsSelectRunCount,
		langsmith.RunStatsQueryParamsSelectLatencyP50,
		langsmith.RunStatsQueryParamsSelectLatencyP99,
		langsmith.RunStatsQueryParamsSelectTotalTokens,
		langsmith.RunStatsQueryParamsSelectPromptTokens,
		langsmith.RunStatsQueryParamsSelectCompletionTokens,
		langsmith.RunStatsQueryParamsSelectMedianTokens,
		langsmith.RunStatsQueryParamsSelectTokensP99,
		langsmith.RunStatsQueryParamsSelectTotalCost,
		langsmith.RunStatsQueryParamsSelectPromptCost,
		langsmith.RunStatsQueryParamsSelectCompletionCost,
		langsmith.RunStatsQueryParamsSelectCostP50,
		langsmith.RunStatsQueryParamsSelectCostP99,
		langsmith.RunStatsQueryParamsSelectErrorRate,
		langsmith.RunStatsQueryParamsSelectFeedbackStats,
	}
}

// statsFromResponse and statsFromMapItem map the two shapes of the stats
// response union onto runStats. The SDK models them as distinct structs with
// identical fields, so neither generics nor a shared interface applies.
func statsFromResponse(s langsmith.RunStatsResponseRunStats) runStats {
	return runStats{
		RunCount:         s.RunCount,
		LatencyP50:       s.LatencyP50,
		LatencyP99:       s.LatencyP99,
		TotalTokens:      s.TotalTokens,
		PromptTokens:     s.PromptTokens,
		CompletionTokens: s.CompletionTokens,
		MedianTokens:     s.MedianTokens,
		TokensP99:        s.TokensP99,
		TotalCost:        s.TotalCost,
		PromptCost:       s.PromptCost,
		CompletionCost:   s.CompletionCost,
		CostP50:          s.CostP50,
		CostP99:          s.CostP99,
		ErrorRate:        s.ErrorRate,
		FeedbackStats:    copyFeedbackStats(s.FeedbackStats),
	}
}

func statsFromMapItem(s langsmith.RunStatsResponseMapItem) runStats {
	return runStats{
		RunCount:         s.RunCount,
		LatencyP50:       s.LatencyP50,
		LatencyP99:       s.LatencyP99,
		TotalTokens:      s.TotalTokens,
		PromptTokens:     s.PromptTokens,
		CompletionTokens: s.CompletionTokens,
		MedianTokens:     s.MedianTokens,
		TokensP99:        s.TokensP99,
		TotalCost:        s.TotalCost,
		PromptCost:       s.PromptCost,
		CompletionCost:   s.CompletionCost,
		CostP50:          s.CostP50,
		CostP99:          s.CostP99,
		ErrorRate:        s.ErrorRate,
		FeedbackStats:    copyFeedbackStats(s.FeedbackStats),
	}
}

func copyFeedbackStats(in map[string]interface{}) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
			{"Tokens p50 / trace", fmt.Sprintf("%d", p.MedianTokens), fmt.Sprintf("%d", c.MedianTokens), fmtDeltaInt(p.MedianTokens, c.MedianTokens)},
			{"Tokens p99 / trace", fmt.Sprintf("%d", p.TokensP99), fmt.Sprintf("%d", c.TokensP99), fmtDeltaInt(p.TokensP99, c.TokensP99)},
			{"Total cost", fmtCost(p.TotalCost), fmtCost(c.TotalCost), fmtDeltaCost(p.TotalCost, c.TotalCost)},
			{"Prompt cost", fmtCost(p.PromptCost), fmtCost(c.PromptCost), fmtDeltaCost(p.PromptCost, c.PromptCost)},
			{"Completion cost", fmtCost(p.CompletionCost), fmtCost(c.CompletionCost), fmtDeltaCost(p.CompletionCost, c.CompletionCost)},
			{"Cost p50 / trace", fmtCost(p.CostP50), fmtCost(c.CostP50), fmtDeltaCost(p.CostP50, c.CostP50)},
			{"Cost p99 / trace", fmtCost(p.CostP99), fmtCost(c.CostP99), fmtDeltaCost(p.CostP99, c.CostP99)},
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
			{"Tokens p50 / trace", fmt.Sprintf("%d", p.MedianTokens)},
			{"Tokens p99 / trace", fmt.Sprintf("%d", p.TokensP99)},
			{"Total cost", fmtCost(p.TotalCost)},
			{"Prompt cost", fmtCost(p.PromptCost)},
			{"Completion cost", fmtCost(p.CompletionCost)},
			{"Cost p50 / trace", fmtCost(p.CostP50)},
			{"Cost p99 / trace", fmtCost(p.CostP99)},
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

// fmtCost renders a USD amount without scientific notation. Per-trace costs are
// routinely well under a cent, so small values get more decimal places.
func fmtCost(f float64) string {
	if f == 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return "-"
	}
	if math.Abs(f) >= 0.01 {
		return fmt.Sprintf("%.4f", f)
	}
	return fmt.Sprintf("%.6f", f)
}

func fmtDeltaCost(a, b float64) string {
	if math.IsNaN(a) || math.IsNaN(b) {
		return "-"
	}
	d := a - b
	if d == 0 {
		return "-"
	}
	sign := ""
	if d > 0 {
		sign = "+"
	}
	return sign + fmtCost(d)
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
