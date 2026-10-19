package cmd

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
)

// traceRunsV1Limit caps how many runs a v1 trace fetch returns. The v2 trace
// endpoint returns the whole trace in one response.
const traceRunsV1Limit = 1000

// queryTracesAuto lists traces (root runs). On v2 it reads /v2/traces/query,
// whose total_tokens, total_cost and first_token_time cover the whole trace; a
// root row from /v2/runs/query carries only the root's own usage, which is
// empty for a chain root. v1 root rows already carry the trace-wide rollup.
// params is the canonical v1 query from BuildRunQueryParams; v2Opts apply only
// to the v2 request.
func queryTracesAuto(ctx context.Context, c *client.Client, params langsmith.RunQueryParams, v2Selects []langsmith.RunSelectField, sessionID string, limit, minTokens int, v2Opts ...option.RequestOption) ([]langsmith.RunSchema, error) {
	useV2, err := c.UseV2API(ctx)
	if err != nil {
		return nil, err
	}
	if !useV2 {
		return queryRuns(ctx, c, params, sessionID, limit, minTokens)
	}
	return queryTracesV2(ctx, c, params, v2Selects, sessionID, limit, minTokens, v2Opts...)
}

func queryTracesV2(ctx context.Context, c *client.Client, params langsmith.RunQueryParams, selects []langsmith.RunSelectField, sessionID string, limit, minTokens int, opts ...option.RequestOption) ([]langsmith.RunSchema, error) {
	if params.Error.Present {
		if params.Error.Value {
			addFilterClause(&params, `eq(status, "error")`)
		} else {
			addFilterClause(&params, `neq(status, "error")`)
		}
	}
	// On this endpoint total_tokens in a filter resolves to the trace total.
	if minTokens > 0 {
		addFilterClause(&params, fmt.Sprintf("gte(total_tokens, %d)", minTokens))
	}

	query := langsmith.TraceQueryParams{
		ProjectID: langsmith.F(sessionID),
		Selects:   langsmith.F(selects),
		PageSize:  langsmith.F(int64(min(limit, 100))),
	}
	if params.StartTime.Present {
		query.MinStartTime = langsmith.F(params.StartTime.Value)
	}
	if params.EndTime.Present {
		query.MaxStartTime = langsmith.F(params.EndTime.Value)
	}
	if params.Trace.Present {
		query.TraceIDs = langsmith.F([]string{params.Trace.Value})
	}
	if params.Filter.Present && params.Filter.Value != "" {
		query.TraceFilter = langsmith.F(params.Filter.Value)
	}

	var runs []langsmith.RunSchema
	iter := c.SDK.Traces.QueryAutoPaging(ctx, query, opts...)
	for len(runs) < limit && iter.Next() {
		trace := iter.Current()
		runs = append(runs, withTraceAggregates(runV2ToSchema(trace.RootRun), trace.TraceAggregates))
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("querying traces (v2): %w", err)
	}
	return runs, nil
}

// traceFeedbackWindowOptions sets the feedback modified-at window on a
// /v2/traces/query request. The SDK release in use has no typed fields for it.
func traceFeedbackWindowOptions(since, before time.Time) []option.RequestOption {
	var opts []option.RequestOption
	if !since.IsZero() {
		opts = append(opts, option.WithJSONSet("min_feedback_modified_at", since))
	}
	if !before.IsZero() {
		opts = append(opts, option.WithJSONSet("max_feedback_modified_at", before))
	}
	return opts
}

// withTraceAggregates replaces a root's own usage with the trace-wide totals.
// The aggregates have no prompt/completion split, so those fields are cleared
// rather than left describing the root run alone.
func withTraceAggregates(run langsmith.RunSchema, agg langsmith.TraceAggregates) langsmith.RunSchema {
	run.TotalTokens = agg.TotalTokens
	run.TotalCost = agg.TotalCost
	run.PromptTokens, run.CompletionTokens = 0, 0
	run.PromptCost, run.CompletionCost = 0, 0
	if !agg.FirstTokenTime.IsZero() {
		run.FirstTokenTime = agg.FirstTokenTime
	}
	return run
}

