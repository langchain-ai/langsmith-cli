// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestRunsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "create",
			"--id", "id",
			"--agent-environment", "LOCAL",
			"--agent-id", "agent_id",
			"--dotted-order", "dotted_order",
			"--end-time", "end_time",
			"--error", "error",
			"--event", "{foo: bar}",
			"--extra", "{foo: bar}",
			"--input-attachments", "{foo: bar}",
			"--inputs", "{foo: bar}",
			"--name", "name",
			"--output-attachments", "{foo: bar}",
			"--outputs", "{foo: bar}",
			"--parent-run-id", "parent_run_id",
			"--reference-example-id", "reference_example_id",
			"--run-type", "tool",
			"--serialized", "{foo: bar}",
			"--session-id", "session_id",
			"--session-name", "session_name",
			"--start-time", "start_time",
			"--status", "status",
			"--tag", "string",
			"--trace-id", "trace_id",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"id: id\n" +
			"agent_environment: LOCAL\n" +
			"agent_id: agent_id\n" +
			"dotted_order: dotted_order\n" +
			"end_time: end_time\n" +
			"error: error\n" +
			"events:\n" +
			"  - foo: bar\n" +
			"extra:\n" +
			"  foo: bar\n" +
			"input_attachments:\n" +
			"  foo: bar\n" +
			"inputs:\n" +
			"  foo: bar\n" +
			"name: name\n" +
			"output_attachments:\n" +
			"  foo: bar\n" +
			"outputs:\n" +
			"  foo: bar\n" +
			"parent_run_id: parent_run_id\n" +
			"reference_example_id: reference_example_id\n" +
			"run_type: tool\n" +
			"serialized:\n" +
			"  foo: bar\n" +
			"session_id: session_id\n" +
			"session_name: session_name\n" +
			"start_time: start_time\n" +
			"status: status\n" +
			"tags:\n" +
			"  - string\n" +
			"trace_id: trace_id\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "create",
		)
	})
}

func TestRunsUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "update",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--id", "id",
			"--agent-environment", "LOCAL",
			"--agent-id", "agent_id",
			"--dotted-order", "dotted_order",
			"--end-time", "end_time",
			"--error", "error",
			"--event", "{foo: bar}",
			"--extra", "{foo: bar}",
			"--input-attachments", "{foo: bar}",
			"--inputs", "{foo: bar}",
			"--name", "name",
			"--output-attachments", "{foo: bar}",
			"--outputs", "{foo: bar}",
			"--parent-run-id", "parent_run_id",
			"--reference-example-id", "reference_example_id",
			"--run-type", "tool",
			"--serialized", "{foo: bar}",
			"--session-id", "session_id",
			"--session-name", "session_name",
			"--start-time", "start_time",
			"--status", "status",
			"--tag", "string",
			"--trace-id", "trace_id",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"id: id\n" +
			"agent_environment: LOCAL\n" +
			"agent_id: agent_id\n" +
			"dotted_order: dotted_order\n" +
			"end_time: end_time\n" +
			"error: error\n" +
			"events:\n" +
			"  - foo: bar\n" +
			"extra:\n" +
			"  foo: bar\n" +
			"input_attachments:\n" +
			"  foo: bar\n" +
			"inputs:\n" +
			"  foo: bar\n" +
			"name: name\n" +
			"output_attachments:\n" +
			"  foo: bar\n" +
			"outputs:\n" +
			"  foo: bar\n" +
			"parent_run_id: parent_run_id\n" +
			"reference_example_id: reference_example_id\n" +
			"run_type: tool\n" +
			"serialized:\n" +
			"  foo: bar\n" +
			"session_id: session_id\n" +
			"session_name: session_name\n" +
			"start_time: start_time\n" +
			"status: status\n" +
			"tags:\n" +
			"  - string\n" +
			"trace_id: trace_id\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "update",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestRunsGetURL(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "get-url",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--project-id", "project_id",
			"--trace-id", "trace_id",
			"--start-time", "start_time",
		)
	})
}

