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

var runsShareCreate = cli.Command{
	Name:    "create",
	Usage:   "Creates or returns a share token for a run. Child runs share their trace root.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "run-id",
			Required:  true,
			PathParam: "run_id",
		},
		&requestflag.Flag[string]{
			Name:     "session-id",
			Usage:    "session_id is the tracing project UUID containing the trace.",
			BodyPath: "session_id",
		},
		&requestflag.Flag[string]{
			Name:     "trace-id",
			Usage:    "trace_id is the root trace UUID to share.",
			BodyPath: "trace_id",
		},
	},
	Action:          handleRunsShareCreate,
	HideHelpCommand: true,
}

var runsShareDelete = cli.Command{
	Name:    "delete",
	Usage:   "Deletes the share token for the trace identified by trace_id and session_id.\nIdempotent: returns 204 whether or not a share token existed.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "trace-id",
			Required:  true,
			PathParam: "trace_id",
		},
		&requestflag.Flag[string]{
			Name:     "session-id",
			Usage:    "session_id is the tracing project UUID containing the trace.",
			BodyPath: "session_id",
		},
	},
	Action:          handleRunsShareDelete,
	HideHelpCommand: true,
}

func handleRunsShareCreate(ctx context.Context, cmd *cli.Command) error {
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
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunShareNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.Share.New(
		ctx,
		cmd.Value("run-id").(string),
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
		Title:          "runs:share create",
		Transform:      transform,
	})
}

func handleRunsShareDelete(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("trace-id") && len(unusedArgs) > 0 {
		cmd.Set("trace-id", unusedArgs[0])
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

	params := langsmith.RunShareDeleteParams{}

	return client.Runs.Share.Delete(
		ctx,
		cmd.Value("trace-id").(string),
		params,
		options...,
	)
}
