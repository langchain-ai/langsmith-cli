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

var datasetsClone = cli.Command{
	Name:    "clone",
	Usage:   "Copy examples from a source dataset into an existing target dataset.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "source-dataset-id",
			Usage:    "ID of the dataset in this workspace to copy examples from. It is not modified.",
			Required: true,
			BodyPath: "source_dataset_id",
		},
		&requestflag.Flag[string]{
			Name:     "target-dataset-id",
			Usage:    "ID of the dataset in this workspace that receives the copied examples. Its current examples are kept.",
			Required: true,
			BodyPath: "target_dataset_id",
		},
		&requestflag.Flag[any]{
			Name:     "as-of",
			Usage:    "Only modifications made on or before this time are included. If None, the latest version of the dataset is used.",
			BodyPath: "as_of",
		},
		&requestflag.Flag[[]string]{
			Name:     "example",
			Usage:    "IDs of source examples to copy. Defaults to every example in the source version. When IDs are given and nothing is copied, because none match or either dataset is not found, the request fails with 404.",
			Default:  []string{},
			BodyPath: "examples",
		},
		&requestflag.Flag[any]{
			Name:     "split",
			Usage:    "Split name, or list of split names, assigned to every copied example in place of its source splits. Defaults to keeping the source splits.",
			BodyPath: "split",
		},
		&requestflag.Flag[any]{
			Name:     "tag-value-id",
			Usage:    "List of resource tag value IDs to attach to the target dataset, up to 100. Requires the datasets:tag_on_create permission.",
			BodyPath: "tag_value_ids",
		},
	},
	Action:          handleDatasetsClone,
	HideHelpCommand: true,
}

var datasetsRetrieveVersion = cli.Command{
	Name:    "retrieve-version",
	Usage:   "Get dataset version by as_of or exact tag.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[any]{
			Name:      "as-of",
			QueryPath: "as_of",
		},
		&requestflag.Flag[*string]{
			Name:      "tag",
			QueryPath: "tag",
		},
	},
	Action:          handleDatasetsRetrieveVersion,
	HideHelpCommand: true,
}

var datasetsUpdateTags = cli.Command{
	Name:    "update-tags",
	Usage:   "Tag a dataset version.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[any]{
			Name:     "as-of",
			Usage:    "The version to tag: an ISO 8601 timestamp, or the name of an existing tag on this dataset (latest resolves to the most recent example change, and returns 404 for a dataset with no examples). A value that does not parse as a timestamp is treated as a tag name, and an unknown tag name returns 404. Timestamps are not checked against the dataset history: the tag covers every example change made on or before that time, so a future time keeps picking up new changes until it passes. Use the exact as_of of an existing version, since looking up a version by tag returns 404 when the tag's time does not match a version.",
			Required: true,
			BodyPath: "as_of",
		},
		&requestflag.Flag[string]{
			Name:     "tag",
			Usage:    "The tag to set. If the dataset already has this tag, it is moved to the new version. The tag latest is reserved and returns 400.",
			Required: true,
			BodyPath: "tag",
		},
	},
	Action:          handleDatasetsUpdateTags,
	HideHelpCommand: true,
}

func handleDatasetsClone(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetCloneParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.Clone(ctx, params, options...)
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
		Title:          "datasets clone",
		Transform:      transform,
	})
}

func handleDatasetsRetrieveVersion(ctx context.Context, cmd *cli.Command) error {
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
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.DatasetGetVersionParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.GetVersion(
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
		Title:          "datasets retrieve-version",
		Transform:      transform,
	})
}

func handleDatasetsUpdateTags(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetUpdateTagsParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.UpdateTags(
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
		Title:          "datasets update-tags",
		Transform:      transform,
	})
}
