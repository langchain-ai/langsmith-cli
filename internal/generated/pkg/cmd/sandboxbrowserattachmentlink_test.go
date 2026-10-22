// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestSandboxesBrowsersAttachmentsLinksCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers:attachments:links", "create",
			"--name", "name",
			"--attachment-id", "attachment_id",
			"--embed-origin", "string",
			"--ttl-seconds", "60",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"embed_origins:\n" +
			"  - string\n" +
			"ttl_seconds: 60\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers:attachments:links", "create",
			"--name", "name",
			"--attachment-id", "attachment_id",
		)
	})
}
