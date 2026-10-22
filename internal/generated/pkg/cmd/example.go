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

var examplesRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Get a specific example.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "example-id",
			Required:  true,
			PathParam: "example_id",
		},
		&requestflag.Flag[any]{
			Name:      "as-of",
			Usage:     "Only modifications made on or before this time are included. If None, the latest version of the dataset is used.",
			Default:   "latest",
			QueryPath: "as_of",
		},
		&requestflag.Flag[*string]{
			Name:      "dataset",
			QueryPath: "dataset",
		},
	},
	Action:          handleExamplesRetrieve,
	HideHelpCommand: true,
}

var examplesUpdate = requestflag.WithInnerFlags(cli.Command{
	Name:    "update",
	Usage:   "Update a specific example.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "example-id",
			Required:  true,
			PathParam: "example_id",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "attachments-operations",
			BodyPath: "attachments_operations",
		},
		&requestflag.Flag[*string]{
			Name:     "dataset-id",
			BodyPath: "dataset_id",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "inputs",
			BodyPath: "inputs",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "metadata",
			BodyPath: "metadata",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "outputs",
			BodyPath: "outputs",
		},
		&requestflag.Flag[bool]{
			Name:     "overwrite",
			Default:  false,
			BodyPath: "overwrite",
		},
		&requestflag.Flag[any]{
			Name:     "split",
			BodyPath: "split",
		},
	},
	Action:          handleExamplesUpdate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"attachments-operations": {
		&requestflag.InnerFlag[map[string]any]{
			Name:       "attachments-operations.rename",
			Usage:      "Mapping of old attachment names to new names",
			InnerField: "rename",
		},
		&requestflag.InnerFlag[[]string]{
			Name:       "attachments-operations.retain",
			Usage:      "List of attachment names to keep",
			InnerField: "retain",
		},
	},
})

var examplesDeleteAll = cli.Command{
	Name:    "delete-all",
	Usage:   "Soft delete examples. Only deletes the examples in the 'latest' version of the\ndataset.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[[]string]{
			Name:      "example-id",
			Required:  true,
			QueryPath: "example_ids",
		},
	},
	Action:          handleExamplesDeleteAll,
	HideHelpCommand: true,
}

var examplesRetrieveCount = cli.Command{
	Name:    "retrieve-count",
	Usage:   "Count all examples by query params",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[any]{
			Name:      "id",
			QueryPath: "id",
		},
		&requestflag.Flag[any]{
			Name:      "as-of",
			Usage:     "Only modifications made on or before this time are included. If None, the latest version of the dataset is used.",
			Default:   "latest",
			QueryPath: "as_of",
		},
		&requestflag.Flag[*string]{
			Name:      "dataset",
			QueryPath: "dataset",
		},
		&requestflag.Flag[*string]{
			Name:      "filter",
			QueryPath: "filter",
		},
		&requestflag.Flag[any]{
			Name:      "full-text-contain",
			QueryPath: "full_text_contains",
		},
		&requestflag.Flag[*string]{
			Name:      "metadata",
			QueryPath: "metadata",
		},
		&requestflag.Flag[any]{
			Name:      "split",
			QueryPath: "splits",
		},
	},
	Action:          handleExamplesRetrieveCount,
	HideHelpCommand: true,
}

func handleExamplesRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("example-id") && len(unusedArgs) > 0 {
		cmd.Set("example-id", unusedArgs[0])
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

	params := langsmith.ExampleGetParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Examples.Get(
		ctx,
		cmd.Value("example-id").(string),
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
		Title:          "examples retrieve",
		Transform:      transform,
	})
}

func handleExamplesUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("example-id") && len(unusedArgs) > 0 {
		cmd.Set("example-id", unusedArgs[0])
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

	params := langsmith.ExampleUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Examples.Update(
		ctx,
		cmd.Value("example-id").(string),
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
		Title:          "examples update",
		Transform:      transform,
	})
}

func handleExamplesDeleteAll(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

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

	params := langsmith.ExampleDeleteAllParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Examples.DeleteAll(ctx, params, options...)
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
		Title:          "examples delete-all",
		Transform:      transform,
	})
}

func handleExamplesRetrieveCount(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

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

	params := langsmith.ExampleGetCountParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Examples.GetCount(ctx, params, options...)
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
		Title:          "examples retrieve-count",
		Transform:      transform,
	})
}