func TestRunsIngestBatch(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "ingest-batch",
			"--patch", "{id: id, agent_environment: LOCAL, agent_id: agent_id, dotted_order: dotted_order, end_time: end_time, error: error, events: [{foo: bar}], extra: {foo: bar}, input_attachments: {foo: bar}, inputs: {foo: bar}, name: name, output_attachments: {foo: bar}, outputs: {foo: bar}, parent_run_id: parent_run_id, reference_example_id: reference_example_id, run_type: tool, serialized: {foo: bar}, session_id: session_id, session_name: session_name, start_time: start_time, status: status, tags: [string], trace_id: trace_id}",
			"--post", "{id: id, agent_environment: LOCAL, agent_id: agent_id, dotted_order: dotted_order, end_time: end_time, error: error, events: [{foo: bar}], extra: {foo: bar}, input_attachments: {foo: bar}, inputs: {foo: bar}, name: name, output_attachments: {foo: bar}, outputs: {foo: bar}, parent_run_id: parent_run_id, reference_example_id: reference_example_id, run_type: tool, serialized: {foo: bar}, session_id: session_id, session_name: session_name, start_time: start_time, status: status, tags: [string], trace_id: trace_id}",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(runsIngestBatch)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "ingest-batch",
			"--patch.id", "id",
			"--patch.agent-environment", "LOCAL",
			"--patch.agent-id", "agent_id",
			"--patch.dotted-order", "dotted_order",
			"--patch.end-time", "end_time",
			"--patch.error", "error",
			"--patch.events", "[{foo: bar}]",
			"--patch.extra", "{foo: bar}",
			"--patch.input-attachments", "{foo: bar}",
			"--patch.inputs", "{foo: bar}",
			"--patch.name", "name",
			"--patch.output-attachments", "{foo: bar}",
			"--patch.outputs", "{foo: bar}",
			"--patch.parent-run-id", "parent_run_id",
			"--patch.reference-example-id", "reference_example_id",
			"--patch.run-type", "tool",
			"--patch.serialized", "{foo: bar}",
			"--patch.session-id", "session_id",
			"--patch.session-name", "session_name",
			"--patch.start-time", "start_time",
			"--patch.status", "status",
			"--patch.tags", "[string]",
			"--patch.trace-id", "trace_id",
			"--post.id", "id",
			"--post.agent-environment", "LOCAL",
			"--post.agent-id", "agent_id",
			"--post.dotted-order", "dotted_order",
			"--post.end-time", "end_time",
			"--post.error", "error",
			"--post.events", "[{foo: bar}]",
			"--post.extra", "{foo: bar}",
			"--post.input-attachments", "{foo: bar}",
			"--post.inputs", "{foo: bar}",
			"--post.name", "name",
			"--post.output-attachments", "{foo: bar}",
			"--post.outputs", "{foo: bar}",
			"--post.parent-run-id", "parent_run_id",
			"--post.reference-example-id", "reference_example_id",
			"--post.run-type", "tool",
			"--post.serialized", "{foo: bar}",
			"--post.session-id", "session_id",
			"--post.session-name", "session_name",
			"--post.start-time", "start_time",
			"--post.status", "status",
			"--post.tags", "[string]",
			"--post.trace-id", "trace_id",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"patch:\n" +
			"  - id: id\n" +
			"    agent_environment: LOCAL\n" +
			"    agent_id: agent_id\n" +
			"    dotted_order: dotted_order\n" +
			"    end_time: end_time\n" +
			"    error: error\n" +
			"    events:\n" +
			"      - foo: bar\n" +
			"    extra:\n" +
			"      foo: bar\n" +
			"    input_attachments:\n" +
			"      foo: bar\n" +
			"    inputs:\n" +
			"      foo: bar\n" +
			"    name: name\n" +
			"    output_attachments:\n" +
			"      foo: bar\n" +
			"    outputs:\n" +
			"      foo: bar\n" +
			"    parent_run_id: parent_run_id\n" +
			"    reference_example_id: reference_example_id\n" +
			"    run_type: tool\n" +
			"    serialized:\n" +
			"      foo: bar\n" +
			"    session_id: session_id\n" +
			"    session_name: session_name\n" +
			"    start_time: start_time\n" +
			"    status: status\n" +
			"    tags:\n" +
			"      - string\n" +
			"    trace_id: trace_id\n" +
			"post:\n" +
			"  - id: id\n" +
			"    agent_environment: LOCAL\n" +
			"    agent_id: agent_id\n" +
			"    dotted_order: dotted_order\n" +
			"    end_time: end_time\n" +
			"    error: error\n" +
			"    events:\n" +
			"      - foo: bar\n" +
			"    extra:\n" +
			"      foo: bar\n" +
			"    input_attachments:\n" +
			"      foo: bar\n" +
			"    inputs:\n" +
			"      foo: bar\n" +
			"    name: name\n" +
			"    output_attachments:\n" +
			"      foo: bar\n" +
			"    outputs:\n" +
			"      foo: bar\n" +
			"    parent_run_id: parent_run_id\n" +
			"    reference_example_id: reference_example_id\n" +
			"    run_type: tool\n" +
			"    serialized:\n" +
			"      foo: bar\n" +
			"    session_id: session_id\n" +
			"    session_name: session_name\n" +
			"    start_time: start_time\n" +
			"    status: status\n" +
			"    tags:\n" +
			"      - string\n" +
			"    trace_id: trace_id\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "ingest-batch",
		)
	})
}

