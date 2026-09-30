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

var sessionsCreate = cli.Command{
	Name:    "create",
	Usage:   "Create a new project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[bool]{
			Name:      "upsert",
			Default:   false,
			QueryPath: "upsert",
		},
		&requestflag.Flag[*string]{
			Name:     "id",
			BodyPath: "id",
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
		&requestflag.Flag[any]{
			Name:     "evaluator-key",
			BodyPath: "evaluator_keys",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "extra",
			BodyPath: "extra",
		},
		&requestflag.Flag[*string]{
			Name:     "kicked-off-by",
			BodyPath: "kicked_off_by",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[*int64]{
			Name:     "num-examples",
			BodyPath: "num_examples",
		},
		&requestflag.Flag[*int64]{
			Name:     "num-repetitions",
			BodyPath: "num_repetitions",
		},
		&requestflag.Flag[*string]{
			Name:     "reference-dataset-id",
			BodyPath: "reference_dataset_id",
		},
		&requestflag.Flag[any]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
		&requestflag.Flag[any]{
			Name:     "tag-value-id",
			BodyPath: "tag_value_ids",
		},
		&requestflag.Flag[*string]{
			Name:     "trace-tier",
			Usage:    `Allowed values: "longlived", "shortlived".`,
			BodyPath: "trace_tier",
		},
	},
	Action:          handleSessionsCreate,
	HideHelpCommand: true,
}

var sessionsRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Get a specific project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
		&requestflag.Flag[bool]{
			Name:      "include-stats",
			Default:   false,
			QueryPath: "include_stats",
		},
		&requestflag.Flag[any]{
			Name:      "stats-start-time",
			QueryPath: "stats_start_time",
		},
		&requestflag.Flag[string]{
			Name:       "accept",
			HeaderPath: "accept",
		},
	},
	Action:          handleSessionsRetrieve,
	HideHelpCommand: true,
}

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

var sessionsList = cli.Command{
	Name:    "list",
	Usage:   "List all projects.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[any]{
			Name:      "id",
			QueryPath: "id",
		},
		&requestflag.Flag[*string]{
			Name:      "dataset-version",
			QueryPath: "dataset_version",
		},
		&requestflag.Flag[bool]{
			Name:      "facets",
			Default:   false,
			QueryPath: "facets",
		},
		&requestflag.Flag[*string]{
			Name:      "filter",
			QueryPath: "filter",
		},
		&requestflag.Flag[bool]{
			Name:      "include-stats",
			Default:   false,
			QueryPath: "include_stats",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Default:   100,
			QueryPath: "limit",
		},
		&requestflag.Flag[*string]{
			Name:      "metadata",
			QueryPath: "metadata",
		},
		&requestflag.Flag[*string]{
			Name:      "name",
			QueryPath: "name",
		},
		&requestflag.Flag[*string]{
			Name:      "name-contains",
			QueryPath: "name_contains",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[any]{
			Name:      "reference-dataset",
			QueryPath: "reference_dataset",
		},
		&requestflag.Flag[*bool]{
			Name:      "reference-free",
			QueryPath: "reference_free",
		},
		&requestflag.Flag[string]{
			Name:      "sort-by",
			Usage:     `Allowed values: "name", "start_time", "last_run_start_time", "latency_p50", "latency_p99", "error_rate", "feedback".`,
			QueryPath: "sort_by",
		},
		&requestflag.Flag[bool]{
			Name:      "sort-by-desc",
			Default:   true,
			QueryPath: "sort_by_desc",
		},
		&requestflag.Flag[*string]{
			Name:      "sort-by-feedback-key",
			QueryPath: "sort_by_feedback_key",
		},
		&requestflag.Flag[*string]{
			Name:      "sort-by-feedback-source",
			Usage:     `Allowed values: "session", "run".`,
			QueryPath: "sort_by_feedback_source",
		},
		&requestflag.Flag[*string]{
			Name:      "stats-filter",
			QueryPath: "stats_filter",
		},
		&requestflag.Flag[any]{
			Name:      "stats-select",
			QueryPath: "stats_select",
		},
		&requestflag.Flag[any]{
			Name:      "stats-start-time",
			QueryPath: "stats_start_time",
		},
		&requestflag.Flag[any]{
			Name:      "tag-value-id",
			QueryPath: "tag_value_id",
		},
		&requestflag.Flag[bool]{
			Name:      "use-approx-stats",
			Default:   false,
			QueryPath: "use_approx_stats",
		},
		&requestflag.Flag[string]{
			Name:       "accept",
			HeaderPath: "accept",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleSessionsList,
	HideHelpCommand: true,
}

var sessionsDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete a specific project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "session-id",
			Required:  true,
			PathParam: "session_id",
		},
	},
	Action:          handleSessionsDelete,
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

func handleSessionsCreate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

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

	params := langsmith.SessionNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sessions.New(ctx, params, options...)
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
		Title:          "sessions create",
		Transform:      transform,
	})
}

func handleSessionsRetrieve(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SessionGetParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sessions.Get(
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
		Title:          "sessions retrieve",
		Transform:      transform,
	})
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

func handleSessionsList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SessionListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Sessions.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "sessions list",
			Transform:      transform,
		})
	} else {
		iter := client.Sessions.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "sessions list",
			Transform:      transform,
		})
	}
}

func handleSessionsDelete(ctx context.Context, cmd *cli.Command) error {
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

	return client.Sessions.Delete(ctx, cmd.Value("session-id").(string), options...)
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
