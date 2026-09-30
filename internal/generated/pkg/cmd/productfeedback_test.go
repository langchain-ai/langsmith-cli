// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestProductFeedbackCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"product-feedback", "create",
			"--category", "BUG",
			"--message", "x",
			"--source", "LANGSMITH_CLI",
			"--client", "{architecture: architecture, os: os, version: version}",
			"--idempotency-key", "Idempotency-Key",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(productFeedbackCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"product-feedback", "create",
			"--category", "BUG",
			"--message", "x",
			"--source", "LANGSMITH_CLI",
			"--client.architecture", "architecture",
			"--client.os", "os",
			"--client.version", "version",
			"--idempotency-key", "Idempotency-Key",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"category: BUG\n" +
			"message: x\n" +
			"source: LANGSMITH_CLI\n" +
			"client:\n" +
			"  architecture: architecture\n" +
			"  os: os\n" +
			"  version: version\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"product-feedback", "create",
			"--idempotency-key", "Idempotency-Key",
		)
	})
}

func TestProductFeedbackRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"product-feedback", "retrieve",
			"--id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
