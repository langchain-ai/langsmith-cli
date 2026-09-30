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

var publicRunsRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Returns one run within the trace identified by the share token. The request\nsupplies only the run ID and that run's exact start_time coordinate.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "share-token",
			Required:  true,
			PathParam: "share_token",
		},
		&requestflag.Flag[string]{
			Name:      "run-id",
			Required:  true,
			PathParam: "run_id",
		},
		&requestflag.Flag[[]string]{
			Name:      "select",
			Usage:     "repeatable public run fields to include",
			Required:  true,
			QueryPath: "selects",
		},
		&requestflag.Flag[any]{
			Name:      "start-time",
			Usage:     "Run start_time coordinate (RFC3339)",
			Required:  true,
			QueryPath: "start_time",
		},
		&requestflag.Flag[string]{
			Name:       "accept",
			HeaderPath: "Accept",
		},
	},
	Action:          handlePublicRunsRetrieve,
	HideHelpCommand: true,
}

var publicRunsQuery = cli.Command{
	Name:    "query",
	Usage:   "Returns all runs within the trace identified by the share token. The share token\nsupplies the tenant, project, and trace scope.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "share-token",
			Required:  true,
			PathParam: "share_token",
		},
		&requestflag.Flag[[]string]{
			Name:     "select",
			Usage:    "`selects` lists which public run properties to include on each returned run.",
			BodyPath: "selects",
		},
		&requestflag.Flag[string]{
			Name:       "accept",
			HeaderPath: "Accept",
		},
	},
	Action:          handlePublicRunsQuery,
	HideHelpCommand: true,
}

func handlePublicRunsRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("share-token") && len(unusedArgs) > 0 {
		cmd.Set("share-token", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
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
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.PublicRunGetParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Public.Runs.Get(
		ctx,
		cmd.Value("share-token").(string),
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
		Title:          "public:runs retrieve",
		Transform:      transform,
	})
}

func handlePublicRunsQuery(ctx context.Context, cmd *cli.Command) error {
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
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.PublicRunQueryParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Public.Runs.Query(
		ctx,
		cmd.Value("share-token").(string),
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
		Title:          "public:runs query",
		Transform:      transform,
	})
}
