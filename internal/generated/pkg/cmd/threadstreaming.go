// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/apiquery"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
	"github.com/langchain-ai/langsmith-go"
	"github.com/urfave/cli/v3"
)

var threadsStreamingQuery = cli.Command{
	Name:    "query",
	Usage:   "GET with body payload — no resources created. Returns the threads in a tracing\nproject matching the query passed as the request payload. Results are filtered\nby the given time range and optional filters, with cursor-based pagination.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "ai-search",
			Usage:    "`ai_search` is a plain-language criterion evaluated against the messages from the agent trajectory scoped to the thread.\nAND-ed with the ordinary filters. Requires semantic filtering enabled for the deployment.\nMust contain nonempty text of at most 2000 UTF-8 bytes.",
			BodyPath: "ai_search",
		},
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
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleThreadsStreamingQuery,
	HideHelpCommand: true,
}

func handleThreadsStreamingQuery(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.ThreadStreamingQueryParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	stream := client.Threads.Streaming.QueryStreaming(ctx, params, options...)
	maxItems := int64(-1)
	if cmd.IsSet("max-items") {
		maxItems = cmd.Value("max-items").(int64)
	}
	return ShowJSONIterator(stream, maxItems, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "threads:streaming query",
		Transform:      transform,
	})
}
