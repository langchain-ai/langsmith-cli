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

var datasetsCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "Create a new dataset.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "name",
			Required: true,
			BodyPath: "name",
		},
		&requestflag.Flag[*string]{
			Name:     "id",
			BodyPath: "id",
		},
		&requestflag.Flag[any]{
			Name:     "created-at",
			BodyPath: "created_at",
		},
		&requestflag.Flag[string]{
			Name:     "data-type",
			Usage:    "Enum for dataset data types.",
			BodyPath: "data_type",
		},
		&requestflag.Flag[*string]{
			Name:     "description",
			BodyPath: "description",
		},
		&requestflag.Flag[*bool]{
			Name:     "externally-managed",
			Default:  requestflag.Ptr[bool](false),
			BodyPath: "externally_managed",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "extra",
			BodyPath: "extra",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "inputs-schema-definition",
			BodyPath: "inputs_schema_definition",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "outputs-schema-definition",
			BodyPath: "outputs_schema_definition",
		},
		&requestflag.Flag[any]{
			Name:     "tag-value-id",
			BodyPath: "tag_value_ids",
		},
		&requestflag.Flag[any]{
			Name:     "transformation",
			BodyPath: "transformations",
		},
	},
	Action:          handleDatasetsCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"transformation": {
		&requestflag.InnerFlag[[]string]{
			Name:                  "transformation.path",
			InnerField:            "path",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[string]{
			Name:                  "transformation.transformation-type",
			Usage:                 "Enum for dataset transformation types.\nOrdering determines the order in which transformations are applied if there are multiple transformations on the same path.",
			InnerField:            "transformation_type",
			OuterIsArrayOfObjects: true,
		},
	},
})

var datasetsRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Get a specific dataset.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
	},
	Action:          handleDatasetsRetrieve,
	HideHelpCommand: true,
}

var datasetsUpdate = cli.Command{
	Name:    "update",
	Usage:   "Update a specific dataset.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[any]{
			Name:     "baseline-experiment-id",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "baseline_experiment_id",
		},
		&requestflag.Flag[any]{
			Name:     "description",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "description",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "inputs-schema-definition",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "inputs_schema_definition",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "metadata",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "metadata",
		},
		&requestflag.Flag[any]{
			Name:     "name",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "name",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "outputs-schema-definition",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "outputs_schema_definition",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "patch-examples",
			BodyPath: "patch_examples",
		},
		&requestflag.Flag[any]{
			Name:     "transformations",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "transformations",
		},
	},
	Action:          handleDatasetsUpdate,
	HideHelpCommand: true,
}

var datasetsList = cli.Command{
	Name:    "list",
	Usage:   "Get all datasets by query params and owner.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[any]{
			Name:      "id",
			QueryPath: "id",
		},
		&requestflag.Flag[any]{
			Name:      "datatype",
			Usage:     "Enum for dataset data types.",
			QueryPath: "data_type",
		},
		&requestflag.Flag[any]{
			Name:      "exclude",
			QueryPath: "exclude",
		},
		&requestflag.Flag[bool]{
			Name:      "exclude-corrections-datasets",
			Default:   false,
			QueryPath: "exclude_corrections_datasets",
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
		&requestflag.Flag[*string]{
			Name:      "name",
			QueryPath: "name",
		},
		&requestflag.Flag[*string]{
			Name:      "name-contains",
			QueryPath: "name_contains",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[string]{
			Name:      "sort-by",
			Usage:     "Enum for available dataset columns to sort by.",
			QueryPath: "sort_by",
		},
		&requestflag.Flag[bool]{
			Name:      "sort-by-desc",
			Default:   true,
			QueryPath: "sort_by_desc",
		},
		&requestflag.Flag[any]{
			Name:      "tag-value-id",
			QueryPath: "tag_value_id",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleDatasetsList,
	HideHelpCommand: true,
}

var datasetsDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete a specific dataset.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
	},
	Action:          handleDatasetsDelete,
	HideHelpCommand: true,
}

var datasetsClone = cli.Command{
	Name:    "clone",
	Usage:   "Clone a dataset.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "source-dataset-id",
			Required: true,
			BodyPath: "source_dataset_id",
		},
		&requestflag.Flag[string]{
			Name:     "target-dataset-id",
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
			Default:  []string{},
			BodyPath: "examples",
		},
		&requestflag.Flag[any]{
			Name:     "split",
			BodyPath: "split",
		},
		&requestflag.Flag[any]{
			Name:     "tag-value-id",
			BodyPath: "tag_value_ids",
		},
	},
	Action:          handleDatasetsClone,
	HideHelpCommand: true,
}

var datasetsRetrieveCsv = cli.Command{
	Name:    "retrieve-csv",
	Usage:   "Download a dataset as CSV format.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[any]{
			Name:      "as-of",
			Usage:     "Only modifications made on or before this time are included. If None, the latest version of the dataset is used.",
			QueryPath: "as_of",
		},
	},
	Action:          handleDatasetsRetrieveCsv,
	HideHelpCommand: true,
}

