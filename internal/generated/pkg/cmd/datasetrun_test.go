// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestDatasetsRunsQuery(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:runs", "query",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--format", "csv",
			"--comparative-experiment-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--example-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--filters", "{foo: [string]}",
			"--include-annotator-detail=true",
			"--limit", "1",
			"--offset", "0",
			"--preview=true",
			"--sort-params", "{sort_by: sort_by, sort_order: ASC}",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(datasetsRunsQuery)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:runs", "query",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--format", "csv",
			"--comparative-experiment-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--example-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--filters", "{foo: [string]}",
			"--include-annotator-detail=true",
			"--limit", "1",
			"--offset", "0",
			"--preview=true",
			"--sort-params.sort-by", "sort_by",
			"--sort-params.sort-order", "ASC",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"session_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"comparative_experiment_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"example_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"filters:\n" +
			"  foo:\n" +
			"    - string\n" +
			"include_annotator_detail: true\n" +
			"limit: 1\n" +
			"offset: 0\n" +
			"preview: true\n" +
			"sort_params:\n" +
			"  sort_by: sort_by\n" +
			"  sort_order: ASC\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:runs", "query",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--format", "csv",
		)
	})
}
