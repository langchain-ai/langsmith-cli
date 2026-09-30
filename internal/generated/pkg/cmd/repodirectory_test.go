// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestReposDirectoriesList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos:directories", "list",
			"--owner", "owner",
			"--repo", "repo",
			"--commit", "commit",
		)
	})
}

func TestReposDirectoriesDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos:directories", "delete",
			"--owner", "owner",
			"--repo", "repo",
			"--repo-type", "agent",
		)
	})
}

func TestReposDirectoriesCommit(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos:directories", "commit",
			"--owner", "owner",
			"--repo", "repo",
			"--files", "{agents/pinned: {repo_handle: review-agent, type: agent, commit_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, selector: {commit_id: 0198f3ab-7c2d-7def-8a91-23456789abcd, type: COMMIT}}, skills/current: {repo_handle: shared-skill, type: skill, commit_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, selector: {type: LATEST}}}",
			"--parent-commit", "parent_commit",
			"--skip-webhooks=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"files:\n" +
			"  agents/pinned:\n" +
			"    repo_handle: review-agent\n" +
			"    type: agent\n" +
			"    commit_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"    selector:\n" +
			"      commit_id: 0198f3ab-7c2d-7def-8a91-23456789abcd\n" +
			"      type: COMMIT\n" +
			"  skills/current:\n" +
			"    repo_handle: shared-skill\n" +
			"    type: skill\n" +
			"    commit_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"    selector:\n" +
			"      type: LATEST\n" +
			"parent_commit: parent_commit\n" +
			"skip_webhooks: true\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos:directories", "commit",
			"--owner", "owner",
			"--repo", "repo",
		)
	})
}
