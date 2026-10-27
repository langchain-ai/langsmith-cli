// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestTracesQuery(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"traces", "query",
			"--max-items", "10",
			"--ai-search", "A run that contains a billing dispute",
			"--cursor", "cursor",
			"--max-feedback-modified-at", "'2024-12-31T23:59:59Z'",
			"--max-start-time", "'2024-12-31T23:59:59Z'",
			"--min-feedback-modified-at", "'2024-01-01T00:00:00Z'",
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
			"ai_search: A run that contains a billing dispute\n" +
			"cursor: cursor\n" +
			"max_feedback_modified_at: '2024-12-31T23:59:59Z'\n" +
			"max_start_time: '2024-12-31T23:59:59Z'\n" +
			"min_feedback_modified_at: '2024-01-01T00:00:00Z'\n" +
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
