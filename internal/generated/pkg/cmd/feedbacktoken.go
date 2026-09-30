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

var feedbackTokensCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "Create a new feedback ingest token.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "feedback-key",
			BodyPath: "feedback_key",
		},
		&requestflag.Flag[string]{
			Name:     "run-id",
			BodyPath: "run_id",
		},
		&requestflag.Flag[any]{
			Name:     "expires-at",
			BodyPath: "expires_at",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "expires-in",
			Usage:    "Timedelta input.",
			BodyPath: "expires_in",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "feedback-config",
			BodyPath: "feedback_config",
		},
		&requestflag.Flag[[]map[string]any]{
			Name:     "body",
			BodyRoot: true,
		},
	},
	Action:          handleFeedbackTokensCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"expires-in": {
		&requestflag.InnerFlag[int64]{
			Name:       "expires-in.days",
			InnerField: "days",
		},
		&requestflag.InnerFlag[int64]{
			Name:       "expires-in.hours",
			InnerField: "hours",
		},
		&requestflag.InnerFlag[int64]{
			Name:       "expires-in.minutes",
			InnerField: "minutes",
		},
	},
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
	"body": {
		&requestflag.InnerFlag[string]{
			Name:       "body.feedback-key",
			InnerField: "feedback_key",
		},
		&requestflag.InnerFlag[string]{
			Name:       "body.run-id",
			InnerField: "run_id",
		},
		&requestflag.InnerFlag[any]{
			Name:       "body.expires-at",
			InnerField: "expires_at",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "body.expires-in",
			Usage:      "Timedelta input.",
			InnerField: "expires_in",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "body.feedback-config",
			InnerField: "feedback_config",
		},
	},
})

var feedbackTokensRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Create a new feedback with a token.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "token",
			Required:  true,
			PathParam: "token",
		},
		&requestflag.Flag[*string]{
			Name:      "comment",
			QueryPath: "comment",
		},
		&requestflag.Flag[*string]{
			Name:      "correction",
			QueryPath: "correction",
		},
		&requestflag.Flag[bool]{
			Name:      "extend-trace-retention",
			Default:   true,
			QueryPath: "extend_trace_retention",
		},
		&requestflag.Flag[any]{
			Name:      "score",
			QueryPath: "score",
		},
		&requestflag.Flag[any]{
			Name:      "value",
			QueryPath: "value",
		},
	},
	Action:          handleFeedbackTokensRetrieve,
	HideHelpCommand: true,
}

var feedbackTokensUpdate = cli.Command{
	Name:    "update",
	Usage:   "Create a new feedback with a token.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "token",
			Required:  true,
			PathParam: "token",
		},
		&requestflag.Flag[*string]{
			Name:     "comment",
			BodyPath: "comment",
		},
		&requestflag.Flag[any]{
			Name:     "correction",
			BodyPath: "correction",
		},
		&requestflag.Flag[bool]{
			Name:     "extend-trace-retention",
			Default:  true,
			BodyPath: "extend_trace_retention",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "metadata",
			BodyPath: "metadata",
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
	Action:          handleFeedbackTokensUpdate,
	HideHelpCommand: true,
}

var feedbackTokensList = cli.Command{
	Name:    "list",
	Usage:   "List all feedback ingest tokens for a run.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "run-id",
			Required:  true,
			QueryPath: "run_id",
		},
	},
	Action:          handleFeedbackTokensList,
	HideHelpCommand: true,
}

func handleFeedbackTokensCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.FeedbackTokenNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Feedback.Tokens.New(ctx, params, options...)
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
		Title:          "feedback:tokens create",
		Transform:      transform,
	})
}

func handleFeedbackTokensRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("token") && len(unusedArgs) > 0 {
		cmd.Set("token", unusedArgs[0])
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

	params := langsmith.FeedbackTokenGetParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Feedback.Tokens.Get(
		ctx,
		cmd.Value("token").(string),
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
		Title:          "feedback:tokens retrieve",
		Transform:      transform,
	})
}

func handleFeedbackTokensUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("token") && len(unusedArgs) > 0 {
		cmd.Set("token", unusedArgs[0])
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

	params := langsmith.FeedbackTokenUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Feedback.Tokens.Update(
		ctx,
		cmd.Value("token").(string),
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
		Title:          "feedback:tokens update",
		Transform:      transform,
	})
}

func handleFeedbackTokensList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.FeedbackTokenListParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Feedback.Tokens.List(ctx, params, options...)
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
		Title:          "feedback:tokens list",
		Transform:      transform,
	})
}
