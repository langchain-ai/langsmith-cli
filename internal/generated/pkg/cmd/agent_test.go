// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestAgentsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"agents", "create",
			"--id", "id",
			"--source-type", "LSD",
			"--agent-type", "CUSTOM",
			"--name", "name",
			"--production-only=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"id: id\n" +
			"source_type: LSD\n" +
			"agent_type: CUSTOM\n" +
			"name: name\n" +
			"production_only: true\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"agents", "create",
		)
	})
}

func TestAgentsRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"agents", "retrieve",
			"--agent-id", "agent_id",
			"--include", "runtime_bindings",
		)
	})
}

func TestAgentsUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"agents", "update",
			"--agent-id", "agent_id",
			"--agent-type", "CUSTOM",
			"--name", "name",
			"--source-type", "LSD",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"agent_type: CUSTOM\n" +
			"name: name\n" +
			"source_type: LSD\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"agents", "update",
			"--agent-id", "agent_id",
		)
	})
}

func TestAgentsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"agents", "list",
			"--max-items", "10",
			"--id", "string",
			"--cursor", "cursor",
			"--name", "name",
			"--page-size", "1",
			"--sort-by", "last_run_start_time",
			"--sort-order", "asc",
			"--source-type", "LSD",
		)
	})
}

func TestAgentsDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"agents", "delete",
			"--agent-id", "agent_id",
		)
	})
}
