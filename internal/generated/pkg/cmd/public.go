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

var publicRetrieveFeedbacks = cli.Command{
	Name:    "retrieve-feedbacks",
	Usage:   "Read Shared Feedbacks",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "share-token",
			Required:  true,
			PathParam: "share_token",
		},
		&requestflag.Flag[*bool]{
			Name:      "has-comment",
			QueryPath: "has_comment",
		},
		&requestflag.Flag[*bool]{
			Name:      "has-score",
			QueryPath: "has_score",
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
	Action:          handlePublicRetrieveFeedbacks,
	HideHelpCommand: true,
}

func handlePublicRetrieveFeedbacks(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("share-token") && len(unusedArgs) > 0 {
		cmd.Set("share-token", unusedArgs[0])
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

	params := langsmith.PublicGetFeedbacksParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Public.GetFeedbacks(
			ctx,
			cmd.Value("share-token").(string),
			params,
			options...,
		)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "public retrieve-feedbacks",
			Transform:      transform,
		})
	} else {
		iter := client.Public.GetFeedbacksAutoPaging(
			ctx,
			cmd.Value("share-token").(string),
			params,
			options...,
		)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "public retrieve-feedbacks",
			Transform:      transform,
		})
	}
}
