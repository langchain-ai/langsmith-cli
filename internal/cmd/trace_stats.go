package cmd

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newTraceStatsCmd() *cobra.Command {
	var (
		project     string
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

			projectName := ResolveProject(project)
			if projectName == "" {
				return fmt.Errorf("--project is required (or set LANGSMITH_PROJECT)")
			}

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				return fmt.Errorf("resolving project %q: %w", projectName, err)
			}

			// Primary window
			params := buildRunStatsParams(sessionID, since, before, lastNMin, filter)
			primary, err := c.SDK.Runs.Stats(ctx, params)
			if err != nil {
				return fmt.Errorf("fetching stats: %w", err)
			}

			// Optional comparison window
			var compare *langsmith.RunStatsResponseUnion
			hasCompare := cmpSince != "" || cmpBefore != "" || cmpLastNMin > 0
			if hasCompare {
				cmpParams := buildRunStatsParams(sessionID, cmpSince, cmpBefore, cmpLastNMin, filter)
				compare, err = c.SDK.Runs.Stats(ctx, cmpParams)
				if err != nil {
					return fmt.Errorf("fetching comparison stats: %w", err)
				}
			}

			fmt_ := GetFormat()
			if fmt_ == "pretty" {
				printStatsPretty(primary, compare, hasCompare)
			} else {
				result := map[string]any{"stats": primary}
				if hasCompare {
					result["compare"] = compare
				}
				output.OutputJSON(result, outputFile)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().StringVar(&since, "since", "", "Start of time window (RFC3339 or YYYY-MM-DD; default: 7 days ago)")
	cmd.Flags().StringVar(&before, "before", "", "End of time window (RFC3339 or YYYY-MM-DD; default: now)")
	cmd.Flags().IntVar(&lastNMin, "last-n-minutes", 0, "Shorthand: window = last N minutes (overrides --since)")
	cmd.Flags().StringVar(&cmpSince, "compare-since", "", "Comparison window start (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&cmpBefore, "compare-before", "", "Comparison window end (RFC3339 or YYYY-MM-DD; default: same as --since)")
	cmd.Flags().IntVar(&cmpLastNMin, "compare-last-n-minutes", 0, "Shorthand: comparison window = N minutes before the primary window starts")
	cmd.Flags().StringVar(&filter, "filter", "", "LangSmith filter DSL (applied to both windows if comparing)")
	cmd.Flags().StringVar(&outputFile, "output", "", "Write JSON output to file instead of stdout")
	return cmd
}

// buildRunStatsParams constructs RunStatsParams for a given session/time/filter.
func buildRunStatsParams(sessionID, since, before string, lastNMin int, filter string) langsmith.RunStatsParams {
	startTime := resolveStartTime(since, lastNMin)

	qp := langsmith.RunStatsQueryParams{
		Session:  langsmith.F([]string{sessionID}),
		IsRoot:   langsmith.F(true),
		StartTime: langsmith.F(startTime),
		Select: langsmith.F([]langsmith.RunStatsQueryParamsSelect{
			langsmith.RunStatsQueryParamsSelectRunCount,
			langsmith.RunStatsQueryParamsSelectLatencyP50,
			langsmith.RunStatsQueryParamsSelectLatencyP99,
			langsmith.RunStatsQueryParamsSelectLatencyAvg,
			langsmith.RunStatsQueryParamsSelectTotalTokens,
			langsmith.RunStatsQueryParamsSelectPromptTokens,
			langsmith.RunStatsQueryParamsSelectCompletionTokens,
			langsmith.RunStatsQueryParamsSelectTotalCost,
			langsmith.RunStatsQueryParamsSelectErrorRate,
			langsmith.RunStatsQueryParamsSelectFeedbackStats,
		}),
	}

	if before != "" {
		t, err := parseFlexTime(before)
		if err == nil {
			qp.EndTime = langsmith.F(t)
		}
	}

	if filter != "" {
		qp.Filter = langsmith.F(filter)
	}

	return langsmith.RunStatsParams{RunStatsQueryParams: qp}
}

// parseFlexTime parses RFC3339 or YYYY-MM-DD.
func parseFlexTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

// extractRunStats pulls a RunStatsResponseRunStats out of the union.
func extractRunStats(u *langsmith.RunStatsResponseUnion) *langsmith.RunStatsResponseRunStats {
	if u == nil {
		return nil
	}
	switch v := (*u).(type) {
	case langsmith.RunStatsResponseRunStats:
		return &v
	}
	return nil
}

func printStatsPretty(primary, compare *langsmith.RunStatsResponseUnion, hasCompare bool) {
	p := extractRunStats(primary)
	if p == nil {
		fmt.Println("No stats returned.")
		return
	}
	var c *langsmith.RunStatsResponseRunStats
	if hasCompare {
		c = extractRunStats(compare)
	}

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
			{"Total cost", p.TotalCost, c.TotalCost, ""},
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
			{"Total cost", p.TotalCost},
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
