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

var datasetsExamplesDelete = cli.Command{
	Name:    "delete",
	Usage:   "Soft-delete an example, preserving prior versions and their attachments. If the\nlatest version is already deleted, the request succeeds without creating another\nversion. Deletion is recorded at the current time or just after the latest\nversion, whichever is later. For future-dated versions, latest reads reflect\ndeletion immediately; timestamp reads reflect deletion only at or after the\nrecorded deletion timestamp.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Required:  true,
			PathParam: "dataset_id",
		},
		&requestflag.Flag[string]{
			Name:      "example-id",
			Required:  true,
			PathParam: "example_id",
		},
	},
	Action:          handleDatasetsExamplesDelete,
	HideHelpCommand: true,
}

func handleDatasetsExamplesDelete(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("dataset-id") && len(unusedArgs) > 0 {
		cmd.Set("dataset-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("example-id") && len(unusedArgs) > 0 {
		cmd.Set("example-id", unusedArgs[0])
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

	return client.Datasets.Examples.Delete(
		ctx,
		cmd.Value("dataset-id").(string),
		cmd.Value("example-id").(string),
		options...,
	)
}
