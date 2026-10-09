// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestPromptWebhooksCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"prompt-webhooks", "create",
			"--url", "https://example.com",
			"--id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--exclude-prompt", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--headers", "{foo: bar}",
			"--include-prompt", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--trigger", "commit",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"url: https://example.com\n" +
			"id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"exclude_prompts:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"headers:\n" +
			"  foo: bar\n" +
			"include_prompts:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"triggers:\n" +
			"  - commit\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"prompt-webhooks", "create",
		)
	})
}

func TestPromptWebhooksRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"prompt-webhooks", "retrieve",
			"--webhook-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestPromptWebhooksUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"prompt-webhooks", "update",
			"--webhook-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--exclude-prompt", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--headers", "{foo: bar}",
			"--include-prompt", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--trigger", "[commit]",
			"--url", "https://example.com",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"exclude_prompts:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"headers:\n" +
			"  foo: bar\n" +
			"include_prompts:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"triggers:\n" +
			"  - commit\n" +
			"url: https://example.com\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"prompt-webhooks", "update",
			"--webhook-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestPromptWebhooksList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"prompt-webhooks", "list",
		)
	})
}

func TestPromptWebhooksDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"prompt-webhooks", "delete",
			"--webhook-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestPromptWebhooksTest(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"prompt-webhooks", "test",
			"--payload", "{commit_hash: commit_hash, created_at: created_at, created_by: created_by, event: commit, manifest: {foo: bar}, prompt_id: prompt_id, prompt_name: prompt_name, tag_name: tag_name}",
			"--webhook", "{url: https://example.com, exclude_prompts: [182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e], headers: {foo: bar}, include_prompts: [182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e], triggers: [commit]}",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(promptWebhooksTest)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"prompt-webhooks", "test",
			"--payload.commit-hash", "commit_hash",
			"--payload.created-at", "created_at",
			"--payload.created-by", "created_by",
			"--payload.event", "commit",
			"--payload.manifest", "{foo: bar}",
			"--payload.prompt-id", "prompt_id",
			"--payload.prompt-name", "prompt_name",
			"--payload.tag-name", "tag_name",
			"--webhook.url", "https://example.com",
			"--webhook.exclude-prompts", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--webhook.headers", "{foo: bar}",
			"--webhook.include-prompts", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--webhook.triggers", "[commit]",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"payload:\n" +
			"  commit_hash: commit_hash\n" +
			"  created_at: created_at\n" +
			"  created_by: created_by\n" +
			"  event: commit\n" +
			"  manifest:\n" +
			"    foo: bar\n" +
			"  prompt_id: prompt_id\n" +
			"  prompt_name: prompt_name\n" +
			"  tag_name: tag_name\n" +
			"webhook:\n" +
			"  url: https://example.com\n" +
			"  exclude_prompts:\n" +
			"    - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  headers:\n" +
			"    foo: bar\n" +
			"  include_prompts:\n" +
			"    - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  triggers:\n" +
			"    - commit\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"prompt-webhooks", "test",
		)
	})
}
