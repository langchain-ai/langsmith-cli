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

var annotationQueuesRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Get Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
	},
	Action:          handleAnnotationQueuesRetrieve,
	HideHelpCommand: true,
}

var annotationQueuesUpdate = requestflag.WithInnerFlags(cli.Command{
	Name:    "update",
	Usage:   "Update Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[*string]{
			Name:     "default-dataset",
			BodyPath: "default_dataset",
		},
		&requestflag.Flag[*string]{
			Name:     "description",
			BodyPath: "description",
		},
		&requestflag.Flag[bool]{
			Name:     "enable-reservations",
			Default:  true,
			BodyPath: "enable_reservations",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "metadata",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "metadata",
		},
		&requestflag.Flag[*string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[any]{
			Name:     "num-reviewers-per-item",
			Default:  1,
			BodyPath: "num_reviewers_per_item",
		},
		&requestflag.Flag[*int64]{
			Name:     "reservation-minutes",
			BodyPath: "reservation_minutes",
		},
		&requestflag.Flag[*string]{
			Name:     "reviewer-access-mode",
			Usage:    `Allowed values: "any", "assigned".`,
			BodyPath: "reviewer_access_mode",
		},
		&requestflag.Flag[*string]{
			Name:     "rubric-instructions",
			BodyPath: "rubric_instructions",
		},
		&requestflag.Flag[any]{
			Name:     "rubric-item",
			BodyPath: "rubric_items",
		},
	},
	Action:          handleAnnotationQueuesUpdate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"rubric-item": {
		&requestflag.InnerFlag[string]{
			Name:                  "rubric-item.feedback-key",
			InnerField:            "feedback_key",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[*string]{
			Name:                  "rubric-item.description",
			InnerField:            "description",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[*bool]{
			Name:                  "rubric-item.is-assertion",
			InnerField:            "is_assertion",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[*bool]{
			Name:                  "rubric-item.is-required",
			InnerField:            "is_required",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[any]{
			Name:                  "rubric-item.regex-validator",
			InnerField:            "regex_validator",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:                  "rubric-item.score-descriptions",
			InnerField:            "score_descriptions",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:                  "rubric-item.value-descriptions",
			InnerField:            "value_descriptions",
			OuterIsArrayOfObjects: true,
		},
	},
})

var annotationQueuesDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
	},
	Action:          handleAnnotationQueuesDelete,
	HideHelpCommand: true,
}

var annotationQueuesAnnotationQueues = requestflag.WithInnerFlags(cli.Command{
	Name:    "annotation-queues",
	Usage:   "Create Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "name",
			Required: true,
			BodyPath: "name",
		},
		&requestflag.Flag[string]{
			Name:     "id",
			BodyPath: "id",
		},
		&requestflag.Flag[any]{
			Name:     "created-at",
			BodyPath: "created_at",
		},
		&requestflag.Flag[*string]{
			Name:     "default-dataset",
			BodyPath: "default_dataset",
		},
		&requestflag.Flag[*string]{
			Name:     "description",
			BodyPath: "description",
		},
		&requestflag.Flag[*bool]{
			Name:     "enable-reservations",
			Default:  requestflag.Ptr[bool](true),
			BodyPath: "enable_reservations",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "metadata",
			BodyPath: "metadata",
		},
		&requestflag.Flag[*int64]{
			Name:     "num-reviewers-per-item",
			Default:  requestflag.Ptr[int64](1),
			BodyPath: "num_reviewers_per_item",
		},
		&requestflag.Flag[*int64]{
			Name:     "reservation-minutes",
			Default:  requestflag.Ptr[int64](1),
			BodyPath: "reservation_minutes",
		},
		&requestflag.Flag[string]{
			Name:     "reviewer-access-mode",
			Default:  "any",
			BodyPath: "reviewer_access_mode",
		},
		&requestflag.Flag[*string]{
			Name:     "rubric-instructions",
			BodyPath: "rubric_instructions",
		},
		&requestflag.Flag[any]{
			Name:     "rubric-item",
			BodyPath: "rubric_items",
		},
		&requestflag.Flag[any]{
			Name:     "session-id",
			BodyPath: "session_ids",
		},
		&requestflag.Flag[any]{
			Name:     "updated-at",
			BodyPath: "updated_at",
		},
	},
	Action:          handleAnnotationQueuesAnnotationQueues,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"rubric-item": {
		&requestflag.InnerFlag[string]{
			Name:                  "rubric-item.feedback-key",
			InnerField:            "feedback_key",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[*string]{
			Name:                  "rubric-item.description",
			InnerField:            "description",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[*bool]{
			Name:                  "rubric-item.is-assertion",
			InnerField:            "is_assertion",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[*bool]{
			Name:                  "rubric-item.is-required",
			InnerField:            "is_required",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[any]{
			Name:                  "rubric-item.regex-validator",
			InnerField:            "regex_validator",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:                  "rubric-item.score-descriptions",
			InnerField:            "score_descriptions",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:                  "rubric-item.value-descriptions",
			InnerField:            "value_descriptions",
			OuterIsArrayOfObjects: true,
		},
	},
})

var annotationQueuesCreateRunStatus = cli.Command{
	Name:    "create-run-status",
	Usage:   "Create Identity Annotation Queue Run Status",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "annotation-queue-run-id",
			Required:  true,
			PathParam: "annotation_queue_run_id",
		},
		&requestflag.Flag[any]{
			Name:     "override-added-at",
			BodyPath: "override_added_at",
		},
		&requestflag.Flag[*string]{
			Name:     "status",
			BodyPath: "status",
		},
	},
	Action:          handleAnnotationQueuesCreateRunStatus,
	HideHelpCommand: true,
}

var annotationQueuesExport = cli.Command{
	Name:    "export",
	Usage:   "Export Annotation Queue Archived Runs",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[any]{
			Name:     "end-time",
			BodyPath: "end_time",
		},
		&requestflag.Flag[bool]{
			Name:     "include-annotator-detail",
			Default:  false,
			BodyPath: "include_annotator_detail",
		},
		&requestflag.Flag[any]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
	},
	Action:          handleAnnotationQueuesExport,
	HideHelpCommand: true,
}