func TestRunsQueryV1(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "query-v1",
			"--max-items", "10",
			"--id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--cursor", "cursor",
			"--data-source-type", "current",
			"--end-time", "'2019-12-27T18:11:19.117Z'",
			"--error=true",
			"--execution-order", "1",
			"--filter", "filter",
			"--is-root=true",
			"--limit", "1",
			"--order", "asc",
			"--parent-run", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--query", "query",
			"--reference-example", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--run-type", "tool",
			"--search-filter", "search_filter",
			"--select", "id",
			"--session", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--skip-pagination=true",
			"--skip-prev-cursor=true",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--trace", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--trace-filter", "trace_filter",
			"--tree-filter", "tree_filter",
			"--use-experimental-search=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"id:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"cursor: cursor\n" +
			"data_source_type: current\n" +
			"end_time: '2019-12-27T18:11:19.117Z'\n" +
			"error: true\n" +
			"execution_order: 1\n" +
			"filter: filter\n" +
			"is_root: true\n" +
			"limit: 1\n" +
			"order: asc\n" +
			"parent_run: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"query: query\n" +
			"reference_example:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"run_type: tool\n" +
			"search_filter: search_filter\n" +
			"select:\n" +
			"  - id\n" +
			"session:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"skip_pagination: true\n" +
			"skip_prev_cursor: true\n" +
			"start_time: '2019-12-27T18:11:19.117Z'\n" +
			"trace: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"trace_filter: trace_filter\n" +
			"tree_filter: tree_filter\n" +
			"use_experimental_search: true\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "query-v1",
			"--max-items", "10",
		)
	})
}

func TestRunsQueryV2(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "query-v2",
			"--max-items", "10",
			"--cursor", "eyJ2IjoxLCJhIjoicnVucy5xdWVyeSIsImsiOiJwYXNzIiwiYiI6InNkYiIsInQiOiJsdChjdXJzb3IsICcyMDI1LTEyLTEyIDE5OjAzOjI4LjQ4MTI1NTAxOWIxM2YyJykifQ",
			"--filter", `and(eq(run_type, "llm"), gt(latency, 5))`,
			"--has-error=false",
			"--id", "018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327",
			"--id", "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			"--is-root=true",
			"--max-start-time", "'2024-12-31T23:59:59Z'",
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
			"--accept", "Accept",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"cursor: >-\n" +
			"  eyJ2IjoxLCJhIjoicnVucy5xdWVyeSIsImsiOiJwYXNzIiwiYiI6InNkYiIsInQiOiJsdChjdXJzb3IsICcyMDI1LTEyLTEyIDE5OjAzOjI4LjQ4MTI1NTAxOWIxM2YyJykifQ\n" +
			"filter: and(eq(run_type, \"llm\"), gt(latency, 5))\n" +
			"has_error: false\n" +
			"ids:\n" +
			"  - 018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327\n" +
			"  - f47ac10b-58cc-4372-a567-0e02b2c3d479\n" +
			"is_root: true\n" +
			"max_start_time: '2024-12-31T23:59:59Z'\n" +
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
			"runs", "query-v2",
			"--max-items", "10",
			"--accept", "Accept",
		)
	})
}

func TestRunsRetrieveV1(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "retrieve-v1",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--exclude-s3-stored-attributes=true",
			"--exclude-serialized=true",
			"--include-messages=true",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
		)
	})
}

func TestRunsRetrieveV2(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "retrieve-v2",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--project-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--select", "ID",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--accept", "Accept",
		)
	})
}

