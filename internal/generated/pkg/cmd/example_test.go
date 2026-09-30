// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestExamplesCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "create",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--created-at", "created_at",
			"--inputs", "{foo: bar}",
			"--metadata", "{foo: bar}",
			"--outputs", "{foo: bar}",
			"--source-run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--source-run-start-time", "'2019-12-27T18:11:19.117Z'",
			"--source-session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--source-trace-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--split", "[string]",
			"--use-legacy-message-format=true",
			"--use-source-run-attachment", "string",
			"--use-source-run-io=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"created_at: created_at\n" +
			"inputs:\n" +
			"  foo: bar\n" +
			"metadata:\n" +
			"  foo: bar\n" +
			"outputs:\n" +
			"  foo: bar\n" +
			"source_run_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"source_run_start_time: '2019-12-27T18:11:19.117Z'\n" +
			"source_session_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"source_trace_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"split:\n" +
			"  - string\n" +
			"use_legacy_message_format: true\n" +
			"use_source_run_attachments:\n" +
			"  - string\n" +
			"use_source_run_io: true\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "create",
		)
	})
}

func TestExamplesRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "retrieve",
			"--example-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--as-of", "'2019-12-27T18:11:19.117Z'",
			"--dataset", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestExamplesUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "update",
			"--example-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--attachments-operations", "{rename: {foo: string}, retain: [string]}",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--inputs", "{foo: bar}",
			"--metadata", "{foo: bar}",
			"--outputs", "{foo: bar}",
			"--overwrite=true",
			"--split", "[string]",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(examplesUpdate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "update",
			"--example-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--attachments-operations.rename", "{foo: string}",
			"--attachments-operations.retain", "[string]",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--inputs", "{foo: bar}",
			"--metadata", "{foo: bar}",
			"--outputs", "{foo: bar}",
			"--overwrite=true",
			"--split", "[string]",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"attachments_operations:\n" +
			"  rename:\n" +
			"    foo: string\n" +
			"  retain:\n" +
			"    - string\n" +
			"dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"inputs:\n" +
			"  foo: bar\n" +
			"metadata:\n" +
			"  foo: bar\n" +
			"outputs:\n" +
			"  foo: bar\n" +
			"overwrite: true\n" +
			"split:\n" +
			"  - string\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "update",
			"--example-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestExamplesList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "list",
			"--max-items", "10",
			"--id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--as-of", "'2019-12-27T18:11:19.117Z'",
			"--dataset", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--descending=true",
			"--filter", "filter",
			"--full-text-contain", "[string, string]",
			"--limit", "1",
			"--metadata", "metadata",
			"--offset", "0",
			"--order", "recent",
			"--random-seed", "0",
			"--select", "id",
			"--split", "[string, string]",
		)
	})
}

func TestExamplesDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "delete",
			"--example-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestExamplesDeleteAll(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "delete-all",
			"--example-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestExamplesRetrieveCount(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "retrieve-count",
			"--id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--as-of", "'2019-12-27T18:11:19.117Z'",
			"--dataset", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--filter", "filter",
			"--full-text-contain", "[string, string]",
			"--metadata", "metadata",
			"--split", "[string, string]",
		)
	})
}

func TestExamplesUploadFromCsv(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "upload-from-csv",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--file", mocktest.TestFile(t, "Example data"),
			"--input-key", "string",
			"--metadata-key", "string",
			"--output-key", "string",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		testFile := mocktest.TestFile(t, "Example data")
		// Test piping YAML data over stdin
		pipeDataStr := "" +
			"file: Example data\n" +
			"input_keys:\n" +
			"  - string\n" +
			"metadata_keys:\n" +
			"  - string\n" +
			"output_keys:\n" +
			"  - string\n"
		pipeDataStr = strings.ReplaceAll(pipeDataStr, "Example data", testFile)
		pipeData := []byte(pipeDataStr)
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples", "upload-from-csv",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
