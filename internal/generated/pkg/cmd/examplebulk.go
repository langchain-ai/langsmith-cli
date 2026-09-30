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

var examplesBulkCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "Create bulk examples.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[[]map[string]any]{
			Name:     "body",
			Usage:    "Schema for a batch of examples to be created.",
			Required: true,
			BodyRoot: true,
		},
	},
	Action:          handleExamplesBulkCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"body": {
		&requestflag.InnerFlag[string]{
			Name:       "body.dataset-id",
			InnerField: "dataset_id",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "body.id",
			InnerField: "id",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "body.created-at",
			InnerField: "created_at",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "body.inputs",
			InnerField: "inputs",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "body.metadata",
			InnerField: "metadata",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "body.outputs",
			InnerField: "outputs",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "body.source-run-id",
			InnerField: "source_run_id",
		},
		&requestflag.InnerFlag[any]{
			Name:       "body.source-run-start-time",
			InnerField: "source_run_start_time",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "body.source-session-id",
			InnerField: "source_session_id",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "body.source-trace-id",
			InnerField: "source_trace_id",
		},
		&requestflag.InnerFlag[any]{
			Name:       "body.split",
			InnerField: "split",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "body.use-legacy-message-format",
			Usage:      "Use Legacy Message Format for LLM runs",
			InnerField: "use_legacy_message_format",
		},
		&requestflag.InnerFlag[[]string]{
			Name:       "body.use-source-run-attachments",
			InnerField: "use_source_run_attachments",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "body.use-source-run-io",
			InnerField: "use_source_run_io",
		},
	},
})

var examplesBulkPatchAll = requestflag.WithInnerFlags(cli.Command{
	Name:    "patch-all",
	Usage:   "Legacy update examples in bulk. For update involving attachments, use PATCH\n/v1/platform/datasets/{dataset_id}/examples instead.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[[]map[string]any]{
			Name:     "body",
			Required: true,
			BodyRoot: true,
		},
	},
	Action:          handleExamplesBulkPatchAll,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"body": {
		&requestflag.InnerFlag[string]{
			Name:       "body.id",
			InnerField: "id",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "body.attachments-operations",
			InnerField: "attachments_operations",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "body.dataset-id",
			InnerField: "dataset_id",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "body.inputs",
			InnerField: "inputs",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "body.metadata",
			InnerField: "metadata",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "body.outputs",
			InnerField: "outputs",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "body.overwrite",
			InnerField: "overwrite",
		},
		&requestflag.InnerFlag[any]{
			Name:       "body.split",
			InnerField: "split",
		},
	},
})

func handleExamplesBulkCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.ExampleBulkNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Examples.Bulk.New(ctx, params, options...)
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
		Title:          "examples:bulk create",
		Transform:      transform,
	})
}

func handleExamplesBulkPatchAll(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.ExampleBulkPatchAllParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Examples.Bulk.PatchAll(ctx, params, options...)
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
		Title:          "examples:bulk patch-all",
		Transform:      transform,
	})
}
