// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestDatasetsUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "update",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--baseline-experiment-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--description", "string",
			"--inputs-schema-definition", "{foo: bar}",
			"--metadata", "{foo: bar}",
			"--name", "string",
			"--outputs-schema-definition", "{foo: bar}",
			"--patch-examples", "{foo: {attachments_operations: {rename: {foo: string}, retain: [string]}, dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, inputs: {foo: bar}, metadata: {foo: bar}, outputs: {foo: bar}, overwrite: true, split: [string]}}",
			"--transformations", "[{path: [string], transformation_type: convert_to_openai_message}]",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"baseline_experiment_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"description: string\n" +
			"inputs_schema_definition:\n" +
			"  foo: bar\n" +
			"metadata:\n" +
			"  foo: bar\n" +
			"name: string\n" +
			"outputs_schema_definition:\n" +
			"  foo: bar\n" +
			"patch_examples:\n" +
			"  foo:\n" +
			"    attachments_operations:\n" +
			"      rename:\n" +
			"        foo: string\n" +
			"      retain:\n" +
			"        - string\n" +
			"    dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"    inputs:\n" +
			"      foo: bar\n" +
			"    metadata:\n" +
			"      foo: bar\n" +
			"    outputs:\n" +
			"      foo: bar\n" +
			"    overwrite: true\n" +
			"    split:\n" +
			"      - string\n" +
			"transformations:\n" +
			"  - path:\n" +
			"      - string\n" +
			"    transformation_type: convert_to_openai_message\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "update",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestDatasetsClone(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "clone",
			"--source-dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--target-dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--as-of", "'2019-12-27T18:11:19.117Z'",
			"--example", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--split", "string",
			"--tag-value-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"source_dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"target_dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"as_of: '2019-12-27T18:11:19.117Z'\n" +
			"examples:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"split: string\n" +
			"tag_value_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "clone",
		)
	})
}

func TestDatasetsRetrieveVersion(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "retrieve-version",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--as-of", "'2019-12-27T18:11:19.117Z'",
			"--tag", "tag",
		)
	})
}

func TestDatasetsUpdateTags(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "update-tags",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--as-of", "'2019-12-27T18:11:19.117Z'",
			"--tag", "tag",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"as_of: '2019-12-27T18:11:19.117Z'\n" +
			"tag: tag\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "update-tags",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
