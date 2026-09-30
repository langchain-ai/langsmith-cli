// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestPublicDatasetsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"public:datasets", "list",
			"--share-token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--limit", "1",
			"--offset", "0",
			"--sort-by", "name",
			"--sort-by-desc=true",
		)
	})
}

func TestPublicDatasetsListComparative(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"public:datasets", "list-comparative",
			"--max-items", "10",
			"--share-token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--limit", "1",
			"--name", "name",
			"--name-contains", "name_contains",
			"--offset", "0",
			"--sort-by", "name",
			"--sort-by-desc=true",
		)
	})
}

func TestPublicDatasetsListFeedback(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"public:datasets", "list-feedback",
			"--max-items", "10",
			"--share-token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--has-comment=true",
			"--has-score=true",
			"--key", "[string, string]",
			"--level", "run",
			"--limit", "1",
			"--offset", "0",
			"--run", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--session", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--source", "[api, model]",
			"--user", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
		)
	})
}

func TestPublicDatasetsListSessions(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"public:datasets", "list-sessions",
			"--max-items", "10",
			"--share-token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--id", "[182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e]",
			"--dataset-version", "dataset_version",
			"--facets=true",
			"--limit", "1",
			"--name", "name",
			"--name-contains", "name_contains",
			"--offset", "0",
			"--sort-by", "name",
			"--sort-by-desc=true",
			"--sort-by-feedback-key", "sort_by_feedback_key",
			"--sort-by-feedback-source", "sort_by_feedback_source",
			"--stats-filter", "stats_filter",
			"--stats-select", "[string, string]",
			"--stats-start-time", "'2019-12-27T18:11:19.117Z'",
			"--use-approx-stats=true",
			"--accept", "accept",
		)
	})
}

func TestPublicDatasetsRetrieveSessionsBulk(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"public:datasets", "retrieve-sessions-bulk",
			"--share-token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
