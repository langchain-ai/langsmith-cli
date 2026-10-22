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

var sessionsInsightsConfigsCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "Create an Insights job configuration for a project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "config",
			Usage:    "Configuration for an Insights job.",
			Required: true,
			BodyPath: "config",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			Required: true,
			BodyPath: "name",
		},
		&requestflag.Flag[*string]{
			Name:     "description",
			BodyPath: "description",
		},
		&requestflag.Flag[*string]{
			Name:     "schedule-cron",
			BodyPath: "schedule_cron",
		},
	},
	Action:          handleSessionsInsightsConfigsCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"config": {
		&requestflag.InnerFlag[map[string]any]{
			Name:       "config.attribute-schemas",
			InnerField: "attribute_schemas",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "config.cluster-model",
			InnerField: "cluster_model",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "config.config-id",
			InnerField: "config_id",
		},
		&requestflag.InnerFlag[any]{
			Name:       "config.end-time",
			InnerField: "end_time",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "config.filter",
			InnerField: "filter",
		},
		&requestflag.InnerFlag[any]{
			Name:       "config.hierarchy",
			InnerField: "hierarchy",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "config.is-scheduled",
			InnerField: "is_scheduled",
		},
		&requestflag.InnerFlag[*int64]{
			Name:       "config.last-n-hours",
			InnerField: "last_n_hours",
		},
		&requestflag.InnerFlag[string]{
			Name:       "config.model",
			Usage:      `Allowed values: "openai", "anthropic".`,
			InnerField: "model",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "config.name",
			InnerField: "name",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "config.partitions",
			InnerField: "partitions",
		},
		&requestflag.InnerFlag[*float64]{
			Name:       "config.sample",
			InnerField: "sample",
		},
		&requestflag.InnerFlag[any]{
			Name:       "config.start-time",
			InnerField: "start_time",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "config.summary-model",
			InnerField: "summary_model",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "config.summary-prompt",
			InnerField: "summary_prompt",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "config.user-context",
			InnerField: "user_context",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "config.validate-model-secrets",
			InnerField: "validate_model_secrets",
		},
	},
})

func handleSessionsInsightsConfigsCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SessionInsightConfigNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sessions.Insights.Configs.New(
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
		Title:          "sessions:insights:configs create",
		Transform:      transform,
	})
}
