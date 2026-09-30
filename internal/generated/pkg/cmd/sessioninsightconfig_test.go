// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestSessionsInsightsConfigsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights:configs", "create",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config", "{attribute_schemas: {foo: bar}, cluster_model: cluster_model, config_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, end_time: '2019-12-27T18:11:19.117Z', filter: filter, hierarchy: [0], is_scheduled: true, last_n_hours: 0, model: openai, name: name, partitions: {foo: string}, sample: 0, start_time: '2019-12-27T18:11:19.117Z', summary_model: summary_model, summary_prompt: summary_prompt, user_context: {foo: string}, validate_model_secrets: true}",
			"--name", "name",
			"--description", "description",
			"--schedule-cron", "schedule_cron",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(sessionsInsightsConfigsCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights:configs", "create",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config.attribute-schemas", "{foo: bar}",
			"--config.cluster-model", "cluster_model",
			"--config.config-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config.end-time", "2019-12-27T18:11:19.117Z",
			"--config.filter", "filter",
			"--config.hierarchy", "[0]",
			"--config.is-scheduled=true",
			"--config.last-n-hours", "0",
			"--config.model", "openai",
			"--config.name", "name",
			"--config.partitions", "{foo: string}",
			"--config.sample", "0",
			"--config.start-time", "2019-12-27T18:11:19.117Z",
			"--config.summary-model", "summary_model",
			"--config.summary-prompt", "summary_prompt",
			"--config.user-context", "{foo: string}",
			"--config.validate-model-secrets=true",
			"--name", "name",
			"--description", "description",
			"--schedule-cron", "schedule_cron",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"config:\n" +
			"  attribute_schemas:\n" +
			"    foo: bar\n" +
			"  cluster_model: cluster_model\n" +
			"  config_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  end_time: '2019-12-27T18:11:19.117Z'\n" +
			"  filter: filter\n" +
			"  hierarchy:\n" +
			"    - 0\n" +
			"  is_scheduled: true\n" +
			"  last_n_hours: 0\n" +
			"  model: openai\n" +
			"  name: name\n" +
			"  partitions:\n" +
			"    foo: string\n" +
			"  sample: 0\n" +
			"  start_time: '2019-12-27T18:11:19.117Z'\n" +
			"  summary_model: summary_model\n" +
			"  summary_prompt: summary_prompt\n" +
			"  user_context:\n" +
			"    foo: string\n" +
			"  validate_model_secrets: true\n" +
			"name: name\n" +
			"description: description\n" +
			"schedule_cron: schedule_cron\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights:configs", "create",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestSessionsInsightsConfigsUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights:configs", "update",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config", "{attribute_schemas: {foo: bar}, cluster_model: cluster_model, config_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, end_time: '2019-12-27T18:11:19.117Z', filter: filter, hierarchy: [0], is_scheduled: true, last_n_hours: 0, model: openai, name: name, partitions: {foo: string}, sample: 0, start_time: '2019-12-27T18:11:19.117Z', summary_model: summary_model, summary_prompt: summary_prompt, user_context: {foo: string}, validate_model_secrets: true}",
			"--description", "description",
			"--name", "name",
			"--schedule-cron", "schedule_cron",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(sessionsInsightsConfigsUpdate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights:configs", "update",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config.attribute-schemas", "{foo: bar}",
			"--config.cluster-model", "cluster_model",
			"--config.config-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config.end-time", "2019-12-27T18:11:19.117Z",
			"--config.filter", "filter",
			"--config.hierarchy", "[0]",
			"--config.is-scheduled=true",
			"--config.last-n-hours", "0",
			"--config.model", "openai",
			"--config.name", "name",
			"--config.partitions", "{foo: string}",
			"--config.sample", "0",
			"--config.start-time", "2019-12-27T18:11:19.117Z",
			"--config.summary-model", "summary_model",
			"--config.summary-prompt", "summary_prompt",
			"--config.user-context", "{foo: string}",
			"--config.validate-model-secrets=true",
			"--description", "description",
			"--name", "name",
			"--schedule-cron", "schedule_cron",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"config:\n" +
			"  attribute_schemas:\n" +
			"    foo: bar\n" +
			"  cluster_model: cluster_model\n" +
			"  config_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  end_time: '2019-12-27T18:11:19.117Z'\n" +
			"  filter: filter\n" +
			"  hierarchy:\n" +
			"    - 0\n" +
			"  is_scheduled: true\n" +
			"  last_n_hours: 0\n" +
			"  model: openai\n" +
			"  name: name\n" +
			"  partitions:\n" +
			"    foo: string\n" +
			"  sample: 0\n" +
			"  start_time: '2019-12-27T18:11:19.117Z'\n" +
			"  summary_model: summary_model\n" +
			"  summary_prompt: summary_prompt\n" +
			"  user_context:\n" +
			"    foo: string\n" +
			"  validate_model_secrets: true\n" +
			"description: description\n" +
			"name: name\n" +
			"schedule_cron: schedule_cron\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights:configs", "update",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestSessionsInsightsConfigsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights:configs", "list",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--include-prebuilts=true",
		)
	})
}

func TestSessionsInsightsConfigsDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions:insights:configs", "delete",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--config-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}
