// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestCommitsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"commits", "create",
			"--owner", "owner",
			"--repo", "repo",
			"--description", "description",
			"--manifest", "{}",
			"--parent-commit", "parent_commit",
			"--skip-webhooks", "{}",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"description: description\n" +
			"manifest: {}\n" +
			"parent_commit: parent_commit\n" +
			"skip_webhooks: {}\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"commits", "create",
			"--owner", "owner",
			"--repo", "repo",
		)
	})
}

func TestCommitsRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"commits", "retrieve",
			"--owner", "owner",
			"--repo", "repo",
			"--commit", "commit",
			"--get-examples=true",
			"--include", "include",
			"--include-model=true",
			"--is-view=true",
		)
	})
}

func TestCommitsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"commits", "list",
			"--max-items", "10",
			"--owner", "owner",
			"--repo", "repo",
			"--include-stats=true",
			"--limit", "1",
			"--offset", "0",
			"--tag", "tag",
		)
	})
}
