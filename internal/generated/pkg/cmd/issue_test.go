// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestIssuesRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"issues", "retrieve",
			"--id", "id",
			"--include-linear-context=true",
		)
	})
}

func TestIssuesList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"issues", "list",
			"--max-items", "10",
			"--activity", "fixing",
			"--limit", "0",
			"--offset", "0",
			"--session-id", "session_id",
			"--session-name", "session_name",
			"--severity", "0",
			"--severity-exact", "0",
			"--sort-by", "default",
			"--status", "open",
			"--status-first=true",
			"--tag", "tag",
			"--trace-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--updated-at", "updated_at",
		)
	})
}
