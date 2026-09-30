// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestExamplesBulkCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples:bulk", "create",
			"--body", "{dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, created_at: created_at, inputs: {foo: bar}, metadata: {foo: bar}, outputs: {foo: bar}, source_run_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, source_run_start_time: '2019-12-27T18:11:19.117Z', source_session_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, source_trace_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, split: [string], use_legacy_message_format: true, use_source_run_attachments: [string], use_source_run_io: true}",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(examplesBulkCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples:bulk", "create",
			"--body.dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body.id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body.created-at", "created_at",
			"--body.inputs", "{foo: bar}",
			"--body.metadata", "{foo: bar}",
			"--body.outputs", "{foo: bar}",
			"--body.source-run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body.source-run-start-time", "2019-12-27T18:11:19.117Z",
			"--body.source-session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body.source-trace-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body.split", "[string]",
			"--body.use-legacy-message-format=true",
			"--body.use-source-run-attachments", "[string]",
			"--body.use-source-run-io=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"- dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  created_at: created_at\n" +
			"  inputs:\n" +
			"    foo: bar\n" +
			"  metadata:\n" +
			"    foo: bar\n" +
			"  outputs:\n" +
			"    foo: bar\n" +
			"  source_run_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  source_run_start_time: '2019-12-27T18:11:19.117Z'\n" +
			"  source_session_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  source_trace_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  split:\n" +
			"    - string\n" +
			"  use_legacy_message_format: true\n" +
			"  use_source_run_attachments:\n" +
			"    - string\n" +
			"  use_source_run_io: true\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples:bulk", "create",
		)
	})
}

func TestExamplesBulkPatchAll(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples:bulk", "patch-all",
			"--body", "{id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, attachments_operations: {rename: {foo: string}, retain: [string]}, dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, inputs: {foo: bar}, metadata: {foo: bar}, outputs: {foo: bar}, overwrite: true, split: [string]}",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(examplesBulkPatchAll)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples:bulk", "patch-all",
			"--body.id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body.attachments-operations", "{rename: {foo: string}, retain: [string]}",
			"--body.dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body.inputs", "{foo: bar}",
			"--body.metadata", "{foo: bar}",
			"--body.outputs", "{foo: bar}",
			"--body.overwrite=true",
			"--body.split", "[string]",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"- id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  attachments_operations:\n" +
			"    rename:\n" +
			"      foo: string\n" +
			"    retain:\n" +
			"      - string\n" +
			"  dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  inputs:\n" +
			"    foo: bar\n" +
			"  metadata:\n" +
			"    foo: bar\n" +
			"  outputs:\n" +
			"    foo: bar\n" +
			"  overwrite: true\n" +
			"  split:\n" +
			"    - string\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"examples:bulk", "patch-all",
		)
	})
}
