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

var onlineEvaluatorsCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "Create a new LLM or code evaluator for the current workspace.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[map[string]any]{
			Name:     "code-evaluator",
			BodyPath: "code_evaluator",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "llm-evaluator",
			BodyPath: "llm_evaluator",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[string]{
			Name:     "type",
			Usage:    `Allowed values: "llm", "code".`,
			BodyPath: "type",
		},
	},
	Action:          handleOnlineEvaluatorsCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"code-evaluator": {
		&requestflag.InnerFlag[bool]{
			Name:       "code-evaluator.advanced-features-enabled",
			InnerField: "advanced_features_enabled",
		},
		&requestflag.InnerFlag[string]{
			Name:       "code-evaluator.code",
			InnerField: "code",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "code-evaluator.dependencies",
			InnerField: "dependencies",
		},
		&requestflag.InnerFlag[string]{
			Name:       "code-evaluator.language",
			Usage:      `Default: "python"`,
			InnerField: "language",
		},
		&requestflag.InnerFlag[string]{
			Name:       "code-evaluator.managed-code-evaluator-key",
			Usage:      `Allowed values: "voice_metrics".`,
			InnerField: "managed_code_evaluator_key",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "code-evaluator.managed-code-evaluator-settings",
			InnerField: "managed_code_evaluator_settings",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "code-evaluator.require-attachments",
			Usage:      "RequireAttachments opts the evaluator into selecting/presigning run\nattachments (s3_urls) at evaluation time. Default false.",
			InnerField: "require_attachments",
		},
	},
	"llm-evaluator": {
		&requestflag.InnerFlag[string]{
			Name:       "llm-evaluator.commit-hash-or-tag",
			InnerField: "commit_hash_or_tag",
		},
		&requestflag.InnerFlag[string]{
			Name:       "llm-evaluator.playground-settings-id",
			Usage:      "Model Configuration ID",
			InnerField: "playground_settings_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "llm-evaluator.prompt-repo-handle",
			InnerField: "prompt_repo_handle",
		},
		&requestflag.InnerFlag[any]{
			Name:       "llm-evaluator.variable-mapping",
			InnerField: "variable_mapping",
		},
	},
})

var onlineEvaluatorsRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Retrieve a single evaluator by its ID.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "evaluator-id",
			Required:  true,
			PathParam: "evaluator_id",
		},
	},
	Action:          handleOnlineEvaluatorsRetrieve,
	HideHelpCommand: true,
}

var onlineEvaluatorsUpdate = requestflag.WithInnerFlags(cli.Command{
	Name:    "update",
	Usage:   "Update an existing evaluator's name, LLM configuration, or code configuration.\nReturns 409 when a code evaluator build is ENQUEUED or BUILDING.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "evaluator-id",
			Required:  true,
			PathParam: "evaluator_id",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "code-evaluator",
			BodyPath: "code_evaluator",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "llm-evaluator",
			BodyPath: "llm_evaluator",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			BodyPath: "name",
		},
	},
	Action:          handleOnlineEvaluatorsUpdate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"code-evaluator": {
		&requestflag.InnerFlag[bool]{
			Name:       "code-evaluator.advanced-features-enabled",
			InnerField: "advanced_features_enabled",
		},
		&requestflag.InnerFlag[string]{
			Name:       "code-evaluator.code",
			InnerField: "code",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "code-evaluator.dependencies",
			InnerField: "dependencies",
		},
		&requestflag.InnerFlag[string]{
			Name:       "code-evaluator.language",
			InnerField: "language",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "code-evaluator.managed-code-evaluator-settings",
			InnerField: "managed_code_evaluator_settings",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "code-evaluator.require-attachments",
			Usage:      "RequireAttachments is fetch-time config: updating it does not rebuild\nthe sandbox snapshot.",
			InnerField: "require_attachments",
		},
	},
	"llm-evaluator": {
		&requestflag.InnerFlag[string]{
			Name:       "llm-evaluator.commit-hash-or-tag",
			InnerField: "commit_hash_or_tag",
		},
		&requestflag.InnerFlag[int64]{
			Name:       "llm-evaluator.num-few-shot-examples",
			InnerField: "num_few_shot_examples",
		},
		&requestflag.InnerFlag[string]{
			Name:       "llm-evaluator.playground-settings-id",
			Usage:      "Model Configuration ID",
			InnerField: "playground_settings_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "llm-evaluator.prompt-repo-handle",
			InnerField: "prompt_repo_handle",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "llm-evaluator.use-corrections-dataset",
			InnerField: "use_corrections_dataset",
		},
		&requestflag.InnerFlag[any]{
			Name:       "llm-evaluator.variable-mapping",
			InnerField: "variable_mapping",
		},
	},
})

