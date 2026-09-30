// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestEvaluatorsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"evaluators", "list",
			"--id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--evaluator-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--include-backfill-progress=true",
			"--name-contains", "name_contains",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--tag-value-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--type", "session",
		)
	})
}
