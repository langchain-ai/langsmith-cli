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

var examplesCreate = cli.Command{
	Name:    "create",
	Usage:   "Create a new example.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "dataset-id",
			Required: true,
			BodyPath: "dataset_id",
		},
		&requestflag.Flag[*string]{
			Name:     "id",
			BodyPath: "id",
		},
		&requestflag.Flag[string]{
			Name:     "created-at",
			BodyPath: "created_at",
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
		&requestflag.Flag[*string]{
			Name:     "source-run-id",
			BodyPath: "source_run_id",
		},
		&requestflag.Flag[any]{
			Name:     "source-run-start-time",
			BodyPath: "source_run_start_time",
		},
		&requestflag.Flag[*string]{
			Name:     "source-session-id",
			BodyPath: "source_session_id",
		},
		&requestflag.Flag[*string]{
			Name:     "source-trace-id",
			BodyPath: "source_trace_id",
		},
		&requestflag.Flag[any]{
			Name:     "split",
			Default:  "base",
			BodyPath: "split",
		},
		&requestflag.Flag[bool]{
			Name:     "use-legacy-message-format",
			Usage:    "Use Legacy Message Format for LLM runs",
			Default:  false,
			BodyPath: "use_legacy_message_format",
		},
		&requestflag.Flag[[]string]{
			Name:     "use-source-run-attachment",
			Default:  []string{},
			BodyPath: "use_source_run_attachments",
		},
		&requestflag.Flag[bool]{
			Name:     "use-source-run-io",
			Default:  false,
			BodyPath: "use_source_run_io",
		},
	},
	Action:          handleExamplesCreate,
	HideHelpCommand: true,
}

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

var examplesList = cli.Command{
	Name:    "list",
	Usage:   "Get all examples by query params",
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
		&requestflag.Flag[*bool]{
			Name:      "descending",
			QueryPath: "descending",
		},
		&requestflag.Flag[*string]{
			Name:      "filter",
			QueryPath: "filter",
		},
		&requestflag.Flag[any]{
			Name:      "full-text-contain",
			QueryPath: "full_text_contains",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Default:   100,
			QueryPath: "limit",
		},
		&requestflag.Flag[*string]{
			Name:      "metadata",
			QueryPath: "metadata",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[string]{
			Name:      "order",
			Usage:     `Allowed values: "recent", "random", "recently_created", "id".`,
			Default:   "recent",
			QueryPath: "order",
		},
		&requestflag.Flag[*float64]{
			Name:      "random-seed",
			QueryPath: "random_seed",
		},
		&requestflag.Flag[[]string]{
			Name:      "select",
			Default:   []string{"id", "created_at", "modified_at", "name", "dataset_id", "source_run_id", "source_session_id", "source_run_start_time", "source_trace_id", "source_thread_id", "metadata", "inputs", "outputs"},
			QueryPath: "select",
		},
		&requestflag.Flag[any]{
			Name:      "split",
			QueryPath: "splits",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleExamplesList,
	HideHelpCommand: true,
}

var examplesDelete = cli.Command{
	Name:    "delete",
	Usage:   "Soft delete an example. Only deletes the example in the 'latest' version of the\ndataset.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "example-id",
			Required:  true,
			PathParam: "example_id",
		},
	},
	Action:          handleExamplesDelete,
	HideHelpCommand: true,
}

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

var examplesUploadFromCsv = cli.Command{
	Name:    "upload-from-csv",
	Usage:   "Upload examples from a CSV file.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[string]{
			Name:      "file",
			Required:  true,
			BodyPath:  "file",
			FileInput: true,
		},
		&requestflag.Flag[[]string]{
			Name:     "input-key",
			Required: true,
			BodyPath: "input_keys",
		},
		&requestflag.Flag[[]string]{
			Name:     "metadata-key",
			BodyPath: "metadata_keys",
		},
		&requestflag.Flag[[]string]{
			Name:     "output-key",
			BodyPath: "output_keys",
		},
	},
	Action:          handleExamplesUploadFromCsv,
	HideHelpCommand: true,
}

func handleExamplesCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.ExampleNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Examples.New(ctx, params, options...)
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
		Title:          "examples create",
		Transform:      transform,
	})
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

func handleExamplesList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.ExampleListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Examples.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "examples list",
			Transform:      transform,
		})
	} else {
		iter := client.Examples.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "examples list",
			Transform:      transform,
		})
	}
}

func handleExamplesDelete(ctx context.Context, cmd *cli.Command) error {
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

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Examples.Delete(ctx, cmd.Value("example-id").(string), options...)
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
		Title:          "examples delete",
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

func handleExamplesUploadFromCsv(ctx context.Context, cmd *cli.Command) error {
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
		MultipartFormEncoded,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.ExampleUploadFromCsvParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Examples.UploadFromCsv(
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
		Title:          "examples upload-from-csv",
		Transform:      transform,
	})
}
