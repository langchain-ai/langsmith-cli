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

var datasetsRunsQuery = requestflag.WithInnerFlags(cli.Command{
	Name:    "query",
	Usage:   "Fetch examples for a dataset, and fetch the runs for each example if they are\nassociated with the given session_ids.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[[]string]{
			Name:     "session-id",
			Required: true,
			BodyPath: "session_ids",
		},
		&requestflag.Flag[*string]{
			Name:      "format",
			Usage:     "Response format, e.g., 'csv'",
			QueryPath: "format",
		},
		&requestflag.Flag[*string]{
			Name:     "comparative-experiment-id",
			BodyPath: "comparative_experiment_id",
		},
		&requestflag.Flag[any]{
			Name:     "example-id",
			BodyPath: "example_ids",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "filters",
			BodyPath: "filters",
		},
		&requestflag.Flag[bool]{
			Name:     "include-annotator-detail",
			Default:  false,
			BodyPath: "include_annotator_detail",
		},
		&requestflag.Flag[*int64]{
			Name:     "limit",
			BodyPath: "limit",
		},
		&requestflag.Flag[int64]{
			Name:     "offset",
			Default:  0,
			BodyPath: "offset",
		},
		&requestflag.Flag[bool]{
			Name:     "preview",
			Default:  false,
			BodyPath: "preview",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "sort-params",
			BodyPath: "sort_params",
		},
	},
	Action:          handleDatasetsRunsQuery,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"sort-params": {
		&requestflag.InnerFlag[string]{
			Name:       "sort-params.sort-by",
			InnerField: "sort_by",
		},
		&requestflag.InnerFlag[string]{
			Name:       "sort-params.sort-order",
			Usage:      `Allowed values: "ASC", "DESC".`,
			InnerField: "sort_order",
		},
	},
})

func handleDatasetsRunsQuery(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetRunQueryParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.Runs.Query(
		ctx,
		cmd.Value("dataset-id").(string),
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
		Title:          "datasets:runs query",
		Transform:      transform,
	})
}
