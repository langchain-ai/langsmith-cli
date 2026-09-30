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

var sessionsInsightsCreate = cli.Command{
	Name:    "create",
	Usage:   "Create an Insights job for a project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "attribute-schemas",
			BodyPath: "attribute_schemas",
		},
		&requestflag.Flag[*string]{
			Name:     "cluster-model",
			BodyPath: "cluster_model",
		},
		&requestflag.Flag[*string]{
			Name:     "config-id",
			BodyPath: "config_id",
		},
		&requestflag.Flag[any]{
			Name:     "end-time",
			BodyPath: "end_time",
		},
		&requestflag.Flag[*string]{
			Name:     "filter",
			BodyPath: "filter",
		},
		&requestflag.Flag[any]{
			Name:     "hierarchy",
			BodyPath: "hierarchy",
		},
		&requestflag.Flag[bool]{
			Name:     "is-scheduled",
			Default:  false,
			BodyPath: "is_scheduled",
		},
		&requestflag.Flag[*int64]{
			Name:     "last-n-hours",
			BodyPath: "last_n_hours",
		},
		&requestflag.Flag[string]{
			Name:     "model",
			Usage:    `Allowed values: "openai", "anthropic".`,
			Default:  "openai",
			BodyPath: "model",
		},
		&requestflag.Flag[*string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "partitions",
			BodyPath: "partitions",
		},
		&requestflag.Flag[*float64]{
			Name:     "sample",
			BodyPath: "sample",
		},
		&requestflag.Flag[any]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
		&requestflag.Flag[*string]{
			Name:     "summary-model",
			BodyPath: "summary_model",
		},
		&requestflag.Flag[*string]{
			Name:     "summary-prompt",
			BodyPath: "summary_prompt",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "user-context",
			BodyPath: "user_context",
		},
		&requestflag.Flag[bool]{
			Name:     "validate-model-secrets",
			Default:  true,
			BodyPath: "validate_model_secrets",
		},
	},
	Action:          handleSessionsInsightsCreate,
	HideHelpCommand: true,
}

var sessionsInsightsUpdate = cli.Command{
	Name:    "update",
	Usage:   "Update an Insights job for a project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
		&requestflag.Flag[string]{
			Name:      "job-id",
			Required:  true,
			PathParam: "job_id",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			Required: true,
			BodyPath: "name",
		},
	},
	Action:          handleSessionsInsightsUpdate,
	HideHelpCommand: true,
}

var sessionsInsightsList = cli.Command{
	Name:    "list",
	Usage:   "List Insights jobs for a project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
		&requestflag.Flag[*string]{
			Name:      "config-id",
			QueryPath: "config_id",
		},
		&requestflag.Flag[*bool]{
			Name:      "legacy",
			QueryPath: "legacy",
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
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleSessionsInsightsList,
	HideHelpCommand: true,
}

var sessionsInsightsDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete an Insights job for a project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
		&requestflag.Flag[string]{
			Name:      "job-id",
			Required:  true,
			PathParam: "job_id",
		},
	},
	Action:          handleSessionsInsightsDelete,
	HideHelpCommand: true,
}

var sessionsInsightsRetrieveJob = cli.Command{
	Name:    "retrieve-job",
	Usage:   "Get an Insights job for a project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
		&requestflag.Flag[string]{
			Name:      "job-id",
			Required:  true,
			PathParam: "job_id",
		},
	},
	Action:          handleSessionsInsightsRetrieveJob,
	HideHelpCommand: true,
}

var sessionsInsightsRetrieveRuns = cli.Command{
	Name:    "retrieve-runs",
	Usage:   "List runs analyzed by an Insights job, optionally filtered by report cluster.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
		&requestflag.Flag[string]{
			Name:      "job-id",
			Required:  true,
			PathParam: "job_id",
		},
		&requestflag.Flag[*string]{
			Name:      "attribute-sort-key",
			QueryPath: "attribute_sort_key",
		},
		&requestflag.Flag[*string]{
			Name:      "attribute-sort-order",
			Usage:     `Allowed values: "asc", "desc".`,
			QueryPath: "attribute_sort_order",
		},
		&requestflag.Flag[*string]{
			Name:      "cluster-id",
			QueryPath: "cluster_id",
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
	},
	Action:          handleSessionsInsightsRetrieveRuns,
	HideHelpCommand: true,
}

func handleSessionsInsightsCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SessionInsightNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sessions.Insights.New(
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
		Title:          "sessions:insights create",
		Transform:      transform,
	})
}

func handleSessionsInsightsUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("session-id") && len(unusedArgs) > 0 {
		cmd.Set("session-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("job-id") && len(unusedArgs) > 0 {
		cmd.Set("job-id", unusedArgs[0])
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

	params := langsmith.SessionInsightUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sessions.Insights.Update(
		ctx,
		cmd.Value("session-id").(string),
		cmd.Value("job-id").(string),
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
		Title:          "sessions:insights update",
		Transform:      transform,
	})
}

func handleSessionsInsightsList(ctx context.Context, cmd *cli.Command) error {
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
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.SessionInsightListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Sessions.Insights.List(
			ctx,
			cmd.Value("session-id").(string),
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
			Title:          "sessions:insights list",
			Transform:      transform,
		})
	} else {
		iter := client.Sessions.Insights.ListAutoPaging(
			ctx,
			cmd.Value("session-id").(string),
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
			Title:          "sessions:insights list",
			Transform:      transform,
		})
	}
}

func handleSessionsInsightsDelete(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("session-id") && len(unusedArgs) > 0 {
		cmd.Set("session-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("job-id") && len(unusedArgs) > 0 {
		cmd.Set("job-id", unusedArgs[0])
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
	_, err = client.Sessions.Insights.Delete(
		ctx,
		cmd.Value("session-id").(string),
		cmd.Value("job-id").(string),
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
		Title:          "sessions:insights delete",
		Transform:      transform,
	})
}

func handleSessionsInsightsRetrieveJob(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("session-id") && len(unusedArgs) > 0 {
		cmd.Set("session-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("job-id") && len(unusedArgs) > 0 {
		cmd.Set("job-id", unusedArgs[0])
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
	_, err = client.Sessions.Insights.GetJob(
		ctx,
		cmd.Value("session-id").(string),
		cmd.Value("job-id").(string),
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
		Title:          "sessions:insights retrieve-job",
		Transform:      transform,
	})
}

func handleSessionsInsightsRetrieveRuns(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("session-id") && len(unusedArgs) > 0 {
		cmd.Set("session-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("job-id") && len(unusedArgs) > 0 {
		cmd.Set("job-id", unusedArgs[0])
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

	params := langsmith.SessionInsightGetRunsParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sessions.Insights.GetRuns(
		ctx,
		cmd.Value("session-id").(string),
		cmd.Value("job-id").(string),
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
		Title:          "sessions:insights retrieve-runs",
		Transform:      transform,
	})
}
