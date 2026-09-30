// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestAnnotationQueuesRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "retrieve",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestAnnotationQueuesUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "update",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--default-dataset", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--description", "description",
			"--enable-reservations=true",
			"--metadata", "{foo: bar}",
			"--name", "name",
			"--num-reviewers-per-item", "0",
			"--reservation-minutes", "0",
			"--reviewer-access-mode", "any",
			"--rubric-instructions", "rubric_instructions",
			"--rubric-item", "[{feedback_key: feedback_key, description: description, is_assertion: true, is_required: true, regex_validator: string, score_descriptions: {foo: string}, value_descriptions: {foo: string}}]",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(annotationQueuesUpdate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "update",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--default-dataset", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--description", "description",
			"--enable-reservations=true",
			"--metadata", "{foo: bar}",
			"--name", "name",
			"--num-reviewers-per-item", "0",
			"--reservation-minutes", "0",
			"--reviewer-access-mode", "any",
			"--rubric-instructions", "rubric_instructions",
			"--rubric-item.feedback-key", "feedback_key",
			"--rubric-item.description", "description",
			"--rubric-item.is-assertion=true",
			"--rubric-item.is-required=true",
			"--rubric-item.regex-validator", "string",
			"--rubric-item.score-descriptions", "{foo: string}",
			"--rubric-item.value-descriptions", "{foo: string}",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"default_dataset: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"description: description\n" +
			"enable_reservations: true\n" +
			"metadata:\n" +
			"  foo: bar\n" +
			"name: name\n" +
			"num_reviewers_per_item: 0\n" +
			"reservation_minutes: 0\n" +
			"reviewer_access_mode: any\n" +
			"rubric_instructions: rubric_instructions\n" +
			"rubric_items:\n" +
			"  - feedback_key: feedback_key\n" +
			"    description: description\n" +
			"    is_assertion: true\n" +
			"    is_required: true\n" +
			"    regex_validator: string\n" +
			"    score_descriptions:\n" +
			"      foo: string\n" +
			"    value_descriptions:\n" +
			"      foo: string\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "update",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestAnnotationQueuesDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "delete",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestAnnotationQueuesAnnotationQueues(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "annotation-queues",
			"--name", "name",
			"--id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--created-at", "'2019-12-27T18:11:19.117Z'",
			"--default-dataset", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--description", "description",
			"--enable-reservations=true",
			"--metadata", "{foo: bar}",
			"--num-reviewers-per-item", "0",
			"--reservation-minutes", "0",
			"--reviewer-access-mode", "reviewer_access_mode",
			"--rubric-instructions", "rubric_instructions",
			"--rubric-item", "[{feedback_key: feedback_key, description: description, is_assertion: true, is_required: true, regex_validator: string, score_descriptions: {foo: string}, value_descriptions: {foo: string}}]",
			"--session-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--updated-at", "'2019-12-27T18:11:19.117Z'",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(annotationQueuesAnnotationQueues)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "annotation-queues",
			"--name", "name",
			"--id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--created-at", "'2019-12-27T18:11:19.117Z'",
			"--default-dataset", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--description", "description",
			"--enable-reservations=true",
			"--metadata", "{foo: bar}",
			"--num-reviewers-per-item", "0",
			"--reservation-minutes", "0",
			"--reviewer-access-mode", "reviewer_access_mode",
			"--rubric-instructions", "rubric_instructions",
			"--rubric-item.feedback-key", "feedback_key",
			"--rubric-item.description", "description",
			"--rubric-item.is-assertion=true",
			"--rubric-item.is-required=true",
			"--rubric-item.regex-validator", "string",
			"--rubric-item.score-descriptions", "{foo: string}",
			"--rubric-item.value-descriptions", "{foo: string}",
			"--session-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--updated-at", "'2019-12-27T18:11:19.117Z'",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"name: name\n" +
			"id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"created_at: '2019-12-27T18:11:19.117Z'\n" +
			"default_dataset: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"description: description\n" +
			"enable_reservations: true\n" +
			"metadata:\n" +
			"  foo: bar\n" +
			"num_reviewers_per_item: 0\n" +
			"reservation_minutes: 0\n" +
			"reviewer_access_mode: reviewer_access_mode\n" +
			"rubric_instructions: rubric_instructions\n" +
			"rubric_items:\n" +
			"  - feedback_key: feedback_key\n" +
			"    description: description\n" +
			"    is_assertion: true\n" +
			"    is_required: true\n" +
			"    regex_validator: string\n" +
			"    score_descriptions:\n" +
			"      foo: string\n" +
			"    value_descriptions:\n" +
			"      foo: string\n" +
			"session_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"updated_at: '2019-12-27T18:11:19.117Z'\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "annotation-queues",
		)
	})
}

func TestAnnotationQueuesCreateRunStatus(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "create-run-status",
			"--annotation-queue-run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--override-added-at", "'2019-12-27T18:11:19.117Z'",
			"--status", "status",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"override_added_at: '2019-12-27T18:11:19.117Z'\n" +
			"status: status\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "create-run-status",
			"--annotation-queue-run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestAnnotationQueuesExport(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "export",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--end-time", "'2019-12-27T18:11:19.117Z'",
			"--include-annotator-detail=true",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"end_time: '2019-12-27T18:11:19.117Z'\n" +
			"include_annotator_detail: true\n" +
			"start_time: '2019-12-27T18:11:19.117Z'\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "export",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestAnnotationQueuesPopulate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "populate",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--extend-trace-retention=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"queue_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"session_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"extend_trace_retention: true\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "populate",
		)
	})
}

func TestAnnotationQueuesRetrieveAnnotationQueues(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "retrieve-annotation-queues",
			"--max-items", "10",
			"--assigned-to-me=true",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--limit", "1",
			"--name", "name",
			"--name-contains", "name_contains",
			"--offset", "0",
			"--queue-type", "single",
			"--sort-by", "sort_by",
			"--sort-by-desc=true",
			"--tag-value-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
		)
	})
}

func TestAnnotationQueuesRetrieveQueues(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "retrieve-queues",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestAnnotationQueuesRetrieveRun(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "retrieve-run",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--index", "0",
			"--include-extra=true",
		)
	})
}

func TestAnnotationQueuesRetrieveSize(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "retrieve-size",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--status", "needs_my_review",
		)
	})
}

func TestAnnotationQueuesRetrieveTotalArchived(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "retrieve-total-archived",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--end-time", "'2019-12-27T18:11:19.117Z'",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
		)
	})
}

func TestAnnotationQueuesRetrieveTotalSize(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"annotation-queues", "retrieve-total-size",
			"--queue-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
