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

var feedbackCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "Create a new feedback.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "key",
			Usage:    "The feedback key: the metric, tag, or aspect being scored, for example correctness.",
			Required: true,
			BodyPath: "key",
		},
		&requestflag.Flag[string]{
			Name:     "id",
			Usage:    "The ID to give the feedback. Defaults to a new random UUID.",
			BodyPath: "id",
		},
		&requestflag.Flag[string]{
			Name:     "address",
			Usage:    "Addresses the tracing project through an Agent environment, as lrn:agents/{id}/environments/{environment}, instead of session_id. The id is 1 to 63 lowercase ASCII letters, digits, or hyphens, starting with a letter and ending with a letter or digit. The environment is local, development, staging, or production, matched case-insensitively. Never combined with session_id. The Agent and the environment must already exist; sending feedback does not create them. Self-hosted deployments need LangSmith v0.18 or later.",
			BodyPath: "address",
		},
		&requestflag.Flag[*string]{
			Name:     "comment",
			Usage:    "A free-text comment explaining the feedback. Truncated at the feedback size limit.",
			BodyPath: "comment",
		},
		&requestflag.Flag[*string]{
			Name:     "comparative-experiment-id",
			Usage:    "The ID of the comparative (pairwise) experiment this feedback was written for.",
			BodyPath: "comparative_experiment_id",
		},
		&requestflag.Flag[any]{
			Name:     "correction",
			Usage:    "The corrected output for the run, as a string or a JSON object. Dropped if it exceeds the feedback size limit.",
			BodyPath: "correction",
		},
		&requestflag.Flag[any]{
			Name:     "created-at",
			Usage:    "When the feedback was created, as an ISO 8601 timestamp. Defaults to now.",
			BodyPath: "created_at",
		},
		&requestflag.Flag[*bool]{
			Name:     "error",
			Usage:    "Deprecated. Use extra.error instead. If both values are provided, error takes precedence.",
			BodyPath: "error",
		},
		&requestflag.Flag[bool]{
			Name:     "extend-trace-retention",
			Usage:    "Whether writing this feedback may upgrade the trace to extended retention. Defaults to true.",
			Default:  true,
			BodyPath: "extend_trace_retention",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "extra",
			Usage:    "Additional metadata to store with the feedback, as a JSON object.",
			BodyPath: "extra",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "feedback-config",
			Usage:    "The scale the score is checked against. If the feedback key already has a feedback config, this must match it exactly or the request is rejected; if it has none, this becomes the key's feedback config.",
			BodyPath: "feedback_config",
		},
		&requestflag.Flag[*string]{
			Name:     "feedback-config-id",
			Usage:    "The ID of the feedback config this feedback was written against. It must be a feedback config for this feedback key in the same workspace, or the feedback is rejected. It doesn't change how the score is checked; feedback_config does that. If omitted, the server may record the key's enforced feedback config, or the one created for the key by this request.",
			BodyPath: "feedback_config_id",
		},
		&requestflag.Flag[*string]{
			Name:     "feedback-group-id",
			Usage:    "Groups feedback written together, such as one comparison in a pairwise experiment.",
			BodyPath: "feedback_group_id",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "feedback-source",
			Usage:    "Where the feedback came from, as an object whose type is api, app, model, or auto_eval.",
			BodyPath: "feedback_source",
		},
		&requestflag.Flag[*string]{
			Name:     "feedback-thread-id",
			Usage:    "The ID of the thread the feedback is about. Requires session_id (or address) and cannot be combined with run_id.",
			BodyPath: "feedback_thread_id",
		},
		&requestflag.Flag[any]{
			Name:     "modified-at",
			Usage:    "When the feedback was last modified, as an ISO 8601 timestamp. Defaults to now.",
			BodyPath: "modified_at",
		},
		&requestflag.Flag[*string]{
			Name:     "run-id",
			Usage:    "The ID of the run the feedback is about. Omit it for feedback on a whole tracing project or a thread.",
			BodyPath: "run_id",
		},
		&requestflag.Flag[any]{
			Name:     "score",
			Usage:    "The numeric or boolean score. Numbers must be between -99999.9999 and 99999.9999 with at most 4 decimal places.",
			BodyPath: "score",
		},
		&requestflag.Flag[*string]{
			Name:     "session-id",
			Usage:    "Required unless the feedback is addressed by address. The ID of the tracing project (session) the feedback belongs to.",
			BodyPath: "session_id",
		},
		&requestflag.Flag[any]{
			Name:     "start-time",
			Usage:    "The start time of run_id, as an ISO 8601 timestamp. With session_id and trace_id, it lets the server write run feedback without looking the run up.",
			BodyPath: "start_time",
		},
		&requestflag.Flag[*string]{
			Name:     "trace-id",
			Usage:    "The ID of the trace that contains run_id. With session_id and start_time, it lets the server write run feedback without looking the run up.",
			BodyPath: "trace_id",
		},
		&requestflag.Flag[any]{
			Name:     "value",
			Usage:    "A non-numeric value, such as a category label or a JSON object. Dropped if it exceeds the feedback size limit.",
			BodyPath: "value",
		},
	},
	Action:          handleFeedbackCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"feedback-config": {
		&requestflag.InnerFlag[string]{
			Name:       "feedback-config.type",
			Usage:      "Enum for feedback types.",
			InnerField: "type",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "feedback-config.allows-multiple-categories",
			Usage:      "Whether a reviewer can pick several categories. Categorical feedback only.",
			InnerField: "allows_multiple_categories",
		},
		&requestflag.InnerFlag[any]{
			Name:       "feedback-config.categories",
			InnerField: "categories",
		},
		&requestflag.InnerFlag[*float64]{
			Name:       "feedback-config.max",
			InnerField: "max",
		},
		&requestflag.InnerFlag[*float64]{
			Name:       "feedback-config.min",
			InnerField: "min",
		},
	},
})

var feedbackRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Get a specific feedback.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "feedback-id",
			Required:  true,
			PathParam: "feedback_id",
		},
		&requestflag.Flag[*bool]{
			Name:      "include-user-names",
			QueryPath: "include_user_names",
		},
	},
	Action:          handleFeedbackRetrieve,
	HideHelpCommand: true,
}

var feedbackUpdate = requestflag.WithInnerFlags(cli.Command{
	Name:    "update",
	Usage:   "Replace an existing feedback entry with a new, modified entry.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "feedback-id",
			Required:  true,
			PathParam: "feedback_id",
		},
		&requestflag.Flag[*string]{
			Name:     "comment",
			BodyPath: "comment",
		},
		&requestflag.Flag[any]{
			Name:     "correction",
			BodyPath: "correction",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "feedback-config",
			BodyPath: "feedback_config",
		},
		&requestflag.Flag[any]{
			Name:     "score",
			BodyPath: "score",
		},
		&requestflag.Flag[any]{
			Name:     "value",
			BodyPath: "value",
		},
	},
	Action:          handleFeedbackUpdate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"feedback-config": {
		&requestflag.InnerFlag[string]{
			Name:       "feedback-config.type",
			Usage:      "Enum for feedback types.",
			InnerField: "type",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "feedback-config.allows-multiple-categories",
			Usage:      "Whether a reviewer can pick several categories. Categorical feedback only.",
			InnerField: "allows_multiple_categories",
		},
		&requestflag.InnerFlag[any]{
			Name:       "feedback-config.categories",
			InnerField: "categories",
		},
		&requestflag.InnerFlag[*float64]{
			Name:       "feedback-config.max",
			InnerField: "max",
		},
		&requestflag.InnerFlag[*float64]{
			Name:       "feedback-config.min",
			InnerField: "min",
		},
	},
})

