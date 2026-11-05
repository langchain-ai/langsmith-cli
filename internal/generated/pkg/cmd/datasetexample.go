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

var datasetsExamplesUpdateMetadata = requestflag.WithInnerFlags(cli.Command{
	Name:    "update-metadata",
	Usage:   "Applies the same metadata changes, in order, to every listed example in a\ndataset. Examples that are not in the dataset or were deleted are skipped; the\nrest are updated together, so if the update fails none of them change. Answers\n207 with one status per submitted example ID: UPDATED, or NOT_FOUND for a\nskipped example. Metadata keys the operations do not name keep their values. To\nadd examples to a split or remove them from one, use ARRAY_ADD or ARRAY_REMOVE\non the dataset_split key. Requires permission to update datasets.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[[]string]{
			Name:     "example-id",
			Usage:    "The IDs of the examples to change; at most 10000. IDs of examples that are not in\nthe dataset, or were deleted, are skipped and reported as NOT_FOUND.",
			Required: true,
			BodyPath: "example_ids",
		},
		&requestflag.Flag[[]map[string]any]{
			Name:     "operation",
			Usage:    "The metadata changes to apply, in order; at most 50, of which at most 10 are\narray operations.",
			Required: true,
			BodyPath: "operations",
		},
	},
	Action:          handleDatasetsExamplesUpdateMetadata,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"operation": {
		&requestflag.InnerFlag[string]{
			Name:       "operation.key",
			Usage:      "The top-level metadata key to change, at most 255 bytes. Use dataset_split with\nARRAY_ADD or ARRAY_REMOVE to change which splits the examples belong to.",
			InnerField: "key",
		},
		&requestflag.InnerFlag[string]{
			Name:       "operation.operation",
			Usage:      "The change to make: SET replaces the value at key; ARRAY_ADD adds values to the\nlist at key; ARRAY_REMOVE removes values from it. For array operations, a missing\nor non-list value starts as an empty list, except for dataset_split, where it\nstarts as the base split.",
			InnerField: "operation",
		},
		&requestflag.InnerFlag[any]{
			Name:       "operation.value",
			Usage:      "The new value for a SET operation; any JSON value, at most 64 KB. Ignored by\narray operations.",
			InnerField: "value",
		},
		&requestflag.InnerFlag[[]string]{
			Name:       "operation.values",
			Usage:      "The values to add or remove for an array operation; at most 1000 values of at\nmost 255 bytes each. Ignored by SET.",
			InnerField: "values",
		},
	},
})

func handleDatasetsExamplesUpdateMetadata(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetExampleUpdateMetadataParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.Examples.UpdateMetadata(
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
		Title:          "datasets:examples update-metadata",
		Transform:      transform,
	})
}
