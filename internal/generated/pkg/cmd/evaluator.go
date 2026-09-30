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

var evaluatorsList = cli.Command{
	Name:    "list",
	Usage:   "List all run rules.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[any]{
			Name:      "id",
			QueryPath: "id",
		},
		&requestflag.Flag[*string]{
			Name:      "dataset-id",
			QueryPath: "dataset_id",
		},
		&requestflag.Flag[*string]{
			Name:      "evaluator-id",
			QueryPath: "evaluator_id",
		},
		&requestflag.Flag[bool]{
			Name:      "include-backfill-progress",
			Default:   false,
			QueryPath: "include_backfill_progress",
		},
		&requestflag.Flag[*string]{
			Name:      "name-contains",
			QueryPath: "name_contains",
		},
		&requestflag.Flag[*string]{
			Name:      "session-id",
			QueryPath: "session_id",
		},
		&requestflag.Flag[any]{
			Name:      "tag-value-id",
			QueryPath: "tag_value_id",
		},
		&requestflag.Flag[*string]{
			Name:      "type",
			Usage:     `Allowed values: "session", "dataset".`,
			QueryPath: "type",
		},
	},
	Action:          handleEvaluatorsList,
	HideHelpCommand: true,
}

func handleEvaluatorsList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.EvaluatorListParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Evaluators.List(ctx, params, options...)
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
		Title:          "evaluators list",
		Transform:      transform,
	})
}