var annotationQueuesPopulate = cli.Command{
	Name:    "populate",
	Usage:   "Populate annotation queue with runs from an experiment.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "queue-id",
			Required: true,
			BodyPath: "queue_id",
		},
		&requestflag.Flag[[]string]{
			Name:     "session-id",
			Required: true,
			BodyPath: "session_ids",
		},
		&requestflag.Flag[bool]{
			Name:     "extend-trace-retention",
			Default:  false,
			BodyPath: "extend_trace_retention",
		},
	},
	Action:          handleAnnotationQueuesPopulate,
	HideHelpCommand: true,
}

var annotationQueuesRetrieveAnnotationQueues = cli.Command{
	Name:    "retrieve-annotation-queues",
	Usage:   "Get Annotation Queues",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[bool]{
			Name:      "assigned-to-me",
			Default:   false,
			QueryPath: "assigned_to_me",
		},
		&requestflag.Flag[*string]{
			Name:      "dataset-id",
			QueryPath: "dataset_id",
		},
		&requestflag.Flag[any]{
			Name:      "id",
			QueryPath: "ids",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Default:   100,
			QueryPath: "limit",
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
		&requestflag.Flag[*string]{
			Name:      "queue-type",
			Usage:     `Allowed values: "single", "pairwise".`,
			QueryPath: "queue_type",
		},
		&requestflag.Flag[*string]{
			Name:      "sort-by",
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
	Action:          handleAnnotationQueuesRetrieveAnnotationQueues,
	HideHelpCommand: true,
}

var annotationQueuesRetrieveQueues = cli.Command{
	Name:    "retrieve-queues",
	Usage:   "Get Annotation Queues For Run",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "run-id",
			Required:  true,
			PathParam: "run_id",
		},
	},
	Action:          handleAnnotationQueuesRetrieveQueues,
	HideHelpCommand: true,
}

var annotationQueuesRetrieveRun = cli.Command{
	Name:    "retrieve-run",
	Usage:   "Get a run from an annotation queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[int64]{
			Name:      "index",
			Required:  true,
			PathParam: "index",
		},
		&requestflag.Flag[bool]{
			Name:      "include-extra",
			Default:   false,
			QueryPath: "include_extra",
		},
	},
	Action:          handleAnnotationQueuesRetrieveRun,
	HideHelpCommand: true,
}

var annotationQueuesRetrieveSize = cli.Command{
	Name:    "retrieve-size",
	Usage:   "Get Size From Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[*string]{
			Name:      "status",
			Usage:     `Allowed values: "needs_my_review", "needs_others_review", "completed".`,
			QueryPath: "status",
		},
	},
	Action:          handleAnnotationQueuesRetrieveSize,
	HideHelpCommand: true,
}

