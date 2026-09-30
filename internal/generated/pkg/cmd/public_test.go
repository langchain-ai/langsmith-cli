// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestPublicRetrieveFeedbacks(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"public", "retrieve-feedbacks",
			"--max-items", "10",
			"--share-token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--has-comment=true",
			"--has-score=true",
			"--key", "[string, string]",
			"--level", "run",
			"--limit", "1",
			"--offset", "0",
			"--run", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--session", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--source", "[api, model]",
			"--user", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
		)
	})
}
