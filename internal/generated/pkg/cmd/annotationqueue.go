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
			Usage:     "ID of the annotation queue.",
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
			Usage:     "ID of the annotation queue.",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[*string]{
			Name:     "default-dataset",
			Usage:    "Dataset that reviewed runs are added to by default.",
			BodyPath: "default_dataset",
		},
		&requestflag.Flag[*string]{
			Name:     "description",
			Usage:    "New description.",
			BodyPath: "description",
		},
		&requestflag.Flag[any]{
			Name:     "enable-reservations",
			Usage:    "Whether a reviewer reserves a run while reviewing it, so others skip it.",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "enable_reservations",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "metadata",
			Usage:    "Metadata to store on the queue. Replaces the existing metadata.",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "metadata",
		},
		&requestflag.Flag[*string]{
			Name:     "name",
			Usage:    "New name for the queue.",
			BodyPath: "name",
		},
		&requestflag.Flag[any]{
			Name:     "num-reviewers-per-item",
			Usage:    "How many reviewers must review each run.",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "num_reviewers_per_item",
		},
		&requestflag.Flag[any]{
			Name:     "reservation-minutes",
			Usage:    "How long a reservation lasts, in minutes.",
			Default:  map[string]interface{}{"__missing__": "__missing__"},
			BodyPath: "reservation_minutes",
		},
		&requestflag.Flag[*string]{
			Name:     "reviewer-access-mode",
			Usage:    "Who completes a run: any (one reviewer is enough) or assigned (every assigned reviewer must submit).",
			BodyPath: "reviewer_access_mode",
		},
		&requestflag.Flag[*string]{
			Name:     "rubric-instructions",
			Usage:    "Instructions shown to reviewers.",
			BodyPath: "rubric_instructions",
		},
		&requestflag.Flag[any]{
			Name:     "rubric-item",
			Usage:    "Feedback keys reviewers fill in for each run.",
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
		&requestflag.InnerFlag[map[string]any]{
			Name:                  "rubric-item.feedback-config",
			InnerField:            "feedback_config",
			OuterIsArrayOfObjects: true,
		},
		&requestflag.InnerFlag[*string]{
			Name:                  "rubric-item.feedback-config-id",
			InnerField:            "feedback_config_id",
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
			Name:                  "rubric-item.required-if",
			InnerField:            "required_if",
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
		&requestflag.InnerFlag[map[string]any]{
			Name:                  "rubric-item.visible-if",
			InnerField:            "visible_if",
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
			Usage:     "ID of the annotation queue.",
			Required:  true,
			PathParam: "queue_id",
		},
	},
	Action:          handleAnnotationQueuesDelete,
	HideHelpCommand: true,
}

var annotationQueuesPopulate = cli.Command{
	Name:    "populate",
	Usage:   "Populate annotation queue with runs from an experiment.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "queue-id",
			Usage:    "ID of the annotation queue to add runs to.",
			Required: true,
			BodyPath: "queue_id",
		},
		&requestflag.Flag[[]string]{
			Name:     "session-id",
			Usage:    "Experiments whose runs are added to the queue. Each must be an experiment on a dataset.",
			Required: true,
			BodyPath: "session_ids",
		},
		&requestflag.Flag[bool]{
			Name:     "extend-trace-retention",
			Usage:    "Upgrade the added traces from short-lived to extended retention.",
			Default:  false,
			BodyPath: "extend_trace_retention",
		},
	},
	Action:          handleAnnotationQueuesPopulate,
	HideHelpCommand: true,
}

var annotationQueuesRetrieveQueues = cli.Command{
	Name:    "retrieve-queues",
	Usage:   "Get Annotation Queues For Run",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "run-id",
			Usage:     "ID of the run.",
			Required:  true,
			PathParam: "run_id",
		},
	},
	Action:          handleAnnotationQueuesRetrieveQueues,
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