var datasetsRetrieveJSONL = cli.Command{
	Name:    "retrieve-jsonl",
	Usage:   "Download a dataset as CSV format.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[any]{
			Name:      "as-of",
			Usage:     "Only modifications made on or before this time are included. If None, the latest version of the dataset is used.",
			QueryPath: "as_of",
		},
	},
	Action:          handleDatasetsRetrieveJSONL,
	HideHelpCommand: true,
}

var datasetsRetrieveOpenAI = cli.Command{
	Name:    "retrieve-openai",
	Usage:   "Download a dataset as OpenAI Evals Jsonl format.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[any]{
			Name:      "as-of",
			Usage:     "Only modifications made on or before this time are included. If None, the latest version of the dataset is used.",
			QueryPath: "as_of",
		},
	},
	Action:          handleDatasetsRetrieveOpenAI,
	HideHelpCommand: true,
}

var datasetsRetrieveOpenAIFt = cli.Command{
	Name:    "retrieve-openai-ft",
	Usage:   "Download a dataset as OpenAI Jsonl format.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[any]{
			Name:      "as-of",
			Usage:     "Only modifications made on or before this time are included. If None, the latest version of the dataset is used.",
			QueryPath: "as_of",
		},
	},
	Action:          handleDatasetsRetrieveOpenAIFt,
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
	Usage:   "Set a tag on a dataset version.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[any]{
			Name:     "as-of",
			Usage:    "Only modifications made on or before this time are included. If None, the latest version of the dataset is used.",
			Required: true,
			BodyPath: "as_of",
		},
		&requestflag.Flag[string]{
			Name:     "tag",
			Required: true,
			BodyPath: "tag",
		},
	},
	Action:          handleDatasetsUpdateTags,
	HideHelpCommand: true,
}

var datasetsUpload = cli.Command{
	Name:    "upload",
	Usage:   "Create a new dataset from a CSV or JSONL file.",
	Suggest: true,
	Flags: []cli.Flag{
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
		&requestflag.Flag[string]{
			Name:     "data-type",
			Usage:    "Enum for dataset data types.",
			BodyPath: "data_type",
		},
		&requestflag.Flag[*string]{
			Name:     "description",
			BodyPath: "description",
		},
		&requestflag.Flag[*string]{
			Name:     "input-key-mappings",
			BodyPath: "input_key_mappings",
		},
		&requestflag.Flag[*string]{
			Name:     "inputs-schema-definition",
			BodyPath: "inputs_schema_definition",
		},
		&requestflag.Flag[*string]{
			Name:     "metadata-key-mappings",
			BodyPath: "metadata_key_mappings",
		},
		&requestflag.Flag[[]string]{
			Name:     "metadata-key",
			Default:  []string{},
			BodyPath: "metadata_keys",
		},
		&requestflag.Flag[*string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[*string]{
			Name:     "output-key-mappings",
			BodyPath: "output_key_mappings",
		},
		&requestflag.Flag[[]string]{
			Name:     "output-key",
			Default:  []string{},
			BodyPath: "output_keys",
		},
		&requestflag.Flag[*string]{
			Name:     "outputs-schema-definition",
			BodyPath: "outputs_schema_definition",
		},
		&requestflag.Flag[*string]{
			Name:     "tag-value-ids",
			BodyPath: "tag_value_ids",
		},
		&requestflag.Flag[*string]{
			Name:     "transformations",
			BodyPath: "transformations",
		},
	},
	Action:          handleDatasetsUpload,
	HideHelpCommand: true,
}

func handleDatasetsCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.New(ctx, params, options...)
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
		Title:          "datasets create",
		Transform:      transform,
	})
}

func handleDatasetsRetrieve(ctx context.Context, cmd *cli.Command) error {
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

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.Get(ctx, cmd.Value("dataset-id").(string), options...)
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
		Title:          "datasets retrieve",
		Transform:      transform,
	})
}

func handleDatasetsUpdate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.Update(
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
		Title:          "datasets update",
		Transform:      transform,
	})
}

func handleDatasetsList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Datasets.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "datasets list",
			Transform:      transform,
		})
	} else {
		iter := client.Datasets.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "datasets list",
			Transform:      transform,
		})
	}
}

func handleDatasetsDelete(ctx context.Context, cmd *cli.Command) error {
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

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.Delete(ctx, cmd.Value("dataset-id").(string), options...)
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
		Title:          "datasets delete",
		Transform:      transform,
	})
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

func handleDatasetsRetrieveCsv(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetGetCsvParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.GetCsv(
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
		Title:          "datasets retrieve-csv",
		Transform:      transform,
	})
}

func handleDatasetsRetrieveJSONL(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetGetJSONLParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.GetJSONL(
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
		Title:          "datasets retrieve-jsonl",
		Transform:      transform,
	})
}

func handleDatasetsRetrieveOpenAI(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetGetOpenAIParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.GetOpenAI(
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
		Title:          "datasets retrieve-openai",
		Transform:      transform,
	})
}

func handleDatasetsRetrieveOpenAIFt(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.DatasetGetOpenAIFtParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.GetOpenAIFt(
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
		Title:          "datasets retrieve-openai-ft",
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

func handleDatasetsUpload(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

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

	params := langsmith.DatasetUploadParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Datasets.Upload(ctx, params, options...)
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
		Title:          "datasets upload",
		Transform:      transform,
	})
}
