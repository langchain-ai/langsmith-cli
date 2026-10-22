// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestSandboxesBoxesDeleteServiceURL(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "delete-service-url",
			"--name", "name",
			"--port", "0",
		)
	})
}

func TestSandboxesBoxesListServiceURLs(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "list-service-urls",
			"--max-items", "10",
			"--name", "name",
			"--cursor", "cursor",
			"--page-size", "0",
		)
	})
}
