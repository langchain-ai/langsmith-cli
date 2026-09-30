// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestThreadsAggregateStats(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"threads", "aggregate-stats",
			"--project-id", "0190a1b2-c3d4-7ef0-a5b6-6ea3a82e9328",
			"--select", "THREAD_COUNT",
			"--select", "TRACE_COUNT",
			"--select", "TOTAL_TOKENS",
			"--select", "TOTAL_COST",
			"--filter", `eq(status, "error")`,
			"--max-start-time", "'2019-12-27T18:11:19.117Z'",
			"--min-start-time", "'2019-12-27T18:11:19.117Z'",
			"--thread-filter", "gte(turn_count, 3)",
			"--trace-filter", `eq(status, "error")`,
			"--tree-filter", `has(tags, "production")`,
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"project_id: 0190a1b2-c3d4-7ef0-a5b6-6ea3a82e9328\n" +
			"select:\n" +
			"  - THREAD_COUNT\n" +
			"  - TRACE_COUNT\n" +
			"  - TOTAL_TOKENS\n" +
			"  - TOTAL_COST\n" +
			"filter: eq(status, \"error\")\n" +
			"max_start_time: '2019-12-27T18:11:19.117Z'\n" +
			"min_start_time: '2019-12-27T18:11:19.117Z'\n" +
			"thread_filter: gte(turn_count, 3)\n" +
			"trace_filter: eq(status, \"error\")\n" +
			"tree_filter: has(tags, \"production\")\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"threads", "aggregate-stats",
		)
	})
}

func TestThreadsListTraces(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"threads", "list-traces",
			"--max-items", "10",
			"--thread-id", "thread_id",
			"--project-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--cursor", "cursor",
			"--filter", "filter",
			"--page-size", "1",
			"--select", "THREAD_ID",
			"--trace-filter", "trace_filter",
			"--tree-filter", "tree_filter",
		)
	})
}

func TestThreadsQuery(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"threads", "query",
			"--max-items", "10",
			"--cursor", "cursor",
			"--filter", "filter",
			"--max-start-time", "'2019-12-27T18:11:19.117Z'",
			"--min-start-time", "'2019-12-27T18:11:19.117Z'",
			"--page-size", "20",
			"--project-id", "0190a1b2-c3d4-7ef0-a5b6-6ea3a82e9328",
			"--thread-filter", "gte(turn_count, 3)",
			"--trace-filter", `eq(status, "error")`,
			"--tree-filter", `has(tags, "production")`,
			"--accept", "Accept",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
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
			"threads", "query",
			"--max-items", "10",
			"--accept", "Accept",
		)
	})
}

func TestThreadsStats(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"threads", "stats",
			"--thread-id", "thread_id",
			"--select", "TURNS",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--filter", "filter",
		)
	})
}
