// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

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
