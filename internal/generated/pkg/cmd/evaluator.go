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

var evaluatorsCreateDecisionModel = cli.Command{
	Name:    "create-decision-model",
	Usage:   "Create a decision-model evaluator (Jev, SemIf or OpenAI Decisions) from its\nprompt and questions, and attach it to a project or dataset. Each question's key\nbecomes a feedback key.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "display-name",
			Usage:    "The evaluator's name.",
			Required: true,
			BodyPath: "display_name",
		},
		&requestflag.Flag[[]any]{
			Name:     "prompt",
			Usage:    "The prompt the model reads, as (role, template) pairs.",
			Required: true,
			BodyPath: "prompt",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "questions",
			Usage:    "The questions, keyed by feedback key, in the order to ask them.",
			Required: true,
			BodyPath: "questions",
		},
		&requestflag.Flag[*string]{
			Name:     "model-base-url",
			Usage:    "Call this endpoint directly instead of through the LangSmith gateway, with the workspace's TYPESAFE_API_KEY or OPENAI_API_KEY secret. Required for typesafe_compatible.",
			BodyPath: "base_url",
		},
		&requestflag.Flag[*string]{
			Name:     "dataset-id",
			Usage:    "The dataset whose experiments to evaluate.",
			BodyPath: "dataset_id",
		},
		&requestflag.Flag[*string]{
			Name:     "filter",
			Usage:    "Only evaluate runs that match this filter.",
			BodyPath: "filter",
		},
		&requestflag.Flag[*int64]{
			Name:     "max-retries",
			Usage:    "How many times to retry a failed model call. openai_decisions only.",
			BodyPath: "max_retries",
		},
		&requestflag.Flag[*string]{
			Name:     "model",
			Usage:    "Defaults to jev-latest for typesafe and gpt-6-luna for openai_decisions. Use semif-qwen3.5-4b for SemIf.",
			BodyPath: "model",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "model-headers",
			Usage:    "Headers sent with each model call. They are saved in the evaluator's prompt as given, so do not put API keys here.",
			BodyPath: "model_headers",
		},
		&requestflag.Flag[*string]{
			Name:     "playground-settings-id",
			Usage:    "A saved model configuration to start from: its model, endpoint, key and settings apply unless set here.",
			BodyPath: "playground_settings_id",
		},
		&requestflag.Flag[string]{
			Name:     "provider",
			Usage:    "typesafe for Jev and SemIf, typesafe_compatible for a TypeSafe-compatible endpoint, or openai_decisions.",
			Default:  "typesafe",
			BodyPath: "provider",
		},
		&requestflag.Flag[float64]{
			Name:     "sampling-rate",
			Usage:    "The fraction of runs to evaluate.",
			Default:  1,
			BodyPath: "sampling_rate",
		},
		&requestflag.Flag[*string]{
			Name:     "session-id",
			Usage:    "The tracing project to evaluate.",
			BodyPath: "session_id",
		},
		&requestflag.Flag[string]{
			Name:     "template-format",
			Usage:    `Allowed values: "mustache", "f-string".`,
			Default:  "mustache",
			BodyPath: "template_format",
		},
		&requestflag.Flag[*int64]{
			Name:     "timeout-seconds",
			Usage:    "Seconds to wait for each model call. openai_decisions only.",
			BodyPath: "timeout_seconds",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "variable-mapping",
			Usage:    "Maps the prompt's variables to run fields.",
			BodyPath: "variable_mapping",
		},
	},
	Action:          handleEvaluatorsCreateDecisionModel,
	HideHelpCommand: true,
}

func handleEvaluatorsCreateDecisionModel(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.EvaluatorNewDecisionModelParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Evaluators.NewDecisionModel(ctx, params, options...)
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
		Title:          "evaluators create-decision-model",
		Transform:      transform,
	})
}
