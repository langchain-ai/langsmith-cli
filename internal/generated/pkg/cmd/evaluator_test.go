// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestEvaluatorsCreateDecisionModel(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"evaluators", "create-decision-model",
			"--display-name", "display_name",
			"--prompt", "[{}, {}]",
			"--questions", "{foo: {instructions: instructions, type: noul, levels: [string], 'no': 'no', options: [string], 'yes': 'yes'}}",
			"--model-base-url", "base_url",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--filter", "filter",
			"--max-retries", "0",
			"--model", "model",
			"--model-headers", "{foo: string}",
			"--playground-settings-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--provider", "typesafe",
			"--sampling-rate", "0",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--template-format", "mustache",
			"--timeout-seconds", "0",
			"--variable-mapping", "{foo: string}",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"display_name: display_name\n" +
			"prompt:\n" +
			"  - - {}\n" +
			"    - {}\n" +
			"questions:\n" +
			"  foo:\n" +
			"    instructions: instructions\n" +
			"    type: noul\n" +
			"    levels:\n" +
			"      - string\n" +
			"    'no': 'no'\n" +
			"    options:\n" +
			"      - string\n" +
			"    'yes': 'yes'\n" +
			"base_url: base_url\n" +
			"dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"filter: filter\n" +
			"max_retries: 0\n" +
			"model: model\n" +
			"model_headers:\n" +
			"  foo: string\n" +
			"playground_settings_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"provider: typesafe\n" +
			"sampling_rate: 0\n" +
			"session_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"template_format: mustache\n" +
			"timeout_seconds: 0\n" +
			"variable_mapping:\n" +
			"  foo: string\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"evaluators", "create-decision-model",
		)
	})
}
