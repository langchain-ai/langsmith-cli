// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestTracesListRuns(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"traces", "list-runs",
			"--trace-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--project-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--filter", "filter",
			"--max-start-time", "'2019-12-27T18:11:19.117Z'",
			"--min-start-time", "'2019-12-27T18:11:19.117Z'",
			"--select", "ID",
			"--accept", "Accept",
		)
	})
}

func TestTracesQuery(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"traces", "query",
			"--max-items", "10",
			"--cursor", "cursor",
			"--max-start-time", "'2024-12-31T23:59:59Z'",
			"--min-start-time", "'2024-01-01T00:00:00Z'",
			"--page-size", "20",
			"--project-id", "018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327",
			"--select", "ID",
			"--select", "NAME",
			"--select", "START_TIME",
			"--select", "STATUS",
			"--select", "TOTAL_TOKENS",
			"--select", "TOTAL_COST",
			"--select", "FIRST_TOKEN_TIME",
			"--trace-filter", `eq(status, "error")`,
			"--trace-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--tree-filter", `has(tags, "production")`,
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"cursor: cursor\n" +
			"max_start_time: '2024-12-31T23:59:59Z'\n" +
			"min_start_time: '2024-01-01T00:00:00Z'\n" +
			"page_size: 20\n" +
			"project_id: 018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327\n" +
			"selects:\n" +
			"  - ID\n" +
			"  - NAME\n" +
			"  - START_TIME\n" +
			"  - STATUS\n" +
			"  - TOTAL_TOKENS\n" +
			"  - TOTAL_COST\n" +
			"  - FIRST_TOKEN_TIME\n" +
			"trace_filter: eq(status, \"error\")\n" +
			"trace_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"tree_filter: has(tags, \"production\")\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"traces", "query",
			"--max-items", "10",
		)
	})
}
