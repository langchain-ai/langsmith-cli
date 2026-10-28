// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestThreadsStreamingQuery(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"threads:streaming", "query",
			"--max-items", "10",
			"--ai-search", "A conversation about a refund request",
			"--cursor", "cursor",
			"--filter", "filter",
			"--max-start-time", "'2019-12-27T18:11:19.117Z'",
			"--min-start-time", "'2019-12-27T18:11:19.117Z'",
			"--page-size", "20",
			"--project-id", "0190a1b2-c3d4-7ef0-a5b6-6ea3a82e9328",
			"--thread-filter", "gte(turn_count, 3)",
			"--trace-filter", `eq(status, "error")`,
			"--tree-filter", `has(tags, "production")`,
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"ai_search: A conversation about a refund request\n" +
			"cursor: cursor\n" +
			"filter: filter\n" +
			"max_start_time: '2019-12-27T18:11:19.117Z'\n" +
			"min_start_time: '2019-12-27T18:11:19.117Z'\n" +
			"page_size: 20\n" +
			"project_id: 0190a1b2-c3d4-7ef0-a5b6-6ea3a82e9328\n" +
			"thread_filter: gte(turn_count, 3)\n" +
			"trace_filter: eq(status, \"error\")\n" +
			"tree_filter: has(tags, \"production\")\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"threads:streaming", "query",
			"--max-items", "10",
		)
	})
}
