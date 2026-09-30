// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestReposCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos", "create",
			"--is-public=true",
			"--repo-handle", "repo_handle",
			"--description", "description",
			"--readme", "readme",
			"--repo-type", "prompt",
			"--restricted-mode=true",
			"--source", "internal",
			"--tag-value-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--tag", "[string]",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"is_public: true\n" +
			"repo_handle: repo_handle\n" +
			"description: description\n" +
			"readme: readme\n" +
			"repo_type: prompt\n" +
			"restricted_mode: true\n" +
			"source: internal\n" +
			"tag_value_ids:\n" +
			"  - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"tags:\n" +
			"  - string\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos", "create",
		)
	})
}

func TestReposRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos", "retrieve",
			"--owner", "owner",
			"--repo", "repo",
		)
	})
}

func TestReposUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos", "update",
			"--owner", "owner",
			"--repo", "repo",
			"--description", "description",
			"--is-archived=true",
			"--is-public=true",
			"--readme", "readme",
			"--restricted-mode=true",
			"--tag", "[string]",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"description: description\n" +
			"is_archived: true\n" +
			"is_public: true\n" +
			"readme: readme\n" +
			"restricted_mode: true\n" +
			"tags:\n" +
			"  - string\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos", "update",
			"--owner", "owner",
			"--repo", "repo",
		)
	})
}

func TestReposList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos", "list",
			"--max-items", "10",
			"--has-commits=true",
			"--include-owners=true",
			"--is-archived", "true",
			"--is-public", "true",
			"--limit", "1",
			"--offset", "0",
			"--query", "query",
			"--single-repo-type", "prompt",
			"--repo-type", "[prompt, file]",
			"--sort-direction", "asc",
			"--sort-field", "num_likes",
			"--source", "internal",
			"--tag-value-id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--tag", "[string, string]",
			"--tenant-handle", "tenant_handle",
			"--tenant-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--upstream-repo-handle", "upstream_repo_handle",
			"--upstream-repo-owner", "upstream_repo_owner",
			"--with-latest-manifest=true",
		)
	})
}

func TestReposDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"repos", "delete",
			"--owner", "owner",
			"--repo", "repo",
		)
	})
}
