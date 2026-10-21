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

var runsCreate = cli.Command{
	Name:    "create",
	Usage:   "Queues a single run for ingestion. The request body must be a JSON-encoded run\nobject that follows the Run schema.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "id",
			BodyPath: "id",
		},
		&requestflag.Flag[string]{
			Name:     "address",
			Usage:    "Beta. Addresses the run to an Agent environment in place of session_id or\nsession_name, as lrn:agents/{id}/environments/{environment}. The environment is\ncase-insensitive. Only workspaces enabled for Agent addressing accept it;\nothers get a 403.",
			BodyPath: "address",
		},
		&requestflag.Flag[string]{
			Name:     "dotted-order",
			BodyPath: "dotted_order",
		},
		&requestflag.Flag[string]{
			Name:     "end-time",
			BodyPath: "end_time",
		},
		&requestflag.Flag[string]{
			Name:     "error",
			BodyPath: "error",
		},
		&requestflag.Flag[[]map[string]any]{
			Name:     "event",
			BodyPath: "events",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "extra",
			BodyPath: "extra",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "input-attachments",
			BodyPath: "input_attachments",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "inputs",
			BodyPath: "inputs",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "output-attachments",
			BodyPath: "output_attachments",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "outputs",
			BodyPath: "outputs",
		},
		&requestflag.Flag[string]{
			Name:     "parent-run-id",
			BodyPath: "parent_run_id",
		},
		&requestflag.Flag[string]{
			Name:     "reference-example-id",
			BodyPath: "reference_example_id",
		},
		&requestflag.Flag[string]{
			Name:     "run-type",
			Usage:    `Allowed values: "tool", "chain", "llm", "retriever", "embedding", "prompt", "parser".`,
			BodyPath: "run_type",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "serialized",
			BodyPath: "serialized",
		},
		&requestflag.Flag[string]{
			Name:     "session-id",
			BodyPath: "session_id",
		},
		&requestflag.Flag[string]{
			Name:     "session-name",
			BodyPath: "session_name",
		},
		&requestflag.Flag[string]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
		&requestflag.Flag[string]{
			Name:     "status",
			BodyPath: "status",
		},
		&requestflag.Flag[[]string]{
			Name:     "tag",
			BodyPath: "tags",
		},
		&requestflag.Flag[string]{
			Name:     "trace-id",
			BodyPath: "trace_id",
		},
	},
	Action:          handleRunsCreate,
	HideHelpCommand: true,
}

var runsGetURL = cli.Command{
	Name:    "get-url",
	Usage:   "Returns the URL to view a specific run in the LangSmith UI. The caller must\nsupply the run's project_id and trace_id as query parameters; start_time is\noptional.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "run-id",
			Required:  true,
			PathParam: "run_id",
		},
		&requestflag.Flag[string]{
			Name:      "project-id",
			Usage:     "Project (session) UUID",
			Required:  true,
			QueryPath: "project_id",
		},
		&requestflag.Flag[string]{
			Name:      "trace-id",
			Usage:     "Trace UUID",
			Required:  true,
			QueryPath: "trace_id",
		},
		&requestflag.Flag[string]{
			Name:      "start-time",
			Usage:     "Run start time in RFC3339 format; omit if unknown",
			QueryPath: "start_time",
		},
	},
	Action:          handleRunsGetURL,
	HideHelpCommand: true,
}

