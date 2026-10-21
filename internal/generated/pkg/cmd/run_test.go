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
			"--address", "lrn:agents/support-agent/environments/production",
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
			"address: lrn:agents/support-agent/environments/production\n" +
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
			"--patch", "{id: id, address: lrn:agents/support-agent/environments/production, dotted_order: dotted_order, end_time: end_time, error: error, events: [{foo: bar}], extra: {foo: bar}, input_attachments: {foo: bar}, inputs: {foo: bar}, name: name, output_attachments: {foo: bar}, outputs: {foo: bar}, parent_run_id: parent_run_id, reference_example_id: reference_example_id, run_type: tool, serialized: {foo: bar}, session_id: session_id, session_name: session_name, start_time: start_time, status: status, tags: [string], trace_id: trace_id}",
			"--post", "{id: id, address: lrn:agents/support-agent/environments/production, dotted_order: dotted_order, end_time: end_time, error: error, events: [{foo: bar}], extra: {foo: bar}, input_attachments: {foo: bar}, inputs: {foo: bar}, name: name, output_attachments: {foo: bar}, outputs: {foo: bar}, parent_run_id: parent_run_id, reference_example_id: reference_example_id, run_type: tool, serialized: {foo: bar}, session_id: session_id, session_name: session_name, start_time: start_time, status: status, tags: [string], trace_id: trace_id}",
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
			"--patch.address", "lrn:agents/support-agent/environments/production",
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
			"--post.address", "lrn:agents/support-agent/environments/production",
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
			"    address: lrn:agents/support-agent/environments/production\n" +
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
			"    address: lrn:agents/support-agent/environments/production\n" +
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
