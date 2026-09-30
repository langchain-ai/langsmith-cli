// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestFeedbackTokensCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback:tokens", "create",
			"--feedback-key", "feedback_key",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--expires-at", "'2019-12-27T18:11:19.117Z'",
			"--expires-in", "{days: 0, hours: 0, minutes: 0}",
			"--feedback-config", "{type: continuous, categories: [{value: 0, label: x}], max: 0, min: 0}",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(feedbackTokensCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback:tokens", "create",
			"--feedback-key", "feedback_key",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--expires-at", "'2019-12-27T18:11:19.117Z'",
			"--expires-in.days", "0",
			"--expires-in.hours", "0",
			"--expires-in.minutes", "0",
			"--feedback-config.type", "continuous",
			"--feedback-config.categories", "[{value: 0, label: x}]",
			"--feedback-config.max", "0",
			"--feedback-config.min", "0",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"feedback_key: feedback_key\n" +
			"run_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"expires_at: '2019-12-27T18:11:19.117Z'\n" +
			"expires_in:\n" +
			"  days: 0\n" +
			"  hours: 0\n" +
			"  minutes: 0\n" +
			"feedback_config:\n" +
			"  type: continuous\n" +
			"  categories:\n" +
			"    - value: 0\n" +
			"      label: x\n" +
			"  max: 0\n" +
			"  min: 0\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback:tokens", "create",
		)
	})
}

func TestFeedbackTokensRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback:tokens", "retrieve",
			"--token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--comment", "comment",
			"--correction", "correction",
			"--extend-trace-retention=true",
			"--score", "0",
			"--value", "0",
		)
	})
}

func TestFeedbackTokensUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback:tokens", "update",
			"--token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--comment", "comment",
			"--correction", "{foo: bar}",
			"--extend-trace-retention=true",
			"--metadata", "{foo: bar}",
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
			"extend_trace_retention: true\n" +
			"metadata:\n" +
			"  foo: bar\n" +
			"score: 0\n" +
			"value: 0\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback:tokens", "update",
			"--token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestFeedbackTokensList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"feedback:tokens", "list",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
