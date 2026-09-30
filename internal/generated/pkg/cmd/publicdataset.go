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

var publicDatasetsList = cli.Command{
	Name:    "list",
	Usage:   "Get dataset by ids or the shared dataset if not specifed.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "share-token",
			Required:  true,
			PathParam: "share_token",
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
		&requestflag.Flag[string]{
			Name:      "sort-by",
			Usage:     "Enum for available dataset columns to sort by.",
			QueryPath: "sort_by",
		},
		&requestflag.Flag[bool]{
			Name:      "sort-by-desc",
			Default:   true,
			QueryPath: "sort_by_desc",
		},
	},
	Action:          handlePublicDatasetsList,
	HideHelpCommand: true,
}

var publicDatasetsListComparative = cli.Command{
	Name:    "list-comparative",
	Usage:   "Get all comparative experiments for a given dataset.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "share-token",
			Required:  true,
			PathParam: "share_token",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Default:   100,
			QueryPath: "limit",
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
		&requestflag.Flag[string]{
			Name:      "sort-by",
			Usage:     "Enum for available comparative experiment columns to sort by.",
			QueryPath: "sort_by",
		},
		&requestflag.Flag[bool]{
			Name:      "sort-by-desc",
			Default:   true,
			QueryPath: "sort_by_desc",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handlePublicDatasetsListComparative,
	HideHelpCommand: true,
}

var publicDatasetsListFeedback = cli.Command{
	Name:    "list-feedback",
	Usage:   "Get feedback for runs in projects run over a dataset that has been shared.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "share-token",
			Required:  true,
			PathParam: "share_token",
		},
		&requestflag.Flag[*bool]{
			Name:      "has-comment",
			QueryPath: "has_comment",
		},
		&requestflag.Flag[*bool]{
			Name:      "has-score",
			QueryPath: "has_score",
		},
		&requestflag.Flag[any]{
			Name:      "key",
			QueryPath: "key",
		},
		&requestflag.Flag[*string]{
			Name:      "level",
			Usage:     "Enum for feedback levels.",
			QueryPath: "level",
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
		&requestflag.Flag[any]{
			Name:      "run",
			QueryPath: "run",
		},
		&requestflag.Flag[any]{
			Name:      "session",
			QueryPath: "session",
		},
		&requestflag.Flag[any]{
			Name:      "source",
			QueryPath: "source",
		},
		&requestflag.Flag[any]{
			Name:      "user",
			QueryPath: "user",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handlePublicDatasetsListFeedback,
	HideHelpCommand: true,
}

var publicDatasetsListSessions = cli.Command{
	Name:    "list-sessions",
	Usage:   "Get projects run on a dataset that has been shared.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "share-token",
			Required:  true,
			PathParam: "share_token",
		},
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
		&requestflag.Flag[int64]{
			Name:      "limit",
			Default:   100,
			QueryPath: "limit",
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
	Action:          handlePublicDatasetsListSessions,
	HideHelpCommand: true,
}

var publicDatasetsRetrieveSessionsBulk = cli.Command{
	Name:    "retrieve-sessions-bulk",
	Usage:   "Get sessions from multiple datasets using share tokens.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[[]string]{
			Name:      "share-token",
			Required:  true,
			QueryPath: "share_tokens",
		},
	},
	Action:          handlePublicDatasetsRetrieveSessionsBulk,
	HideHelpCommand: true,
}

func handlePublicDatasetsList(ctx context.Context, cmd *cli.Command) error {
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
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.PublicDatasetListParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Public.Datasets.List(
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
		Title:          "public:datasets list",
		Transform:      transform,
	})
}

func handlePublicDatasetsListComparative(ctx context.Context, cmd *cli.Command) error {
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
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.PublicDatasetListComparativeParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Public.Datasets.ListComparative(
			ctx,
			cmd.Value("share-token").(string),
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
			Title:          "public:datasets list-comparative",
			Transform:      transform,
		})
	} else {
		iter := client.Public.Datasets.ListComparativeAutoPaging(
			ctx,
			cmd.Value("share-token").(string),
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
			Title:          "public:datasets list-comparative",
			Transform:      transform,
		})
	}
}

func handlePublicDatasetsListFeedback(ctx context.Context, cmd *cli.Command) error {
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
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.PublicDatasetListFeedbackParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Public.Datasets.ListFeedback(
			ctx,
			cmd.Value("share-token").(string),
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
			Title:          "public:datasets list-feedback",
			Transform:      transform,
		})
	} else {
		iter := client.Public.Datasets.ListFeedbackAutoPaging(
			ctx,
			cmd.Value("share-token").(string),
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
			Title:          "public:datasets list-feedback",
			Transform:      transform,
		})
	}
}

func handlePublicDatasetsListSessions(ctx context.Context, cmd *cli.Command) error {
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
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.PublicDatasetListSessionsParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Public.Datasets.ListSessions(
			ctx,
			cmd.Value("share-token").(string),
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
			Title:          "public:datasets list-sessions",
			Transform:      transform,
		})
	} else {
		iter := client.Public.Datasets.ListSessionsAutoPaging(
			ctx,
			cmd.Value("share-token").(string),
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
			Title:          "public:datasets list-sessions",
			Transform:      transform,
		})
	}
}

func handlePublicDatasetsRetrieveSessionsBulk(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.PublicDatasetGetSessionsBulkParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Public.Datasets.GetSessionsBulk(ctx, params, options...)
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
		Title:          "public:datasets retrieve-sessions-bulk",
		Transform:      transform,
	})
}
