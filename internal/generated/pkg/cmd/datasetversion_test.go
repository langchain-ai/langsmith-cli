// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestDatasetsVersionsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:versions", "list",
			"--max-items", "10",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--example", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--limit", "1",
			"--offset", "0",
			"--search", "search",
		)
	})
}

func TestDatasetsVersionsRetrieveDiff(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"datasets:versions", "retrieve-diff",
			"--dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--from-version", "'2019-12-27T18:11:19.117Z'",
			"--to-version", "'2019-12-27T18:11:19.117Z'",
		)
	})
}
