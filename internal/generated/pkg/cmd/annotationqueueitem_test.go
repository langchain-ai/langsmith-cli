// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestAnnotationQueuesItemsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "create",
			"--queue-id", "queue_id",
			"--extend-trace-retention=true",
			"--item", "{item_type: RUN, project_id: project_id, run_id: run_id, session_id: session_id, source_proposed_example_id: source_proposed_example_id, start_time: '2019-12-27T18:11:19.117Z', thread_id: thread_id}",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(annotationQueuesItemsCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "create",
			"--queue-id", "queue_id",
			"--extend-trace-retention=true",
			"--item.item-type", "RUN",
			"--item.project-id", "project_id",
			"--item.run-id", "run_id",
			"--item.session-id", "session_id",
			"--item.source-proposed-example-id", "source_proposed_example_id",
			"--item.start-time", "2019-12-27T18:11:19.117Z",
			"--item.thread-id", "thread_id",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"items:\n" +
			"  - item_type: RUN\n" +
			"    project_id: project_id\n" +
			"    run_id: run_id\n" +
			"    session_id: session_id\n" +
			"    source_proposed_example_id: source_proposed_example_id\n" +
			"    start_time: '2019-12-27T18:11:19.117Z'\n" +
			"    thread_id: thread_id\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "create",
			"--queue-id", "queue_id",
			"--extend-trace-retention=true",
		)
	})
}

func TestAnnotationQueuesItemsUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "update",
			"--queue-id", "queue_id",
			"--item-id", "item_id",
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
			"annotation-queues:items", "update",
			"--queue-id", "queue_id",
			"--item-id", "item_id",
		)
	})
}

func TestAnnotationQueuesItemsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "list",
			"--max-items", "10",
			"--queue-id", "queue_id",
			"--status", "needs_my_review",
			"--cursor", "cursor",
			"--direction", "forward",
			"--item-type", "RUN",
			"--max-start-time", "'2019-12-27T18:11:19.117Z'",
			"--min-start-time", "'2019-12-27T18:11:19.117Z'",
			"--page-size", "0",
		)
	})
}

func TestAnnotationQueuesItemsCreateStatus(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "create-status",
			"--queue-item-id", "queue_item_id",
			"--override-added-at", "override_added_at",
			"--status", "viewed",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"override_added_at: override_added_at\n" +
			"status: viewed\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "create-status",
			"--queue-item-id", "queue_item_id",
		)
	})
}

func TestAnnotationQueuesItemsDeleteAll(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "delete-all",
			"--queue-id", "queue_id",
			"--item-id", "string",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"item_ids:\n" +
			"  - string\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "delete-all",
			"--queue-id", "queue_id",
		)
	})
}

func TestAnnotationQueuesItemsRetrieveCount(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "retrieve-count",
			"--queue-id", "queue_id",
			"--status", "status",
			"--end-time", "end_time",
			"--max-start-time", "'2019-12-27T18:11:19.117Z'",
			"--min-start-time", "'2019-12-27T18:11:19.117Z'",
			"--start-time", "start_time",
		)
	})
}

func TestAnnotationQueuesItemsRetrievePlacement(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues:items", "retrieve-placement",
			"--queue-id", "queue_id",
			"--item-id", "item_id",
		)
	})
}
