// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/apiquery"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
	"github.com/langchain-ai/langsmith-go"
	"github.com/urfave/cli/v3"
)

var feedbackConfigsDelete = cli.Command{
	Name:    "delete",
	Usage:   "Soft delete a feedback config by marking it as deleted.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "feedback-key",
			Required:  true,
			QueryPath: "feedback_key",
		},
	},
	Action:          handleFeedbackConfigsDelete,
	HideHelpCommand: true,
}

func handleFeedbackConfigsDelete(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.FeedbackConfigDeleteParams{}

	return client.Feedback.Configs.Delete(ctx, params, options...)
}
