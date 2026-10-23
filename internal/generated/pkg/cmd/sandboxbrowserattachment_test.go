// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestSandboxesBrowsersAttachmentsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers:attachments", "create",
			"--name", "name",
			"--controller", "agent",
			"--height", "300",
			"--window-name", "name",
			"--sandbox", "sandbox",
			"--share-localhost=true",
			"--unrestricted=true",
			"--width", "400",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"controller: agent\n" +
			"height: 300\n" +
			"name: name\n" +
			"sandbox: sandbox\n" +
			"share_localhost: true\n" +
			"unrestricted: true\n" +
			"width: 400\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers:attachments", "create",
			"--name", "name",
		)
	})
}

func TestSandboxesBrowsersAttachmentsUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers:attachments", "update",
			"--name", "name",
			"--attachment-id", "attachment_id",
			"--height", "300",
			"--width", "400",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"height: 300\n" +
			"width: 400\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers:attachments", "update",
			"--name", "name",
			"--attachment-id", "attachment_id",
		)
	})
}

func TestSandboxesBrowsersAttachmentsDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers:attachments", "delete",
			"--name", "name",
			"--attachment-id", "attachment_id",
		)
	})
}

func TestSandboxesBrowsersAttachmentsSetController(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers:attachments", "set-controller",
			"--name", "name",
			"--attachment-id", "attachment_id",
			"--controller", "agent",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("controller: agent")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers:attachments", "set-controller",
			"--name", "name",
			"--attachment-id", "attachment_id",
		)
	})
}
