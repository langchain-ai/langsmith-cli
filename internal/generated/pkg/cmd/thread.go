// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/apiquery"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
	"github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli/v3"
)

var threadsAggregateStats = cli.Command{
	Name:    "aggregate-stats",
	Usage:   "GET with body payload — no resources created. Returns aggregate statistics for\nthreads in a tracing project. The response includes the thread counts, run\ncounts, latency percentiles, rates, token totals, and cost totals requested in\n`select`.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "project-id",
			Usage:    "`project_id` is the tracing project UUID.",
			Required: true,
			BodyPath: "project_id",
		},
		&requestflag.Flag[[]string]{
			Name:     "select",
			Usage:    "`select` lists the aggregate statistics to compute and return. At least one value is required.",
			Required: true,
			BodyPath: "select",
		},
		&requestflag.Flag[string]{
			Name:     "filter",
			Usage:    "`filter` is a deprecated, unscoped LangSmith filter expression evaluated\nagainst trace root runs. Kept for compatibility with deployments that\nserve this endpoint via the legacy ClickHouse backend (no SmithDB query\nservice configured); prefer `trace_filter`, `tree_filter`, or\n`thread_filter` otherwise, since those require SmithDB.",
			BodyPath: "filter",
		},
		&requestflag.Flag[any]{
			Name:     "max-start-time",
			Usage:    "`max_start_time` is the exclusive upper bound on thread activity (RFC3339 date-time). Defaults to now (UTC) when omitted.",
			BodyPath: "max_start_time",
		},
		&requestflag.Flag[any]{
			Name:     "min-start-time",
			Usage:    "`min_start_time` is the inclusive lower bound on thread activity (RFC3339 date-time). Defaults to 1 day before now (UTC) when omitted.",
			BodyPath: "min_start_time",
		},
		&requestflag.Flag[string]{
			Name:     "thread-filter",
			Usage:    "`thread_filter` narrows eligible threads using a LangSmith filter expression evaluated against the complete thread summary.",
			BodyPath: "thread_filter",
		},
		&requestflag.Flag[string]{
			Name:     "trace-filter",
			Usage:    "`trace_filter` narrows eligible threads to those containing a trace whose root run matches this LangSmith filter expression.",
			BodyPath: "trace_filter",
		},
		&requestflag.Flag[string]{
			Name:     "tree-filter",
			Usage:    "`tree_filter` narrows eligible threads to those containing a matching run anywhere in a trace tree.",
			BodyPath: "tree_filter",
		},
	},
	Action:          handleThreadsAggregateStats,
	HideHelpCommand: true,
}

var threadsListTraces = cli.Command{
	Name:    "list-traces",
	Usage:   "Retrieve all traces belonging to a specific thread within a project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "thread-id",
			Required:  true,
			PathParam: "thread_id",
		},
		&requestflag.Flag[string]{
			Name:      "project-id",
			Usage:     "`project_id` is the tracing project UUID (required).",
			Required:  true,
			QueryPath: "project_id",
		},
		&requestflag.Flag[string]{
			Name:      "cursor",
			Usage:     "`cursor` is the opaque string from a previous response's `next_cursor`. Omit on the first request; pass the returned cursor to fetch the next page.",
			QueryPath: "cursor",
		},
		&requestflag.Flag[string]{
			Name:      "filter",
			Usage:     "`filter` narrows which traces are returned for this thread, using a LangSmith filter expression evaluated against each root trace run.\nFor example: eq(status, \"success\") or has(tags, \"production\").\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			QueryPath: "filter",
		},
		&requestflag.Flag[int64]{
			Name:      "page-size",
			Usage:     "`page_size` is the maximum number of traces to return in this response. Defaults to 20 when omitted; must be between 1 and 100 inclusive when set.",
			Default:   20,
			QueryPath: "page_size",
		},
		&requestflag.Flag[[]string]{
			Name:      "select",
			Usage:     "`selects` lists which properties to include on each returned trace (repeatable query parameter). Accepts any value of the `ThreadTraceSelectField` enum. Properties not listed are omitted from each trace object; `trace_id` is always returned.",
			QueryPath: "selects",
		},
		&requestflag.Flag[string]{
			Name:      "trace-filter",
			Usage:     "`trace_filter` narrows traces by applying a LangSmith filter expression to each trace's root run.",
			QueryPath: "trace_filter",
		},
		&requestflag.Flag[string]{
			Name:      "tree-filter",
			Usage:     "`tree_filter` narrows traces to those containing at least one run that matches the LangSmith filter expression.",
			QueryPath: "tree_filter",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleThreadsListTraces,
	HideHelpCommand: true,
}