// hydrateTraceAggregates fills trace-wide usage onto v2 roots that were read
// from /v2/runs/query, which `trace list` uses for the feedback window on
// servers whose /v2/traces/query does not accept it.
func hydrateTraceAggregates(ctx context.Context, c *client.Client, sessionID string, roots []langsmith.RunSchema) error {
	for start := 0; start < len(roots); start += 100 {
		batch := roots[start:min(start+100, len(roots))]
		ids := make([]string, len(batch))
		earliest := batch[0].StartTime
		for i, r := range batch {
			ids[i] = r.TraceID
			if r.StartTime.Before(earliest) {
				earliest = r.StartTime
			}
		}
		query := langsmith.TraceQueryParams{
			ProjectID: langsmith.F(sessionID),
			TraceIDs:  langsmith.F(ids),
			PageSize:  langsmith.F(int64(len(batch))),
			Selects: langsmith.F([]langsmith.RunSelectField{
				langsmith.RunSelectFieldTraceID,
				langsmith.RunSelectFieldTotalTokens,
				langsmith.RunSelectFieldTotalCost,
				langsmith.RunSelectFieldFirstTokenTime,
			}),
		}
		if !earliest.IsZero() {
			query.MinStartTime = langsmith.F(earliest)
		}
		page, err := c.SDK.Traces.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("querying trace totals (v2): %w", err)
		}
		aggs := make(map[string]langsmith.TraceAggregates, len(page.Items))
		for _, t := range page.Items {
			aggs[t.RootRun.TraceID] = t.TraceAggregates
		}
		for i := range batch {
			if agg, ok := aggs[batch[i].TraceID]; ok {
				batch[i] = withTraceAggregates(batch[i], agg)
			}
		}
	}
	return nil
}

// fetchTraceRuns returns every run in one trace, oldest first. On v2 it reads
// /v2/traces/{trace_id}/runs, which returns the whole trace in one response and
// finds it without a start-time window; since, when set, keeps only runs that
// started at or after it. v1 queries runs by trace from v1Start and caps the
// result at traceRunsV1Limit.
func fetchTraceRuns(ctx context.Context, c *client.Client, sessionID, traceID string, v1Start, since time.Time, v1Select []langsmith.RunQueryParamsSelect, v2Selects []langsmith.RunSelectField) ([]langsmith.RunSchema, error) {
	useV2, err := c.UseV2API(ctx)
	if err != nil {
		return nil, err
	}
	if !useV2 {
		params := langsmith.RunQueryParams{
			Trace: langsmith.F(traceID),
			Order: langsmith.F(langsmith.RunQueryParamsOrderAsc),
		}
		if v1Select != nil {
			params.Select = langsmith.F(v1Select)
		}
		if !v1Start.IsZero() {
			params.StartTime = langsmith.F(v1Start)
		}
		return queryRuns(ctx, c, params, sessionID, traceRunsV1Limit, 0)
	}

	selects := make([]langsmith.TraceListRunsParamsSelect, len(v2Selects))
	for i, s := range v2Selects {
		selects[i] = langsmith.TraceListRunsParamsSelect(s)
	}
	params := langsmith.TraceListRunsParams{
		ProjectID: langsmith.F(sessionID),
		Selects:   langsmith.F(selects),
	}
	// The endpoint takes both bounds or neither.
	if !since.IsZero() {
		params.MinStartTime = langsmith.F(since)
		params.MaxStartTime = langsmith.F(time.Now().UTC())
	}
	resp, err := c.SDK.Traces.ListRuns(ctx, traceID, params)
	if err != nil {
		return nil, fmt.Errorf("querying trace runs (v2): %w", err)
	}
	runs := make([]langsmith.RunSchema, len(resp.Items))
	for i, r := range resp.Items {
		runs[i] = runV2ToSchema(r)
	}
	// The endpoint does not order its items.
	sort.SliceStable(runs, func(i, j int) bool {
		if !runs[i].StartTime.Equal(runs[j].StartTime) {
			return runs[i].StartTime.Before(runs[j].StartTime)
		}
		return runs[i].DottedOrder < runs[j].DottedOrder
	})
	return runs, nil
}
