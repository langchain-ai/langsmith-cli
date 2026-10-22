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
