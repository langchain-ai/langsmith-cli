// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestDatasetsComparativeCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:comparative", "create",
			"--experiment-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--created-at", "'2019-12-27T18:11:19.117Z'",
			"--description", "description",
			"--extra", "{foo: bar}",
			"--modified-at", "'2019-12-27T18:11:19.117Z'",
			"--name", "name",
			"--reference-dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"experiment_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"created_at: '2019-12-27T18:11:19.117Z'\n" +
			"description: description\n" +
			"extra:\n" +
			"  foo: bar\n" +
			"modified_at: '2019-12-27T18:11:19.117Z'\n" +
			"name: name\n" +
			"reference_dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:comparative", "create",
		)
	})
}

func TestDatasetsComparativeDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:comparative", "delete",
			"--comparative-experiment-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
