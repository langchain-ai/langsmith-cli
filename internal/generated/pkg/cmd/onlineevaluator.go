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
			Usage:    `Allowed values: "llm", "code", "decision_model".`,
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
			Name:       "code-evaluator.code-evaluator-input",
			Usage:      "CodeEvaluatorInput is which thread data the evaluator receives.\nNull or omitted for run evaluators.",
			InnerField: "code_evaluator_input",
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
		&requestflag.InnerFlag[[]string]{
			Name:       "code-evaluator.workspace-secrets-keys",
			Usage:      "WorkspaceSecretsKeys names the workspace secrets this evaluator may use.\nEach is sent as a header on requests to the hosts allowed for that\nsecret; evaluator code never sees the value. Requires\nadvanced_features_enabled, and each secret needs an allowed host with a\nheader.",
			InnerField: "workspace_secrets_keys",
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
			Name:       "code-evaluator.code-evaluator-input",
			Usage:      "CodeEvaluatorInput is config (not a snapshot rebuild). Null clears it.",
			InnerField: "code_evaluator_input",
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
		&requestflag.InnerFlag[[]string]{
			Name:       "code-evaluator.workspace-secrets-keys",
			Usage:      "WorkspaceSecretsKeys names the workspace secrets this evaluator may use.\nEach is sent as a header on requests to the hosts allowed for that\nsecret; evaluator code never sees the value. Requires\nadvanced_features_enabled, and each secret needs an allowed host with a\nheader. Omit to keep the current list; [] clears it. Changing it does\nnot rebuild the evaluator.",
			InnerField: "workspace_secrets_keys",
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
	Usage:   "Returns per-day LLM evaluator spend for the requested 7-day period, grouped by\nevaluator, resource, or run rule. Exactly one of group_by, evaluator_id,\nsession_id, or dataset_id is required. resource_id, type, feedback_key,\ntag_value_id, and agent_id may be supplied with group_by to narrow listing\naggregations.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "period-start",
			Usage:     "Start of the 7-day window (YYYY-MM-DD).",
			Required:  true,
			QueryPath: "period_start",
		},
		&requestflag.Flag[string]{
			Name:      "agent-id",
			Usage:     "Filter grouped results to spend from evaluator attachments on the agent's environments or tagged datasets. Only valid with group_by.",
			QueryPath: "agent_id",
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
			Usage:     "Filter grouped results by evaluator type: 'llm', 'code' or 'decision_model'. Only valid with group_by.",
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
