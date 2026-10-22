// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestPublicRunsRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"public:runs", "retrieve",
			"--share-token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--select", "string",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--accept", "Accept",
		)
	})
}

func TestPublicRunsQuery(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"public:runs", "query",
			"--share-token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--select", "ID",
			"--select", "NAME",
			"--select", "PROJECT_ID",
			"--select", "START_TIME",
			"--select", "RUN_TYPE",
			"--select", "STATUS",
			"--select", "INPUTS_PREVIEW",
			"--select", "OUTPUTS_PREVIEW",
			"--select", "METADATA",
			"--accept", "Accept",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"selects:\n" +
			"  - ID\n" +
			"  - NAME\n" +
			"  - PROJECT_ID\n" +
			"  - START_TIME\n" +
			"  - RUN_TYPE\n" +
			"  - STATUS\n" +
			"  - INPUTS_PREVIEW\n" +
			"  - OUTPUTS_PREVIEW\n" +
			"  - METADATA\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"public:runs", "query",
			"--share-token", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--accept", "Accept",
		)
	})
}
