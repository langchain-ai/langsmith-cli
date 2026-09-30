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
			Required: true,
			BodyPath: "key",
		},
		&requestflag.Flag[string]{
			Name:     "id",
			BodyPath: "id",
		},
		&requestflag.Flag[*string]{
			Name:     "agent-environment",
			Usage:    "Experimental. Only supported in workspaces where Agent addressing is enabled; other workspaces get a 403. The Agent environment whose tracing project the feedback belongs to. Matched case-insensitively. Sent together with agent_id.",
			BodyPath: "agent_environment",
		},
		&requestflag.Flag[*string]{
			Name:     "agent-id",
			Usage:    "Experimental. Only supported in workspaces where Agent addressing is enabled; other workspaces get a 403. The Agent's id, not a UUID: 1 to 63 lowercase ASCII letters, digits, or hyphens, starting with a letter and ending with a letter or digit (e.g. support-agent). Addresses the tracing project through an Agent instead of session_id. Sent together with agent_environment, and never alongside session_id. The Agent and the environment must already exist; sending feedback does not create them.",
			BodyPath: "agent_id",
		},
		&requestflag.Flag[*string]{
			Name:     "comment",
			BodyPath: "comment",
		},
		&requestflag.Flag[*string]{
			Name:     "comparative-experiment-id",
			BodyPath: "comparative_experiment_id",
		},
		&requestflag.Flag[any]{
			Name:     "correction",
			BodyPath: "correction",
		},
		&requestflag.Flag[any]{
			Name:     "created-at",
			BodyPath: "created_at",
		},
		&requestflag.Flag[*bool]{
			Name:     "error",
			Usage:    "Deprecated. Use `extra.error` instead. If both values are provided, `error` takes precedence.",
			BodyPath: "error",
		},
		&requestflag.Flag[bool]{
			Name:     "extend-trace-retention",
			Default:  true,
			BodyPath: "extend_trace_retention",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "extra",
			BodyPath: "extra",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "feedback-config",
			BodyPath: "feedback_config",
		},
		&requestflag.Flag[*string]{
			Name:     "feedback-group-id",
			BodyPath: "feedback_group_id",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "feedback-source",
			Usage:    "Feedback from the LangChainPlus App.",
			BodyPath: "feedback_source",
		},
		&requestflag.Flag[*string]{
			Name:     "feedback-thread-id",
			BodyPath: "feedback_thread_id",
		},
		&requestflag.Flag[any]{
			Name:     "modified-at",
			BodyPath: "modified_at",
		},
		&requestflag.Flag[*string]{
			Name:     "run-id",
			BodyPath: "run_id",
		},
		&requestflag.Flag[any]{
			Name:     "score",
			BodyPath: "score",
		},
		&requestflag.Flag[*string]{
			Name:     "session-id",
			Usage:    "Required unless the feedback is addressed by agent_id and agent_environment. The ID of the tracing project (session) the feedback belongs to.",
			BodyPath: "session_id",
		},
		&requestflag.Flag[any]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
		&requestflag.Flag[*string]{
			Name:     "trace-id",
			BodyPath: "trace_id",
		},
		&requestflag.Flag[any]{
			Name:     "value",
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
	Usage:   "List all Feedback by query params.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[*string]{
			Name:      "comparative-experiment-id",
			QueryPath: "comparative_experiment_id",
		},
		&requestflag.Flag[*string]{
			Name:      "feedback-thread-id",
			QueryPath: "feedback_thread_id",
		},
		&requestflag.Flag[*bool]{
			Name:      "has-comment",
			QueryPath: "has_comment",
		},
		&requestflag.Flag[*bool]{
			Name:      "has-score",
			QueryPath: "has_score",
		},
		&requestflag.Flag[*bool]{
			Name:      "include-user-names",
			QueryPath: "include_user_names",
		},
		&requestflag.Flag[any]{
			Name:      "key",
			QueryPath: "key",
		},
		&requestflag.Flag[*string]{
			Name:      "level",
			Usage:     "Enum for feedback levels.",
			QueryPath: "level",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Default:   100,
			QueryPath: "limit",
		},
		&requestflag.Flag[any]{
			Name:      "max-created-at",
			QueryPath: "max_created_at",
		},
		&requestflag.Flag[any]{
			Name:      "min-created-at",
			QueryPath: "min_created_at",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[any]{
			Name:      "run",
			QueryPath: "run",
		},
		&requestflag.Flag[any]{
			Name:      "session",
			QueryPath: "session",
		},
		&requestflag.Flag[any]{
			Name:      "source",
			QueryPath: "source",
		},
		&requestflag.Flag[any]{
			Name:      "user",
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