var annotationQueuesRetrieveTotalArchived = cli.Command{
	Name:    "retrieve-total-archived",
	Usage:   "Get Total Archived From Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[any]{
			Name:      "end-time",
			QueryPath: "end_time",
		},
		&requestflag.Flag[any]{
			Name:      "start-time",
			QueryPath: "start_time",
		},
	},
	Action:          handleAnnotationQueuesRetrieveTotalArchived,
	HideHelpCommand: true,
}

var annotationQueuesRetrieveTotalSize = cli.Command{
	Name:    "retrieve-total-size",
	Usage:   "Get Total Size From Annotation Queue",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
	},
	Action:          handleAnnotationQueuesRetrieveTotalSize,
	HideHelpCommand: true,
}

func handleAnnotationQueuesRetrieve(ctx context.Context, cmd *cli.Command) error {
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

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Get(ctx, cmd.Value("queue-id").(string), options...)
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
		Title:          "annotation-queues retrieve",
		Transform:      transform,
	})
}

func handleAnnotationQueuesUpdate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.AnnotationQueueUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Update(
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
		Title:          "annotation-queues update",
		Transform:      transform,
	})
}

func handleAnnotationQueuesDelete(ctx context.Context, cmd *cli.Command) error {
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

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Delete(ctx, cmd.Value("queue-id").(string), options...)
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
		Title:          "annotation-queues delete",
		Transform:      transform,
	})
}

func handleAnnotationQueuesAnnotationQueues(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.AnnotationQueueAnnotationQueuesParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.AnnotationQueues(ctx, params, options...)
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
		Title:          "annotation-queues annotation-queues",
		Transform:      transform,
	})
}

func handleAnnotationQueuesCreateRunStatus(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("annotation-queue-run-id") && len(unusedArgs) > 0 {
		cmd.Set("annotation-queue-run-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueNewRunStatusParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.NewRunStatus(
		ctx,
		cmd.Value("annotation-queue-run-id").(string),
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
		Title:          "annotation-queues create-run-status",
		Transform:      transform,
	})
}

func handleAnnotationQueuesExport(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.AnnotationQueueExportParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Export(
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
		Title:          "annotation-queues export",
		Transform:      transform,
	})
}

func handleAnnotationQueuesPopulate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.AnnotationQueuePopulateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Populate(ctx, params, options...)
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
		Title:          "annotation-queues populate",
		Transform:      transform,
	})
}

func handleAnnotationQueuesRetrieveAnnotationQueues(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.AnnotationQueueGetAnnotationQueuesParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.AnnotationQueues.GetAnnotationQueues(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "annotation-queues retrieve-annotation-queues",
			Transform:      transform,
		})
	} else {
		iter := client.AnnotationQueues.GetAnnotationQueuesAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "annotation-queues retrieve-annotation-queues",
			Transform:      transform,
		})
	}
}

func handleAnnotationQueuesRetrieveQueues(ctx context.Context, cmd *cli.Command) error {
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

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.GetQueues(ctx, cmd.Value("run-id").(string), options...)
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
		Title:          "annotation-queues retrieve-queues",
		Transform:      transform,
	})
}

func handleAnnotationQueuesRetrieveRun(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("index") && len(unusedArgs) > 0 {
		cmd.Set("index", unusedArgs[0])
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

	params := langsmith.AnnotationQueueGetRunParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.GetRun(
		ctx,
		cmd.Value("queue-id").(string),
		cmd.Value("index").(int64),
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
		Title:          "annotation-queues retrieve-run",
		Transform:      transform,
	})
}

func handleAnnotationQueuesRetrieveSize(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.AnnotationQueueGetSizeParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.GetSize(
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
		Title:          "annotation-queues retrieve-size",
		Transform:      transform,
	})
}

func handleAnnotationQueuesRetrieveTotalArchived(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.AnnotationQueueGetTotalArchivedParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.GetTotalArchived(
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
		Title:          "annotation-queues retrieve-total-archived",
		Transform:      transform,
	})
}

func handleAnnotationQueuesRetrieveTotalSize(ctx context.Context, cmd *cli.Command) error {
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

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.GetTotalSize(ctx, cmd.Value("queue-id").(string), options...)
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
		Title:          "annotation-queues retrieve-total-size",
		Transform:      transform,
	})
}