var onlineEvaluatorsList = cli.Command{
	Name:    "list",
	Usage:   "List evaluators for the current workspace, with optional filtering by type,\nname, tag, feedback key, or resource ID.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "agent-id",
			Usage:     "Filter to evaluators attached to the agent's environments or tagged datasets",
			QueryPath: "agent_id",
		},
		&requestflag.Flag[string]{
			Name:      "feedback-key",
			Usage:     "Filter by feedback key",
			QueryPath: "feedback_key",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Usage:     "Maximum number of results (1-100)",
			Default:   100,
			QueryPath: "limit",
		},
		&requestflag.Flag[string]{
			Name:      "name-contains",
			Usage:     "Filter by name substring (also searches creator names)",
			QueryPath: "name_contains",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Usage:     "Offset for pagination",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[[]string]{
			Name:      "resource-id",
			Usage:     "Filter by resource IDs",
			QueryPath: "resource_id",
		},
		&requestflag.Flag[string]{
			Name:      "sort-by",
			Usage:     "Field to sort by",
			QueryPath: "sort_by",
		},
		&requestflag.Flag[bool]{
			Name:      "sort-by-desc",
			Usage:     "Sort in descending order",
			QueryPath: "sort_by_desc",
		},
		&requestflag.Flag[[]string]{
			Name:      "tag-value-id",
			Usage:     "Filter by tag value IDs",
			QueryPath: "tag_value_id",
		},
		&requestflag.Flag[string]{
			Name:      "type",
			Usage:     "Filter by evaluator type",
			QueryPath: "type",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleOnlineEvaluatorsList,
	HideHelpCommand: true,
}

var onlineEvaluatorsDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete an evaluator. Returns 409 when a code evaluator build is ENQUEUED or\nBUILDING, or when run rules still reference the evaluator and delete_run_rules\nis false. When delete_run_rules is true, all run rules referencing this\nevaluator are deleted first (same tenant) if the build is not in flight.\nAssociated llm_evaluators and code_evaluators rows are removed by foreign-key\ncascade when the evaluator row is deleted.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "evaluator-id",
			Required:  true,
			PathParam: "evaluator_id",
		},
		&requestflag.Flag[bool]{
			Name:      "delete-run-rules",
			Usage:     "When true, delete all run rules for this evaluator before deleting the evaluator",
			QueryPath: "delete_run_rules",
		},
	},
	Action:          handleOnlineEvaluatorsDelete,
	HideHelpCommand: true,
}

var onlineEvaluatorsBulkDelete = cli.Command{
	Name:    "bulk-delete",
	Usage:   "Delete multiple evaluators by their IDs. Returns per-item success/failure.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[[]string]{
			Name:      "evaluator-id",
			Usage:     "Evaluator IDs to delete",
			Required:  true,
			QueryPath: "evaluator_ids",
		},
		&requestflag.Flag[bool]{
			Name:      "delete-run-rules",
			Usage:     "When true, delete all run rules for this evaluator before deleting the evaluator",
			QueryPath: "delete_run_rules",
		},
	},
	Action:          handleOnlineEvaluatorsBulkDelete,
	HideHelpCommand: true,
}

