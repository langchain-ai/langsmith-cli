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

var annotationQueuesRunsCreate = cli.Command{
	Name:    "create",
	Usage:   "Add Runs To Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[[]string]{
			Name:     "body",
			Required: true,
			BodyRoot: true,
		},
		&requestflag.Flag[bool]{
			Name:      "extend-trace-retention",
			Default:   false,
			QueryPath: "extend_trace_retention",
		},
	},
	Action:          handleAnnotationQueuesRunsCreate,
	HideHelpCommand: true,
}

var annotationQueuesRunsUpdate = cli.Command{
	Name:    "update",
	Usage:   "Update Run In Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[string]{
			Name:      "queue-run-id",
			Required:  true,
			PathParam: "queue_run_id",
		},
		&requestflag.Flag[any]{
			Name:     "added-at",
			BodyPath: "added_at",
		},
		&requestflag.Flag[any]{
			Name:     "last-reviewed-time",
			BodyPath: "last_reviewed_time",
		},
	},
	Action:          handleAnnotationQueuesRunsUpdate,
	HideHelpCommand: true,
}

var annotationQueuesRunsList = cli.Command{
	Name:    "list",
	Usage:   "Get Runs From Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[*bool]{
			Name:      "archived",
			QueryPath: "archived",
		},
		&requestflag.Flag[*bool]{
			Name:      "include-stats",
			QueryPath: "include_stats",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Default:   100,
			QueryPath: "limit",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[*string]{
			Name:      "status",
			Usage:     `Allowed values: "needs_my_review", "needs_others_review", "completed".`,
			QueryPath: "status",
		},
	},
	Action:          handleAnnotationQueuesRunsList,
	HideHelpCommand: true,
}

var annotationQueuesRunsCreateByKey = requestflag.WithInnerFlags(cli.Command{
	Name:    "create-by-key",
	Usage:   "Self-hosted deployments require LangSmith `v0.16` or later.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[[]map[string]any]{
			Name:     "body",
			Required: true,
			BodyRoot: true,
		},
		&requestflag.Flag[bool]{
			Name:      "extend-trace-retention",
			Default:   false,
			QueryPath: "extend_trace_retention",
		},
	},
	Action:          handleAnnotationQueuesRunsCreateByKey,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"body": {
		&requestflag.InnerFlag[string]{
			Name:       "body.run-id",
			InnerField: "run_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "body.session-id",
			InnerField: "session_id",
		},
		&requestflag.InnerFlag[any]{
			Name:       "body.start-time",
			InnerField: "start_time",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "body.source-proposed-example-id",
			InnerField: "source_proposed_example_id",
		},
	},
})

var annotationQueuesRunsDeleteAll = cli.Command{
	Name:    "delete-all",
	Usage:   "Delete Runs From Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[bool]{
			Name:     "delete-all",
			Default:  false,
			BodyPath: "delete_all",
		},
		&requestflag.Flag[any]{
			Name:     "exclude-run-id",
			BodyPath: "exclude_run_ids",
		},
		&requestflag.Flag[any]{
			Name:     "run-id",
			BodyPath: "run_ids",
		},
	},
	Action:          handleAnnotationQueuesRunsDeleteAll,
	HideHelpCommand: true,
}

var annotationQueuesRunsDeleteQueue = cli.Command{
	Name:    "delete-queue",
	Usage:   "Delete Run From Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[string]{
			Name:      "queue-run-id",
			Required:  true,
			PathParam: "queue_run_id",
		},
	},
	Action:          handleAnnotationQueuesRunsDeleteQueue,
	HideHelpCommand: true,
}

func handleAnnotationQueuesRunsCreate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueRunNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Runs.New(
		ctx,
		cmd.Value("queue-id").(string),
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
		Title:          "annotation-queues:runs create",
		Transform:      transform,
	})
}

func handleAnnotationQueuesRunsUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("queue-run-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-run-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueRunUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Runs.Update(
		ctx,
		cmd.Value("queue-id").(string),
		cmd.Value("queue-run-id").(string),
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
		Title:          "annotation-queues:runs update",
		Transform:      transform,
	})
}

func handleAnnotationQueuesRunsList(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueRunListParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Runs.List(
		ctx,
		cmd.Value("queue-id").(string),
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
		Title:          "annotation-queues:runs list",
		Transform:      transform,
	})
}

func handleAnnotationQueuesRunsCreateByKey(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueRunNewByKeyParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Runs.NewByKey(
		ctx,
		cmd.Value("queue-id").(string),
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
		Title:          "annotation-queues:runs create-by-key",
		Transform:      transform,
	})
}

func handleAnnotationQueuesRunsDeleteAll(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueRunDeleteAllParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Runs.DeleteAll(
		ctx,
		cmd.Value("queue-id").(string),
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
		Title:          "annotation-queues:runs delete-all",
		Transform:      transform,
	})
}

func handleAnnotationQueuesRunsDeleteQueue(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("queue-run-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-run-id", unusedArgs[0])
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
	_, err = client.AnnotationQueues.Runs.DeleteQueue(
		ctx,
		cmd.Value("queue-id").(string),
		cmd.Value("queue-run-id").(string),
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
		Title:          "annotation-queues:runs delete-queue",
		Transform:      transform,
	})
}
