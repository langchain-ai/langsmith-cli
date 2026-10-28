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
			"--rubric-item", "[{feedback_key: feedback_key, description: description, feedback_config: {id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, feedback_config: {type: continuous, categories: [{value: 0, label: x}], max: 0, min: 0}, feedback_key: feedback_key, modified_at: '2019-12-27T18:11:19.117Z', tenant_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, is_deleted: true, is_enforced: true, is_first: true, is_lower_score_better: true}, feedback_config_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, is_assertion: true, is_required: true, regex_validator: string, required_if: {join: all, rules: [{key: key, op: is, values: [0]}]}, score_descriptions: {foo: string}, value_descriptions: {foo: string}, visible_if: {join: all, rules: [{key: key, op: is, values: [0]}]}}]",
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
			"--rubric-item.feedback-config", "{id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, feedback_config: {type: continuous, categories: [{value: 0, label: x}], max: 0, min: 0}, feedback_key: feedback_key, modified_at: '2019-12-27T18:11:19.117Z', tenant_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, is_deleted: true, is_enforced: true, is_first: true, is_lower_score_better: true}",
			"--rubric-item.feedback-config-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--rubric-item.is-assertion=true",
			"--rubric-item.is-required=true",
			"--rubric-item.regex-validator", "string",
			"--rubric-item.required-if", "{join: all, rules: [{key: key, op: is, values: [0]}]}",
			"--rubric-item.score-descriptions", "{foo: string}",
			"--rubric-item.value-descriptions", "{foo: string}",
			"--rubric-item.visible-if", "{join: all, rules: [{key: key, op: is, values: [0]}]}",
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
			"    feedback_config:\n" +
			"      id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"      feedback_config:\n" +
			"        type: continuous\n" +
			"        categories:\n" +
			"          - value: 0\n" +
			"            label: x\n" +
			"        max: 0\n" +
			"        min: 0\n" +
			"      feedback_key: feedback_key\n" +
			"      modified_at: '2019-12-27T18:11:19.117Z'\n" +
			"      tenant_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"      is_deleted: true\n" +
			"      is_enforced: true\n" +
			"      is_first: true\n" +
			"      is_lower_score_better: true\n" +
			"    feedback_config_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"    is_assertion: true\n" +
			"    is_required: true\n" +
			"    regex_validator: string\n" +
			"    required_if:\n" +
			"      join: all\n" +
			"      rules:\n" +
			"        - key: key\n" +
			"          op: is\n" +
			"          values:\n" +
			"            - 0\n" +
			"    score_descriptions:\n" +
			"      foo: string\n" +
			"    value_descriptions:\n" +
			"      foo: string\n" +
			"    visible_if:\n" +
			"      join: all\n" +
			"      rules:\n" +
			"        - key: key\n" +
			"          op: is\n" +
			"          values:\n" +
			"            - 0\n")
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