var onlineEvaluatorsSpend = cli.Command{
	Name:    "spend",
	Usage:   "Returns per-day LLM evaluator spend for the requested 7-day period, grouped by\nevaluator, resource, or run rule. Exactly one of group_by, evaluator_id,\nsession_id, or dataset_id is required. resource_id, type, feedback_key, and\ntag_value_id may be supplied with group_by to narrow listing aggregations.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "period-start",
			Usage:     "Start of the 7-day window (YYYY-MM-DD).",
			Required:  true,
			QueryPath: "period_start",
		},
		&requestflag.Flag[string]{
			Name:      "dataset-id",
			Usage:     "Filter to a specific dataset (UUID). Mutually exclusive with group_by.",
			QueryPath: "dataset_id",
		},
		&requestflag.Flag[string]{
			Name:      "evaluator-id",
			Usage:     "Filter to a specific evaluator (UUID). Mutually exclusive with group_by.",
			QueryPath: "evaluator_id",
		},
		&requestflag.Flag[string]{
			Name:      "feedback-key",
			Usage:     "Filter grouped results by evaluator feedback key. Only valid with group_by.",
			QueryPath: "feedback_key",
		},
		&requestflag.Flag[string]{
			Name:      "group-by",
			Usage:     "Aggregation mode: 'evaluator', 'resource', or 'run_rule'. Mutually exclusive with entity filters.",
			QueryPath: "group_by",
		},
		&requestflag.Flag[[]string]{
			Name:      "resource-id",
			Usage:     "Filter grouped results to evaluators attached to all supplied project or dataset IDs. Only valid with group_by.",
			QueryPath: "resource_id",
		},
		&requestflag.Flag[string]{
			Name:      "session-id",
			Usage:     "Filter to a specific project (UUID). Mutually exclusive with group_by.",
			QueryPath: "session_id",
		},
		&requestflag.Flag[[]string]{
			Name:      "tag-value-id",
			Usage:     "Filter grouped results to evaluators, projects, or datasets tagged with all supplied tag value IDs. Only valid with group_by.",
			QueryPath: "tag_value_id",
		},
		&requestflag.Flag[string]{
			Name:      "type",
			Usage:     "Filter grouped results by evaluator type: 'llm' or 'code'. Only valid with group_by.",
			QueryPath: "type",
		},
	},
	Action:          handleOnlineEvaluatorsSpend,
	HideHelpCommand: true,
}

func handleOnlineEvaluatorsCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.OnlineEvaluatorNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.OnlineEvaluators.New(ctx, params, options...)
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
		Title:          "online-evaluators create",
		Transform:      transform,
	})
}

func handleOnlineEvaluatorsRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("evaluator-id") && len(unusedArgs) > 0 {
		cmd.Set("evaluator-id", unusedArgs[0])
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
	_, err = client.OnlineEvaluators.Get(ctx, cmd.Value("evaluator-id").(string), options...)
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
		Title:          "online-evaluators retrieve",
		Transform:      transform,
	})
}

func handleOnlineEvaluatorsUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("evaluator-id") && len(unusedArgs) > 0 {
		cmd.Set("evaluator-id", unusedArgs[0])
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

	params := langsmith.OnlineEvaluatorUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.OnlineEvaluators.Update(
		ctx,
		cmd.Value("evaluator-id").(string),
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
		Title:          "online-evaluators update",
		Transform:      transform,
	})
}

func handleOnlineEvaluatorsList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.OnlineEvaluatorListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.OnlineEvaluators.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "online-evaluators list",
			Transform:      transform,
		})
	} else {
		iter := client.OnlineEvaluators.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "online-evaluators list",
			Transform:      transform,
		})
	}
}

func handleOnlineEvaluatorsDelete(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("evaluator-id") && len(unusedArgs) > 0 {
		cmd.Set("evaluator-id", unusedArgs[0])
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

	params := langsmith.OnlineEvaluatorDeleteParams{}

	return client.OnlineEvaluators.Delete(
		ctx,
		cmd.Value("evaluator-id").(string),
		params,
		options...,
	)
}

func handleOnlineEvaluatorsBulkDelete(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.OnlineEvaluatorBulkDeleteParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.OnlineEvaluators.BulkDelete(ctx, params, options...)
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
		Title:          "online-evaluators bulk-delete",
		Transform:      transform,
	})
}

func handleOnlineEvaluatorsSpend(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.OnlineEvaluatorSpendParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.OnlineEvaluators.Spend(ctx, params, options...)
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
		Title:          "online-evaluators spend",
		Transform:      transform,
	})
}