var threadsQuery = cli.Command{
	Name:    "query",
	Usage:   "Query threads within a project (session), with cursor-based pagination. Returns\nthreads matching the given time range and optional filters.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "cursor",
			Usage:    "`cursor` is the opaque string from a previous response's `next_cursor`. Omit on the first request; pass the returned cursor to fetch the next page.",
			BodyPath: "cursor",
		},
		&requestflag.Flag[string]{
			Name:     "filter",
			Usage:    "`filter` narrows which threads are returned, using a LangSmith filter expression evaluated against each thread's root run.\nFor example: has(tags, \"production\") or eq(status, \"error\").\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "filter",
		},
		&requestflag.Flag[any]{
			Name:     "max-start-time",
			Usage:    "`max_start_time` is the exclusive upper bound on thread activity (RFC3339 date-time). Defaults to now (UTC) when omitted.",
			BodyPath: "max_start_time",
		},
		&requestflag.Flag[any]{
			Name:     "min-start-time",
			Usage:    "`min_start_time` is the inclusive lower bound on thread activity (RFC3339 date-time). Defaults to 1 day before now (UTC) when omitted.",
			BodyPath: "min_start_time",
		},
		&requestflag.Flag[int64]{
			Name:     "page-size",
			Usage:    "`page_size` is the maximum number of threads to return in this response. Defaults to 20 when omitted; must be between 1 and 100 inclusive when set. The response may contain fewer threads than `page_size` even when `next_cursor` is non-null.",
			Default:  20,
			BodyPath: "page_size",
		},
		&requestflag.Flag[string]{
			Name:     "project-id",
			Usage:    "`project_id` is the tracing project UUID.",
			BodyPath: "project_id",
		},
		&requestflag.Flag[string]{
			Name:     "thread-filter",
			Usage:    "`thread_filter` narrows results using a LangSmith filter expression evaluated against each complete thread summary.\nSelf-hosted deployments require LangSmith v0.17 or later; unsupported deployments return 501.\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "thread_filter",
		},
		&requestflag.Flag[string]{
			Name:     "trace-filter",
			Usage:    "`trace_filter` narrows results to threads containing at least one trace whose root run matches this LangSmith filter expression.\nTrace-level aggregate fields are evaluated using the complete trace summary.\nSelf-hosted deployments require LangSmith v0.17 or later; unsupported deployments return 501.\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "trace_filter",
		},
		&requestflag.Flag[string]{
			Name:     "tree-filter",
			Usage:    "`tree_filter` narrows results to threads containing at least one trace with a matching run anywhere in its run tree.\nSelf-hosted deployments require LangSmith v0.17 or later; unsupported deployments return 501.\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "tree_filter",
		},
		&requestflag.Flag[string]{
			Name:       "accept",
			HeaderPath: "Accept",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleThreadsQuery,
	HideHelpCommand: true,
}

var threadsStats = cli.Command{
	Name:    "stats",
	Usage:   "Compute aggregate stats for a single thread (turn count, latency percentiles,\ntoken/cost sums, and detail breakdowns) within a project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "thread-id",
			Required:  true,
			PathParam: "thread_id",
		},
		&requestflag.Flag[[]string]{
			Name:      "select",
			Usage:     "`selects` lists which aggregate stats to compute and return (repeatable query parameter). At least one value is required. Accepts any value of `SingleThreadStatsSelectField`.",
			Required:  true,
			QueryPath: "selects",
		},
		&requestflag.Flag[string]{
			Name:      "session-id",
			Usage:     "`session_id` is the tracing project (session) UUID (required).",
			Required:  true,
			QueryPath: "session_id",
		},
		&requestflag.Flag[string]{
			Name:      "filter",
			Usage:     "`filter` narrows which of the thread's traces are aggregated, using a LangSmith filter expression. For example: lt(start_time, \"2025-01-01T00:00:00Z\") or eq(trace_id, \"0190a1b2-c3d4-7ef0-a5b6-6ea3a82e9328\").\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			QueryPath: "filter",
		},
	},
	Action:          handleThreadsStats,
	HideHelpCommand: true,
}

func handleThreadsAggregateStats(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.ThreadAggregateStatsParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Threads.AggregateStats(ctx, params, options...)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "threads aggregate-stats",
		Transform:      transform,
	})
}

func handleThreadsListTraces(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("thread-id") && len(unusedArgs) > 0 {
		cmd.Set("thread-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.ThreadListTracesParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Threads.ListTraces(
			ctx,
			cmd.Value("thread-id").(string),
			params,
			options...,
		)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "threads list-traces",
			Transform:      transform,
		})
	} else {
		iter := client.Threads.ListTracesAutoPaging(
			ctx,
			cmd.Value("thread-id").(string),
			params,
			options...,
		)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "threads list-traces",
			Transform:      transform,
		})
	}
}

func handleThreadsQuery(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.ThreadQueryParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Threads.Query(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "threads query",
			Transform:      transform,
		})
	} else {
		iter := client.Threads.QueryAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "threads query",
			Transform:      transform,
		})
	}
}

func handleThreadsStats(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("thread-id") && len(unusedArgs) > 0 {
		cmd.Set("thread-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.ThreadStatsParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Threads.Stats(
		ctx,
		cmd.Value("thread-id").(string),
		params,
		options...,
	)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "threads stats",
		Transform:      transform,
	})
}
