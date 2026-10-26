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

var sessionsUpdate = cli.Command{
	Name:    "update",
	Usage:   "Update a project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
		&requestflag.Flag[*string]{
			Name:     "default-dataset-id",
			BodyPath: "default_dataset_id",
		},
		&requestflag.Flag[*string]{
			Name:     "description",
			BodyPath: "description",
		},
		&requestflag.Flag[any]{
			Name:     "end-time",
			BodyPath: "end_time",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "extra",
			BodyPath: "extra",
		},
		&requestflag.Flag[*string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[*string]{
			Name:     "trace-tier",
			Usage:    `Allowed values: "longlived", "shortlived".`,
			BodyPath: "trace_tier",
		},
	},
	Action:          handleSessionsUpdate,
	HideHelpCommand: true,
}

var sessionsDashboard = requestflag.WithInnerFlags(cli.Command{
	Name:    "dashboard",
	Usage:   "Get a prebuilt dashboard for a tracing project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
		&requestflag.Flag[any]{
			Name:     "end-time",
			BodyPath: "end_time",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "group-by",
			Usage:    "Group by param for run stats.",
			BodyPath: "group_by",
		},
		&requestflag.Flag[bool]{
			Name:     "omit-data",
			Default:  false,
			BodyPath: "omit_data",
		},
		&requestflag.Flag[any]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "stride",
			Usage:    "Timedelta input.",
			BodyPath: "stride",
		},
		&requestflag.Flag[string]{
			Name:     "timezone",
			Default:  "UTC",
			BodyPath: "timezone",
		},
		&requestflag.Flag[string]{
			Name:       "accept",
			HeaderPath: "accept",
		},
	},
	Action:          handleSessionsDashboard,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"group-by": {
		&requestflag.InnerFlag[string]{
			Name:       "group-by.attribute",
			Usage:      `Allowed values: "name", "run_type", "tag", "metadata".`,
			InnerField: "attribute",
		},
		&requestflag.InnerFlag[int64]{
			Name:       "group-by.max-groups",
			InnerField: "max_groups",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "group-by.path",
			InnerField: "path",
		},
	},
	"stride": {
		&requestflag.InnerFlag[int64]{
			Name:       "stride.days",
			InnerField: "days",
		},
		&requestflag.InnerFlag[int64]{
			Name:       "stride.hours",
			InnerField: "hours",
		},
		&requestflag.InnerFlag[int64]{
			Name:       "stride.minutes",
			InnerField: "minutes",
		},
	},
})

var sessionsResolve = cli.Command{
	Name:    "resolve",
	Usage:   "Returns the tracing project (session) an address names. An address is an AGENT\n(`id` and `environment`), an EXPERIMENT (`id`), or an EVALUATOR (no `id`:\nevaluator traces share one project per workspace). Send `kind` and `environment`\nin upper case, as listed; they are matched case-insensitively, while the Agent\n`id` is case-sensitive. An address that does not exist, or whose project you\ncannot read, is a 404. Pass the returned `session_id` to any endpoint that takes\na project (session) ID. This is not supported on a BYOC data plane yet, and is a\n501 there. Self-hosted deployments need LangSmith v0.18 or later.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "kind",
			Usage:     "The kind of address.",
			Required:  true,
			QueryPath: "kind",
		},
		&requestflag.Flag[string]{
			Name:      "id",
			Usage:     "The Agent's user-assigned id for AGENT, or the experiment's id for EXPERIMENT. Not set for EVALUATOR.",
			QueryPath: "id",
		},
		&requestflag.Flag[string]{
			Name:      "environment",
			Usage:     "The Agent environment. Only set for AGENT.",
			QueryPath: "environment",
		},
	},
	Action:          handleSessionsResolve,
	HideHelpCommand: true,
}

func handleSessionsUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("session-id") && len(unusedArgs) > 0 {
		cmd.Set("session-id", unusedArgs[0])
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

	params := langsmith.SessionUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sessions.Update(
		ctx,
		cmd.Value("session-id").(string),
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
		Title:          "sessions update",
		Transform:      transform,
	})
}

func handleSessionsDashboard(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("session-id") && len(unusedArgs) > 0 {
		cmd.Set("session-id", unusedArgs[0])
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

	params := langsmith.SessionDashboardParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sessions.Dashboard(
		ctx,
		cmd.Value("session-id").(string),
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
		Title:          "sessions dashboard",
		Transform:      transform,
	})
}

func handleSessionsResolve(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SessionResolveParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sessions.Resolve(ctx, params, options...)
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
		Title:          "sessions resolve",
		Transform:      transform,
	})
}