var feedbackList = cli.Command{
	Name:    "list",
	Usage:   "List feedback, newest first.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[*string]{
			Name:      "comparative-experiment-id",
			Usage:     "Only feedback written for this comparative (pairwise) experiment.",
			QueryPath: "comparative_experiment_id",
		},
		&requestflag.Flag[*string]{
			Name:      "feedback-thread-id",
			Usage:     "Only feedback on this thread. Requires exactly one session and cannot be combined with run.",
			QueryPath: "feedback_thread_id",
		},
		&requestflag.Flag[*bool]{
			Name:      "has-comment",
			Usage:     "true returns only feedback with a non-empty comment; false only feedback without one.",
			QueryPath: "has_comment",
		},
		&requestflag.Flag[*bool]{
			Name:      "has-score",
			Usage:     "true returns only feedback with a score; false only feedback without one.",
			QueryPath: "has_score",
		},
		&requestflag.Flag[*bool]{
			Name:      "include-user-names",
			Usage:     "Whether to add the author's name to each feedback's source.",
			QueryPath: "include_user_names",
		},
		&requestflag.Flag[any]{
			Name:      "key",
			Usage:     "Only feedback with these keys.",
			QueryPath: "key",
		},
		&requestflag.Flag[*string]{
			Name:      "level",
			Usage:     "Feedback level: run for feedback on a run, session for feedback on a whole tracing project (session level requires a session filter).",
			QueryPath: "level",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Usage:     "The maximum number of feedback to return, 1 to 100.",
			Default:   100,
			QueryPath: "limit",
		},
		&requestflag.Flag[any]{
			Name:      "max-created-at",
			Usage:     "Only feedback created at or before this ISO 8601 time.",
			QueryPath: "max_created_at",
		},
		&requestflag.Flag[any]{
			Name:      "min-created-at",
			Usage:     "Only feedback created at or after this ISO 8601 time.",
			QueryPath: "min_created_at",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Usage:     "The number of feedback to skip, for paging.",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[any]{
			Name:      "run",
			Usage:     "Only feedback on these run IDs.",
			QueryPath: "run",
		},
		&requestflag.Flag[any]{
			Name:      "session",
			Usage:     "Only feedback in these tracing project (session) IDs.",
			QueryPath: "session",
		},
		&requestflag.Flag[any]{
			Name:      "source",
			Usage:     "Only feedback from these source types: api, model, app, or auto_eval.",
			QueryPath: "source",
		},
		&requestflag.Flag[any]{
			Name:      "user",
			Usage:     "Only feedback written by these user IDs.",
			QueryPath: "user",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleFeedbackList,
	HideHelpCommand: true,
}

var feedbackDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete a feedback.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "feedback-id",
			Required:  true,
			PathParam: "feedback_id",
		},
	},
	Action:          handleFeedbackDelete,
	HideHelpCommand: true,
}

func handleFeedbackCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.FeedbackNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Feedback.New(ctx, params, options...)
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
		Title:          "feedback create",
		Transform:      transform,
	})
}

func handleFeedbackRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("feedback-id") && len(unusedArgs) > 0 {
		cmd.Set("feedback-id", unusedArgs[0])
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

	params := langsmith.FeedbackGetParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Feedback.Get(
		ctx,
		cmd.Value("feedback-id").(string),
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
		Title:          "feedback retrieve",
		Transform:      transform,
	})
}

func handleFeedbackUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("feedback-id") && len(unusedArgs) > 0 {
		cmd.Set("feedback-id", unusedArgs[0])
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

	params := langsmith.FeedbackUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Feedback.Update(
		ctx,
		cmd.Value("feedback-id").(string),
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
		Title:          "feedback update",
		Transform:      transform,
	})
}

func handleFeedbackList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.FeedbackListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Feedback.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "feedback list",
			Transform:      transform,
		})
	} else {
		iter := client.Feedback.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "feedback list",
			Transform:      transform,
		})
	}
}

func handleFeedbackDelete(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("feedback-id") && len(unusedArgs) > 0 {
		cmd.Set("feedback-id", unusedArgs[0])
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
	_, err = client.Feedback.Delete(ctx, cmd.Value("feedback-id").(string), options...)
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
		Title:          "feedback delete",
		Transform:      transform,
	})
}
