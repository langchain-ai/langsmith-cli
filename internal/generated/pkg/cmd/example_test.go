// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

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
