// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestAnnotationQueuesRunsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "create",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--extend-trace-retention=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("- 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "create",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--extend-trace-retention=true",
		)
	})
}

func TestAnnotationQueuesRunsUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "update",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--queue-run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--added-at", "'2019-12-27T18:11:19.117Z'",
			"--last-reviewed-time", "'2019-12-27T18:11:19.117Z'",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"added_at: '2019-12-27T18:11:19.117Z'\n" +
			"last_reviewed_time: '2019-12-27T18:11:19.117Z'\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "update",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--queue-run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestAnnotationQueuesRunsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "list",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--archived=true",
			"--include-stats=true",
			"--limit", "1",
			"--offset", "0",
			"--status", "needs_my_review",
		)
	})
}

func TestAnnotationQueuesRunsCreateByKey(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "create-by-key",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body", "{run_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, session_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, start_time: '2019-12-27T18:11:19.117Z', source_proposed_example_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e}",
			"--extend-trace-retention=true",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(annotationQueuesRunsCreateByKey)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "create-by-key",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body.run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body.session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--body.start-time", "2019-12-27T18:11:19.117Z",
			"--body.source-proposed-example-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--extend-trace-retention=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"- run_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  session_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  start_time: '2019-12-27T18:11:19.117Z'\n" +
			"  source_proposed_example_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "create-by-key",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--extend-trace-retention=true",
		)
	})
}

func TestAnnotationQueuesRunsDeleteAll(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "delete-all",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--delete-all=true",
			"--exclude-run-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--run-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"delete_all: true\n" +
			"exclude_run_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"run_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "delete-all",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestAnnotationQueuesRunsDeleteQueue(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:runs", "delete-queue",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--queue-run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
