// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestDatasetsExperimentRunsQuery(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:experiment-runs", "query",
			"--max-items", "10",
			"--dataset-id", "dataset_id",
			"--comparative-experiment-id", "comparative_experiment_id",
			"--cursor", "cursor",
			"--example-id", "string",
			"--experiment-id", "string",
			"--filters", "{foo: [string]}",
			"--page-size", "0",
			"--select", "ID",
			"--sort", "{by: by, order: order}",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(datasetsExperimentRunsQuery)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:experiment-runs", "query",
			"--max-items", "10",
			"--dataset-id", "dataset_id",
			"--comparative-experiment-id", "comparative_experiment_id",
			"--cursor", "cursor",
			"--example-id", "string",
			"--experiment-id", "string",
			"--filters", "{foo: [string]}",
			"--page-size", "0",
			"--select", "ID",
			"--sort.by", "by",
			"--sort.order", "order",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"comparative_experiment_id: comparative_experiment_id\n" +
			"cursor: cursor\n" +
			"example_ids:\n" +
			"  - string\n" +
			"experiment_ids:\n" +
			"  - string\n" +
			"filters:\n" +
			"  foo:\n" +
			"    - string\n" +
			"page_size: 0\n" +
			"selects:\n" +
			"  - ID\n" +
			"sort:\n" +
			"  by: by\n" +
			"  order: order\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:experiment-runs", "query",
			"--max-items", "10",
			"--dataset-id", "dataset_id",
		)
	})
}
