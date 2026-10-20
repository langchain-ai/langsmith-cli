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

var threadsShareCreate = cli.Command{
	Name:    "create",
	Usage:   "Mints a public share token for a thread. Idempotent: sharing an already-shared\nthread returns the existing token.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "thread-id",
			Required:  true,
			PathParam: "thread_id",
		},
		&requestflag.Flag[string]{
			Name:     "project-id",
			Usage:    "project_id is the tracing project UUID containing the thread.",
			Required: true,
			BodyPath: "project_id",
		},
	},
	Action:          handleThreadsShareCreate,
	HideHelpCommand: true,
}

var threadsShareRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Returns the share token for a thread. The token is omitted when the thread is\nnot shared. Gated on runs:share so the control's state matches the control's\npermission.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "thread-id",
			Required:  true,
			PathParam: "thread_id",
		},
		&requestflag.Flag[string]{
			Name:      "project-id",
			Usage:     "Project UUID",
			Required:  true,
			QueryPath: "project_id",
		},
	},
	Action:          handleThreadsShareRetrieve,
	HideHelpCommand: true,
}

var threadsShareDelete = cli.Command{
	Name:    "delete",
	Usage:   "Deletes the share token for a thread. Idempotent: returns 204 whether or not a\nshare token existed. Deliberately does not verify the thread still exists.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "thread-id",
			Required:  true,
			PathParam: "thread_id",
		},
		&requestflag.Flag[string]{
			Name:      "project-id",
			Usage:     "Project UUID",
			Required:  true,
			QueryPath: "project_id",
		},
	},
	Action:          handleThreadsShareDelete,
	HideHelpCommand: true,
}

func handleThreadsShareCreate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("thread-id") && len(unusedArgs) > 0 {
		cmd.Set("thread-id", unusedArgs[0])
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

	params := langsmith.ThreadShareNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Threads.Share.New(
		ctx,
		cmd.Value("thread-id").(string),
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
		Title:          "threads:share create",
		Transform:      transform,
	})
}

func handleThreadsShareRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("thread-id") && len(unusedArgs) > 0 {
		cmd.Set("thread-id", unusedArgs[0])
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

	params := langsmith.ThreadShareGetParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Threads.Share.Get(
		ctx,
		cmd.Value("thread-id").(string),
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
		Title:          "threads:share retrieve",
		Transform:      transform,
	})
}

func handleThreadsShareDelete(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("thread-id") && len(unusedArgs) > 0 {
		cmd.Set("thread-id", unusedArgs[0])
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

	params := langsmith.ThreadShareDeleteParams{}

	return client.Threads.Share.Delete(
		ctx,
		cmd.Value("thread-id").(string),
		params,
		options...,
	)
}
