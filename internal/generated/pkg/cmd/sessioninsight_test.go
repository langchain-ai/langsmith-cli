// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestSessionsInsightsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights", "create",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--attribute-schemas", "{foo: bar}",
			"--cluster-model", "cluster_model",
			"--config-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--end-time", "'2019-12-27T18:11:19.117Z'",
			"--filter", "filter",
			"--hierarchy", "[0]",
			"--is-scheduled=true",
			"--last-n-hours", "0",
			"--model", "openai",
			"--name", "name",
			"--partitions", "{foo: string}",
			"--sample", "0",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--summary-model", "summary_model",
			"--summary-prompt", "summary_prompt",
			"--user-context", "{foo: string}",
			"--validate-model-secrets=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"attribute_schemas:\n" +
			"  foo: bar\n" +
			"cluster_model: cluster_model\n" +
			"config_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"end_time: '2019-12-27T18:11:19.117Z'\n" +
			"filter: filter\n" +
			"hierarchy:\n" +
			"  - 0\n" +
			"is_scheduled: true\n" +
			"last_n_hours: 0\n" +
			"model: openai\n" +
			"name: name\n" +
			"partitions:\n" +
			"  foo: string\n" +
			"sample: 0\n" +
			"start_time: '2019-12-27T18:11:19.117Z'\n" +
			"summary_model: summary_model\n" +
			"summary_prompt: summary_prompt\n" +
			"user_context:\n" +
			"  foo: string\n" +
			"validate_model_secrets: true\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights", "create",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestSessionsInsightsUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights", "update",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--job-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--name", "name",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("name: name")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights", "update",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--job-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestSessionsInsightsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights", "list",
			"--max-items", "10",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--legacy=true",
			"--limit", "1",
			"--offset", "0",
		)
	})
}

func TestSessionsInsightsDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights", "delete",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--job-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestSessionsInsightsRetrieveJob(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights", "retrieve-job",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--job-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestSessionsInsightsRetrieveRuns(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights", "retrieve-runs",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--job-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--attribute-sort-key", "attribute_sort_key",
			"--attribute-sort-order", "asc",
			"--cluster-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--limit", "1",
			"--offset", "0",
		)
	})
}
