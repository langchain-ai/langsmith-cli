// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestFeedbackCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback", "create",
			"--key", "key",
			"--id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--agent-environment", "LOCAL",
			"--agent-id", "agent_id",
			"--comment", "comment",
			"--comparative-experiment-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--correction", "{foo: bar}",
			"--created-at", "'2019-12-27T18:11:19.117Z'",
			"--error=true",
			"--extend-trace-retention=true",
			"--extra", "{foo: bar}",
			"--feedback-config", "{type: continuous, categories: [{value: 0, label: x}], max: 0, min: 0}",
			"--feedback-group-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--feedback-source", "{metadata: {foo: bar}, type: app}",
			"--feedback-thread-id", "feedback_thread_id",
			"--modified-at", "'2019-12-27T18:11:19.117Z'",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--score", "0",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--trace-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--value", "0",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(feedbackCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback", "create",
			"--key", "key",
			"--id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--agent-environment", "LOCAL",
			"--agent-id", "agent_id",
			"--comment", "comment",
			"--comparative-experiment-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--correction", "{foo: bar}",
			"--created-at", "'2019-12-27T18:11:19.117Z'",
			"--error=true",
			"--extend-trace-retention=true",
			"--extra", "{foo: bar}",
			"--feedback-config.type", "continuous",
			"--feedback-config.categories", "[{value: 0, label: x}]",
			"--feedback-config.max", "0",
			"--feedback-config.min", "0",
			"--feedback-group-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--feedback-source", "{metadata: {foo: bar}, type: app}",
			"--feedback-thread-id", "feedback_thread_id",
			"--modified-at", "'2019-12-27T18:11:19.117Z'",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--score", "0",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--trace-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--value", "0",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"key: key\n" +
			"id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"agent_environment: LOCAL\n" +
			"agent_id: agent_id\n" +
			"comment: comment\n" +
			"comparative_experiment_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"correction:\n" +
			"  foo: bar\n" +
			"created_at: '2019-12-27T18:11:19.117Z'\n" +
			"error: true\n" +
			"extend_trace_retention: true\n" +
			"extra:\n" +
			"  foo: bar\n" +
			"feedback_config:\n" +
			"  type: continuous\n" +
			"  categories:\n" +
			"    - value: 0\n" +
			"      label: x\n" +
			"  max: 0\n" +
			"  min: 0\n" +
			"feedback_group_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"feedback_source:\n" +
			"  metadata:\n" +
			"    foo: bar\n" +
			"  type: app\n" +
			"feedback_thread_id: feedback_thread_id\n" +
			"modified_at: '2019-12-27T18:11:19.117Z'\n" +
			"run_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"score: 0\n" +
			"session_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"start_time: '2019-12-27T18:11:19.117Z'\n" +
			"trace_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"value: 0\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback", "create",
		)
	})
}

func TestFeedbackRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback", "retrieve",
			"--feedback-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--include-user-names=true",
		)
	})
}

func TestFeedbackUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback", "update",
			"--feedback-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--comment", "comment",
			"--correction", "{foo: bar}",
			"--feedback-config", "{type: continuous, categories: [{value: 0, label: x}], max: 0, min: 0}",
			"--score", "0",
			"--value", "0",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(feedbackUpdate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback", "update",
			"--feedback-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--comment", "comment",
			"--correction", "{foo: bar}",
			"--feedback-config.type", "continuous",
			"--feedback-config.categories", "[{value: 0, label: x}]",
			"--feedback-config.max", "0",
			"--feedback-config.min", "0",
			"--score", "0",
			"--value", "0",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"comment: comment\n" +
			"correction:\n" +
			"  foo: bar\n" +
			"feedback_config:\n" +
			"  type: continuous\n" +
			"  categories:\n" +
			"    - value: 0\n" +
			"      label: x\n" +
			"  max: 0\n" +
			"  min: 0\n" +
			"score: 0\n" +
			"value: 0\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback", "update",
			"--feedback-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestFeedbackList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback", "list",
			"--max-items", "10",
			"--comparative-experiment-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--feedback-thread-id", "feedback_thread_id",
			"--has-comment=true",
			"--has-score=true",
			"--include-user-names=true",
			"--key", "[string, string]",
			"--level", "run",
			"--limit", "1",
			"--max-created-at", "'2019-12-27T18:11:19.117Z'",
			"--min-created-at", "'2019-12-27T18:11:19.117Z'",
			"--offset", "0",
			"--run", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--session", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--source", "[api, model]",
			"--user", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
		)
	})
}

func TestFeedbackDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback", "delete",
			"--feedback-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
