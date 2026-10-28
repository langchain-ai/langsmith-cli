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

var runsStreamingQueryV2 = cli.Command{
	Name:    "query-v2",
	Usage:   "Returns a paginated list of runs for the given projects within min/max\nstart_time. Supports filters, cursor pagination, and `selects` to select fields\nto return.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "ai-search",
			Usage:    "`ai_search` is a plain-language criterion evaluated against the messages from the agent trajectory scoped to the run.\nAND-ed with the ordinary filters. Requires semantic filtering enabled for the deployment.\nMust contain nonempty text of at most 2000 UTF-8 bytes. Not supported on public dataset queries.",
			BodyPath: "ai_search",
		},
		&requestflag.Flag[string]{
			Name:     "cursor",
			Usage:    "`cursor` is the opaque string from a previous response's `next_cursor`. Treat it as opaque and pass it back unmodified.",
			BodyPath: "cursor",
		},
		&requestflag.Flag[string]{
			Name:     "filter",
			Usage:    "`filter` narrows results to runs matching this LangSmith filter expression, evaluated against each individual run.\nFor example: and(eq(run_type, \"llm\"), gt(latency, 5)) or eq(status, \"error\").\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "filter",
		},
		&requestflag.Flag[bool]{
			Name:     "has-error",
			Usage:    "`has_error` filters to runs that errored (true) or completed without error (false).",
			BodyPath: "has_error",
		},
		&requestflag.Flag[[]string]{
			Name:     "id",
			Usage:    "`ids` optionally limits the request to these run UUIDs.",
			BodyPath: "ids",
		},
		&requestflag.Flag[bool]{
			Name:     "include-subtree-usage",
			Usage:    "`include_subtree_usage` enriches selected token and cost fields on root and intermediate runs with subtree totals, and selected `first_token_time` with the earliest observed subtree timestamp.\nDescendants contribute regardless of this request's filters or time bounds. Runs without observed descendants retain their original values. Defaults to false.",
			Default:  false,
			BodyPath: "include_subtree_usage",
		},
		&requestflag.Flag[bool]{
			Name:     "is-root",
			Usage:    "`is_root` returns only root runs (true) or only non-root runs (false).",
			BodyPath: "is_root",
		},
		&requestflag.Flag[any]{
			Name:     "max-feedback-modified-at",
			Usage:    "`max_feedback_modified_at` is the exclusive upper bound (RFC3339) on feedback `modified_at`, applied to the feedback conditions in `filter`, `trace_filter`, or `tree_filter` (for example eq(feedback_key, \"correctness\")).\nRequires at least one feedback condition. Runs must still fall within the `start_time` window.",
			BodyPath: "max_feedback_modified_at",
		},
		&requestflag.Flag[any]{
			Name:     "max-start-time",
			Usage:    "`max_start_time` is the upper bound for run `start_time` (RFC3339). Defaults to now.",
			BodyPath: "max_start_time",
		},
		&requestflag.Flag[any]{
			Name:     "min-feedback-modified-at",
			Usage:    "`min_feedback_modified_at` is the inclusive lower bound (RFC3339) on feedback `modified_at`, applied to the feedback conditions in `filter`, `trace_filter`, or `tree_filter` (for example eq(feedback_key, \"correctness\")).\nRequires at least one feedback condition. Runs must still fall within the `start_time` window.",
			BodyPath: "min_feedback_modified_at",
		},
		&requestflag.Flag[any]{
			Name:     "min-start-time",
			Usage:    "`min_start_time` is the lower bound for run `start_time` (RFC3339). Defaults to 1 day ago.",
			BodyPath: "min_start_time",
		},
		&requestflag.Flag[int64]{
			Name:     "page-size",
			Usage:    "`page_size` is the maximum number of runs to return in this response. Defaults to 100 when omitted; must be between 1 and 1000 inclusive when set.",
			Default:  100,
			BodyPath: "page_size",
		},
		&requestflag.Flag[[]string]{
			Name:     "project-id",
			Usage:    "`project_ids` lists tracing project UUIDs to query.\nRequired unless `reference_dataset_id` is set. Mutually exclusive with `reference_dataset_id` — set exactly one of them.",
			BodyPath: "project_ids",
		},
		&requestflag.Flag[string]{
			Name:     "reference-dataset-id",
			Usage:    "`reference_dataset_id` resolves session IDs server-side from the dataset.\nRequired unless `project_ids` is set. Mutually exclusive with `project_ids` — set exactly one of them.\nWhen provided and `min_start_time` is omitted, the server derives it from the earliest session creation date.",
			BodyPath: "reference_dataset_id",
		},
		&requestflag.Flag[[]string]{
			Name:     "reference-example",
			Usage:    "`reference_examples` optionally limits to runs linked to these dataset example UUIDs.",
			BodyPath: "reference_examples",
		},
		&requestflag.Flag[string]{
			Name:     "run-type",
			Usage:    `Allowed values: "TOOL", "CHAIN", "LLM", "RETRIEVER", "EMBEDDING", "PROMPT", "PARSER".`,
			BodyPath: "run_type",
		},
		&requestflag.Flag[[]string]{
			Name:     "select",
			Usage:    "`selects` lists which properties to include on each returned run. If omitted, only `id` is returned. Properties not listed are omitted from each run object.",
			BodyPath: "selects",
		},
		&requestflag.Flag[string]{
			Name:     "trace-filter",
			Usage:    "`trace_filter` narrows results to runs whose root trace matches this LangSmith filter expression.\nUse this to filter by properties of the trace's root run — for example eq(status, \"success\") to include only traces that completed without error.\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "trace_filter",
		},
		&requestflag.Flag[string]{
			Name:     "trace-id",
			Usage:    "`trace_id` optionally limits results to runs belonging to this trace UUID.",
			BodyPath: "trace_id",
		},
		&requestflag.Flag[string]{
			Name:     "tree-filter",
			Usage:    "`tree_filter` narrows results to runs that belong to a trace containing at least one run matching this LangSmith filter expression anywhere in the run tree (not just the root).\nUse this to find runs inside traces that involved a specific tool, tag, or model — for example has(tags, \"production\") or eq(name, \"my_tool\").\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "tree_filter",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleRunsStreamingQueryV2,
	HideHelpCommand: true,
}

func handleRunsStreamingQueryV2(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.RunStreamingQueryV2Params{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	stream := client.Runs.Streaming.QueryV2Streaming(ctx, params, options...)
	maxItems := int64(-1)
	if cmd.IsSet("max-items") {
		maxItems = cmd.Value("max-items").(int64)
	}
	return ShowJSONIterator(stream, maxItems, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "runs:streaming query-v2",
		Transform:      transform,
	})
}