func TestRunsStats(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "stats",
			"--session", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--data-source-type", "current",
			"--end-time", "'2019-12-27T18:11:19.117Z'",
			"--error=true",
			"--execution-order", "1",
			"--filter", "filter",
			"--group-by", "{attribute: name, max_groups: 0, path: path}",
			"--group", "[string]",
			"--include-details=true",
			"--is-root=true",
			"--parent-run", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--query", "query",
			"--reference-dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--reference-example", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--run-type", "tool",
			"--search-filter", "search_filter",
			"--select", "[run_count]",
			"--skip-pagination=true",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--trace", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--trace-filter", "trace_filter",
			"--tree-filter", "tree_filter",
			"--use-experimental-search=true",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(runsStats)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "stats",
			"--session", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--data-source-type", "current",
			"--end-time", "'2019-12-27T18:11:19.117Z'",
			"--error=true",
			"--execution-order", "1",
			"--filter", "filter",
			"--group-by.attribute", "name",
			"--group-by.max-groups", "0",
			"--group-by.path", "path",
			"--group", "[string]",
			"--include-details=true",
			"--is-root=true",
			"--parent-run", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--query", "query",
			"--reference-dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--reference-example", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--run-type", "tool",
			"--search-filter", "search_filter",
			"--select", "[run_count]",
			"--skip-pagination=true",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--trace", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--trace-filter", "trace_filter",
			"--tree-filter", "tree_filter",
			"--use-experimental-search=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"session:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"id:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"data_source_type: current\n" +
			"end_time: '2019-12-27T18:11:19.117Z'\n" +
			"error: true\n" +
			"execution_order: 1\n" +
			"filter: filter\n" +
			"group_by:\n" +
			"  attribute: name\n" +
			"  max_groups: 0\n" +
			"  path: path\n" +
			"groups:\n" +
			"  - string\n" +
			"include_details: true\n" +
			"is_root: true\n" +
			"parent_run: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"query: query\n" +
			"reference_dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"reference_example:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"run_type: tool\n" +
			"search_filter: search_filter\n" +
			"select:\n" +
			"  - run_count\n" +
			"skip_pagination: true\n" +
			"start_time: '2019-12-27T18:11:19.117Z'\n" +
			"trace: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"trace_filter: trace_filter\n" +
			"tree_filter: tree_filter\n" +
			"use_experimental_search: true\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "stats",
		)
	})
}

func TestRunsUpdate2(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "update-2",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--id", "id",
			"--agent-environment", "LOCAL",
			"--agent-id", "agent_id",
			"--dotted-order", "dotted_order",
			"--end-time", "end_time",
			"--error", "error",
			"--event", "{foo: bar}",
			"--extra", "{foo: bar}",
			"--input-attachments", "{foo: bar}",
			"--inputs", "{foo: bar}",
			"--name", "name",
			"--output-attachments", "{foo: bar}",
			"--outputs", "{foo: bar}",
			"--parent-run-id", "parent_run_id",
			"--reference-example-id", "reference_example_id",
			"--run-type", "tool",
			"--serialized", "{foo: bar}",
			"--session-id", "session_id",
			"--session-name", "session_name",
			"--start-time", "start_time",
			"--status", "status",
			"--tag", "string",
			"--trace-id", "trace_id",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"id: id\n" +
			"agent_environment: LOCAL\n" +
			"agent_id: agent_id\n" +
			"dotted_order: dotted_order\n" +
			"end_time: end_time\n" +
			"error: error\n" +
			"events:\n" +
			"  - foo: bar\n" +
			"extra:\n" +
			"  foo: bar\n" +
			"input_attachments:\n" +
			"  foo: bar\n" +
			"inputs:\n" +
			"  foo: bar\n" +
			"name: name\n" +
			"output_attachments:\n" +
			"  foo: bar\n" +
			"outputs:\n" +
			"  foo: bar\n" +
			"parent_run_id: parent_run_id\n" +
			"reference_example_id: reference_example_id\n" +
			"run_type: tool\n" +
			"serialized:\n" +
			"  foo: bar\n" +
			"session_id: session_id\n" +
			"session_name: session_name\n" +
			"start_time: start_time\n" +
			"status: status\n" +
			"tags:\n" +
			"  - string\n" +
			"trace_id: trace_id\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs", "update-2",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
