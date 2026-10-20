// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestRunsShareCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs:share", "create",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--session-id", "018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327",
			"--trace-id", "018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"session_id: 018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327\n" +
			"trace_id: 018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs:share", "create",
			"--run-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestRunsShareDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs:share", "delete",
			"--trace-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--session-id", "018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("session_id: 018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"runs:share", "delete",
			"--trace-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
