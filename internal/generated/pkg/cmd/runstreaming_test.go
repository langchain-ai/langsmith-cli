// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestRunsStreamingQueryV2(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs:streaming", "query-v2",
			"--max-items", "10",
			"--ai-search", "The customer was charged twice",
			"--cursor", "eyJ2IjoxLCJhIjoicnVucy5xdWVyeSIsImsiOiJwYXNzIiwiYiI6InNkYiIsInQiOiJsdChjdXJzb3IsICcyMDI1LTEyLTEyIDE5OjAzOjI4LjQ4MTI1NTAxOWIxM2YyJykifQ",
			"--filter", `and(eq(run_type, "llm"), gt(latency, 5))`,
			"--has-error=false",
			"--id", "018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327",
			"--id", "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			"--include-subtree-usage=true",
			"--is-root=true",
			"--max-feedback-modified-at", "'2024-12-31T23:59:59Z'",
			"--max-start-time", "'2024-12-31T23:59:59Z'",
			"--min-feedback-modified-at", "'2024-01-01T00:00:00Z'",
			"--min-start-time", "'2024-01-01T00:00:00Z'",
			"--page-size", "100",
			"--project-id", "018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327",
			"--project-id", "0190a1b2-c3d4-7ef0-a5b6-6ea3a82e9328",
			"--reference-dataset-id", "018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327",
			"--reference-example", "b2c3d4e5-f6a7-4b5c-9d0e-1f2a3b4c5d6e",
			"--reference-example", "c3d4e5f6-a7b8-4c5d-0e1f-2a3b4c5d6e7f",
			"--run-type", "LLM",
			"--select", "ID",
			"--select", "NAME",
			"--select", "PROJECT_ID",
			"--select", "START_TIME",
			"--select", "RUN_TYPE",
			"--select", "STATUS",
			"--trace-filter", `eq(status, "success")`,
			"--trace-id", "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			"--tree-filter", `has(tags, "production")`,
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"ai_search: The customer was charged twice\n" +
			"cursor: >-\n" +
			"  eyJ2IjoxLCJhIjoicnVucy5xdWVyeSIsImsiOiJwYXNzIiwiYiI6InNkYiIsInQiOiJsdChjdXJzb3IsICcyMDI1LTEyLTEyIDE5OjAzOjI4LjQ4MTI1NTAxOWIxM2YyJykifQ\n" +
			"filter: and(eq(run_type, \"llm\"), gt(latency, 5))\n" +
			"has_error: false\n" +
			"ids:\n" +
			"  - 018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327\n" +
			"  - f47ac10b-58cc-4372-a567-0e02b2c3d479\n" +
			"include_subtree_usage: true\n" +
			"is_root: true\n" +
			"max_feedback_modified_at: '2024-12-31T23:59:59Z'\n" +
			"max_start_time: '2024-12-31T23:59:59Z'\n" +
			"min_feedback_modified_at: '2024-01-01T00:00:00Z'\n" +
			"min_start_time: '2024-01-01T00:00:00Z'\n" +
			"page_size: 100\n" +
			"project_ids:\n" +
			"  - 018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327\n" +
			"  - 0190a1b2-c3d4-7ef0-a5b6-6ea3a82e9328\n" +
			"reference_dataset_id: 018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327\n" +
			"reference_examples:\n" +
			"  - b2c3d4e5-f6a7-4b5c-9d0e-1f2a3b4c5d6e\n" +
			"  - c3d4e5f6-a7b8-4c5d-0e1f-2a3b4c5d6e7f\n" +
			"run_type: LLM\n" +
			"selects:\n" +
			"  - ID\n" +
			"  - NAME\n" +
			"  - PROJECT_ID\n" +
			"  - START_TIME\n" +
			"  - RUN_TYPE\n" +
			"  - STATUS\n" +
			"trace_filter: eq(status, \"success\")\n" +
			"trace_id: f47ac10b-58cc-4372-a567-0e02b2c3d479\n" +
			"tree_filter: has(tags, \"production\")\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs:streaming", "query-v2",
			"--max-items", "10",
		)
	})
}
