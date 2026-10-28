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

var tracesStreamingQuery = cli.Command{
	Name:    "query",
	Usage:   "Returns a paginated list of traces (root runs) for a single tracing project.\nEach item carries the trace's root run plus optional trace-wide aggregates\n(`total_tokens`, `total_cost`, `first_token_time`) under `trace_aggregates`, so\nclients never have to merge by `trace_id`.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "ai-search",
			Usage:    "`ai_search` is a plain-language criterion evaluated against the messages from the agent trajectory scoped to the trace.\nAND-ed with the ordinary filters. Requires semantic filtering enabled for the deployment.\nMust contain nonempty text of at most 2000 UTF-8 bytes.",
			BodyPath: "ai_search",
		},
		&requestflag.Flag[string]{
			Name:     "cursor",
			Usage:    "`cursor` is the opaque string returned in a previous response's `next_cursor`.",
			BodyPath: "cursor",
		},
		&requestflag.Flag[any]{
			Name:     "max-feedback-modified-at",
			Usage:    "`max_feedback_modified_at` is the exclusive upper bound (RFC3339) on feedback `modified_at`, applied to the feedback conditions in `trace_filter` or `tree_filter` (for example eq(feedback_key, \"correctness\")).\nRequires at least one feedback condition. Root runs must still fall within the `start_time` window.",
			BodyPath: "max_feedback_modified_at",
		},
		&requestflag.Flag[any]{
			Name:     "max-start-time",
			Usage:    "`max_start_time` is the exclusive upper bound for the root-run start time scan (RFC3339). Defaults to the request time when omitted.",
			BodyPath: "max_start_time",
		},
		&requestflag.Flag[any]{
			Name:     "min-feedback-modified-at",
			Usage:    "`min_feedback_modified_at` is the inclusive lower bound (RFC3339) on feedback `modified_at`, applied to the feedback conditions in `trace_filter` or `tree_filter` (for example eq(feedback_key, \"correctness\")).\nRequires at least one feedback condition. Root runs must still fall within the `start_time` window.",
			BodyPath: "min_feedback_modified_at",
		},
		&requestflag.Flag[any]{
			Name:     "min-start-time",
			Usage:    "`min_start_time` is the inclusive lower bound for the root-run start time scan (RFC3339). Defaults to 24 hours before the request when omitted.",
			BodyPath: "min_start_time",
		},
		&requestflag.Flag[int64]{
			Name:     "page-size",
			Usage:    "`page_size` is the maximum number of traces to return per page. Defaults to 20; must be between 1 and 100 when set.",
			Default:  20,
			BodyPath: "page_size",
		},
		&requestflag.Flag[string]{
			Name:     "project-id",
			Usage:    "`project_id` is the UUID of the tracing project that owns the traces. Required.",
			BodyPath: "project_id",
		},
		&requestflag.Flag[[]string]{
			Name:     "select",
			Usage:    "`selects` lists which properties to include on each returned trace. Properties listed here are routed to the appropriate sub-object on each item: `total_tokens`, `total_cost`, and `first_token_time` appear under `trace_aggregates`; everything else appears under `root_run`. If omitted, only `id` is returned on `root_run`.",
			BodyPath: "selects",
		},
		&requestflag.Flag[string]{
			Name:     "trace-filter",
			Usage:    "`trace_filter` narrows results to traces whose root run matches this LangSmith filter expression. This filter targets root runs only — `is_root = true` is implied.\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "trace_filter",
		},
		&requestflag.Flag[[]string]{
			Name:     "trace-id",
			Usage:    "`trace_ids` is an optional fast-path restriction to a known set of trace UUIDs. Equivalent in result to including each UUID in a `trace_filter`, but more efficient at scale.",
			BodyPath: "trace_ids",
		},
		&requestflag.Flag[string]{
			Name:     "tree-filter",
			Usage:    "`tree_filter` narrows results to traces containing at least one run anywhere in the run tree (root or descendant) that matches this LangSmith filter expression.",
			BodyPath: "tree_filter",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleTracesStreamingQuery,
	HideHelpCommand: true,
}

func handleTracesStreamingQuery(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.TraceStreamingQueryParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	stream := client.Traces.Streaming.QueryStreaming(ctx, params, options...)
	maxItems := int64(-1)
	if cmd.IsSet("max-items") {
		maxItems = cmd.Value("max-items").(int64)
	}
	return ShowJSONIterator(stream, maxItems, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "traces:streaming query",
		Transform:      transform,
	})
}
