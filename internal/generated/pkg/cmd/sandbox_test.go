// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestSandboxesListUsageCosts(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes", "list-usage-costs",
			"--max-items", "10",
			"--end-time", "'2019-12-27T18:11:19.117Z'",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--cursor", "cursor",
			"--granularity", "HOUR",
			"--page-size", "1",
			"--resource-id", "string",
			"--resource-type", "SANDBOX",
		)
	})
}
