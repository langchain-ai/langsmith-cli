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

var datasetsExperimentRunsQuery = requestflag.WithInnerFlags(cli.Command{
	Name:    "query",
	Usage:   "Returns a paginated page of dataset examples with runs from the requested\nexperiments. Response uses the canonical `{items, next_cursor}` envelope.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[string]{
			Name:     "comparative-experiment-id",
			Usage:    "`comparative_experiment_id` scopes pairwise-annotation feedback (optional).",
			BodyPath: "comparative_experiment_id",
		},
		&requestflag.Flag[string]{
			Name:     "cursor",
			Usage:    "`cursor` is the opaque string from a previous response's `next_cursor`. Absent for the first page.",
			BodyPath: "cursor",
		},
		&requestflag.Flag[[]string]{
			Name:     "example-id",
			Usage:    "`example_ids` optionally restricts the page to these dataset example UUIDs (max 1000).",
			BodyPath: "example_ids",
		},
		&requestflag.Flag[[]string]{
			Name:     "experiment-id",
			Usage:    "`experiment_ids` lists the experiment (tracing session) UUIDs to query. Required, non-empty.",
			BodyPath: "experiment_ids",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "filters",
			Usage:    "`filters` maps a project (session) UUID string to a list of filter expressions (optional).",
			BodyPath: "filters",
		},
		&requestflag.Flag[int64]{
			Name:     "page-size",
			Usage:    "`page_size` is the maximum number of examples to return. Defaults to 20, max 100.",
			BodyPath: "page_size",
		},
		&requestflag.Flag[[]string]{
			Name:     "select",
			Usage:    "`selects` lists which run properties to include. Omitted => only `id`. Tokens mirror /v2/runs/query.",
			BodyPath: "selects",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "sort",
			Usage:    "`sort` controls feedback-score sorting (single project only).",
			BodyPath: "sort",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleDatasetsExperimentRunsQuery,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"sort": {
		&requestflag.InnerFlag[string]{
			Name:       "sort.by",
			Usage:      "`by` is the feedback selector, e.g. `feedback.correctness` (the `feedback.` prefix is optional).",
			InnerField: "by",
		},
		&requestflag.InnerFlag[string]{
			Name:       "sort.order",
			Usage:      "`order` is `ASC` or `DESC` (defaults to `DESC`).",
			InnerField: "order",
		},
	},
})

func handleDatasetsExperimentRunsQuery(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("dataset-id") && len(unusedArgs) > 0 {
		cmd.Set("dataset-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
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

	params := langsmith.DatasetExperimentRunQueryParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Datasets.ExperimentRuns.Query(
			ctx,
			cmd.Value("dataset-id").(string),
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
			Title:          "datasets:experiment-runs query",
			Transform:      transform,
		})
	} else {
		iter := client.Datasets.ExperimentRuns.QueryAutoPaging(
			ctx,
			cmd.Value("dataset-id").(string),
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
			Title:          "datasets:experiment-runs query",
			Transform:      transform,
		})
	}
}