var runsIngestBatch = requestflag.WithInnerFlags(cli.Command{
	Name:    "ingest-batch",
	Usage:   "Ingests a batch of runs in a single JSON payload. The payload must have `post`\nand/or `patch` arrays containing run objects. Prefer this endpoint over\nsingle‑run ingestion when submitting hundreds of runs, but `/runs/multipart`\noffers better handling for very large fields and attachments.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[[]map[string]any]{
			Name:     "patch",
			BodyPath: "patch",
		},
		&requestflag.Flag[[]map[string]any]{
			Name:     "post",
			BodyPath: "post",
		},
	},
	Action:          handleRunsIngestBatch,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"patch": {
		&requestflag.InnerFlag[string]{
			Name:       "patch.id",
			InnerField: "id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.address",
			Usage:      "Beta. Addresses the run to an Agent environment in place of session_id or\nsession_name, as lrn:agents/{id}/environments/{environment}. The environment is\ncase-insensitive. Only workspaces enabled for Agent addressing accept it;\nothers get a 403.",
			InnerField: "address",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.dotted-order",
			InnerField: "dotted_order",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.end-time",
			InnerField: "end_time",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.error",
			InnerField: "error",
		},
		&requestflag.InnerFlag[[]map[string]any]{
			Name:       "patch.events",
			InnerField: "events",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.extra",
			InnerField: "extra",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.input-attachments",
			InnerField: "input_attachments",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.inputs",
			InnerField: "inputs",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.name",
			InnerField: "name",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.output-attachments",
			InnerField: "output_attachments",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.outputs",
			InnerField: "outputs",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.parent-run-id",
			InnerField: "parent_run_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.reference-example-id",
			InnerField: "reference_example_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.run-type",
			Usage:      `Allowed values: "tool", "chain", "llm", "retriever", "embedding", "prompt", "parser".`,
			InnerField: "run_type",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.serialized",
			InnerField: "serialized",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.session-id",
			InnerField: "session_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.session-name",
			InnerField: "session_name",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.start-time",
			InnerField: "start_time",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.status",
			InnerField: "status",
		},
		&requestflag.InnerFlag[[]string]{
			Name:       "patch.tags",
			InnerField: "tags",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.trace-id",
			InnerField: "trace_id",
		},
	},
	"post": {
		&requestflag.InnerFlag[string]{
			Name:       "post.id",
			InnerField: "id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.address",
			Usage:      "Beta. Addresses the run to an Agent environment in place of session_id or\nsession_name, as lrn:agents/{id}/environments/{environment}. The environment is\ncase-insensitive. Only workspaces enabled for Agent addressing accept it;\nothers get a 403.",
			InnerField: "address",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.dotted-order",
			InnerField: "dotted_order",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.end-time",
			InnerField: "end_time",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.error",
			InnerField: "error",
		},
		&requestflag.InnerFlag[[]map[string]any]{
			Name:       "post.events",
			InnerField: "events",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.extra",
			InnerField: "extra",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.input-attachments",
			InnerField: "input_attachments",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.inputs",
			InnerField: "inputs",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.name",
			InnerField: "name",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.output-attachments",
			InnerField: "output_attachments",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.outputs",
			InnerField: "outputs",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.parent-run-id",
			InnerField: "parent_run_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.reference-example-id",
			InnerField: "reference_example_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.run-type",
			Usage:      `Allowed values: "tool", "chain", "llm", "retriever", "embedding", "prompt", "parser".`,
			InnerField: "run_type",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.serialized",
			InnerField: "serialized",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.session-id",
			InnerField: "session_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.session-name",
			InnerField: "session_name",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.start-time",
			InnerField: "start_time",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.status",
			InnerField: "status",
		},
		&requestflag.InnerFlag[[]string]{
			Name:       "post.tags",
			InnerField: "tags",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.trace-id",
			InnerField: "trace_id",
		},
	},
})

func handleRunsCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.RunNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.New(ctx, params, options...)
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
		Title:          "runs create",
		Transform:      transform,
	})
}

func handleRunsGetURL(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("run-id") && len(unusedArgs) > 0 {
		cmd.Set("run-id", unusedArgs[0])
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

	params := langsmith.RunGetURLParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.GetURL(
		ctx,
		cmd.Value("run-id").(string),
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
		Title:          "runs get-url",
		Transform:      transform,
	})
}

func handleRunsIngestBatch(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.RunIngestBatchParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.IngestBatch(ctx, params, options...)
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
		Title:          "runs ingest-batch",
		Transform:      transform,
	})
}
