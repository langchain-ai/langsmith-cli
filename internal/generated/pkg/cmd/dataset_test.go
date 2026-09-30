// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestDatasetsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "create",
			"--name", "name",
			"--id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--created-at", "'2019-12-27T18:11:19.117Z'",
			"--data-type", "kv",
			"--description", "description",
			"--externally-managed=true",
			"--extra", "{foo: bar}",
			"--inputs-schema-definition", "{foo: bar}",
			"--outputs-schema-definition", "{foo: bar}",
			"--tag-value-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--transformation", "[{path: [string], transformation_type: convert_to_openai_message}]",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(datasetsCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "create",
			"--name", "name",
			"--id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--created-at", "'2019-12-27T18:11:19.117Z'",
			"--data-type", "kv",
			"--description", "description",
			"--externally-managed=true",
			"--extra", "{foo: bar}",
			"--inputs-schema-definition", "{foo: bar}",
			"--outputs-schema-definition", "{foo: bar}",
			"--tag-value-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--transformation.path", "[string]",
			"--transformation.transformation-type", "convert_to_openai_message",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"name: name\n" +
			"id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"created_at: '2019-12-27T18:11:19.117Z'\n" +
			"data_type: kv\n" +
			"description: description\n" +
			"externally_managed: true\n" +
			"extra:\n" +
			"  foo: bar\n" +
			"inputs_schema_definition:\n" +
			"  foo: bar\n" +
			"outputs_schema_definition:\n" +
			"  foo: bar\n" +
			"tag_value_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"transformations:\n" +
			"  - path:\n" +
			"      - string\n" +
			"    transformation_type: convert_to_openai_message\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "create",
		)
	})
}

func TestDatasetsRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "retrieve",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

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

func TestDatasetsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "list",
			"--max-items", "10",
			"--id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--datatype", "kv",
			"--exclude", "[example_count, example_count]",
			"--exclude-corrections-datasets=true",
			"--limit", "1",
			"--metadata", "metadata",
			"--name", "name",
			"--name-contains", "name_contains",
			"--offset", "0",
			"--sort-by", "name",
			"--sort-by-desc=true",
			"--tag-value-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
		)
	})
}

func TestDatasetsDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "delete",
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

func TestDatasetsRetrieveCsv(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "retrieve-csv",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--as-of", "'2019-12-27T18:11:19.117Z'",
		)
	})
}

func TestDatasetsRetrieveJSONL(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "retrieve-jsonl",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--as-of", "'2019-12-27T18:11:19.117Z'",
		)
	})
}

func TestDatasetsRetrieveOpenAI(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "retrieve-openai",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--as-of", "'2019-12-27T18:11:19.117Z'",
		)
	})
}

func TestDatasetsRetrieveOpenAIFt(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "retrieve-openai-ft",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--as-of", "'2019-12-27T18:11:19.117Z'",
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

func TestDatasetsUpload(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "upload",
			"--file", mocktest.TestFile(t, "Example data"),
			"--input-key", "string",
			"--data-type", "kv",
			"--description", "description",
			"--input-key-mappings", "input_key_mappings",
			"--inputs-schema-definition", "inputs_schema_definition",
			"--metadata-key-mappings", "metadata_key_mappings",
			"--metadata-key", "string",
			"--name", "name",
			"--output-key-mappings", "output_key_mappings",
			"--output-key", "string",
			"--outputs-schema-definition", "outputs_schema_definition",
			"--tag-value-ids", "tag_value_ids",
			"--transformations", "transformations",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		testFile := mocktest.TestFile(t, "Example data")
		// Test piping YAML data over stdin
		pipeDataStr := "" +
			"file: Example data\n" +
			"input_keys:\n" +
			"  - string\n" +
			"data_type: kv\n" +
			"description: description\n" +
			"input_key_mappings: input_key_mappings\n" +
			"inputs_schema_definition: inputs_schema_definition\n" +
			"metadata_key_mappings: metadata_key_mappings\n" +
			"metadata_keys:\n" +
			"  - string\n" +
			"name: name\n" +
			"output_key_mappings: output_key_mappings\n" +
			"output_keys:\n" +
			"  - string\n" +
			"outputs_schema_definition: outputs_schema_definition\n" +
			"tag_value_ids: tag_value_ids\n" +
			"transformations: transformations\n"
		pipeDataStr = strings.ReplaceAll(pipeDataStr, "Example data", testFile)
		pipeData := []byte(pipeDataStr)
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets", "upload",
		)
	})
}
