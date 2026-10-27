// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestOnlineEvaluatorsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"online-evaluators", "create",
			"--code-evaluator", "{advanced_features_enabled: true, code: code, code_evaluator_input: thread, dependencies: dependencies, language: language, managed_code_evaluator_key: voice_metrics, managed_code_evaluator_settings: {foo: {is_enabled: true, key_name: key_name}}, require_attachments: true, workspace_secrets_keys: [string]}",
			"--llm-evaluator", "{commit_hash_or_tag: commit_hash_or_tag, playground_settings_id: playground_settings_id, prompt_repo_handle: prompt_repo_handle, variable_mapping: {}}",
			"--name", "name",
			"--type", "llm",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(onlineEvaluatorsCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"online-evaluators", "create",
			"--code-evaluator.advanced-features-enabled=true",
			"--code-evaluator.code", "code",
			"--code-evaluator.code-evaluator-input", "thread",
			"--code-evaluator.dependencies", "dependencies",
			"--code-evaluator.language", "language",
			"--code-evaluator.managed-code-evaluator-key", "voice_metrics",
			"--code-evaluator.managed-code-evaluator-settings", "{foo: {is_enabled: true, key_name: key_name}}",
			"--code-evaluator.require-attachments=true",
			"--code-evaluator.workspace-secrets-keys", "[string]",
			"--llm-evaluator.commit-hash-or-tag", "commit_hash_or_tag",
			"--llm-evaluator.playground-settings-id", "playground_settings_id",
			"--llm-evaluator.prompt-repo-handle", "prompt_repo_handle",
			"--llm-evaluator.variable-mapping", "{}",
			"--name", "name",
			"--type", "llm",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"code_evaluator:\n" +
			"  advanced_features_enabled: true\n" +
			"  code: code\n" +
			"  code_evaluator_input: thread\n" +
			"  dependencies: dependencies\n" +
			"  language: language\n" +
			"  managed_code_evaluator_key: voice_metrics\n" +
			"  managed_code_evaluator_settings:\n" +
			"    foo:\n" +
			"      is_enabled: true\n" +
			"      key_name: key_name\n" +
			"  require_attachments: true\n" +
			"  workspace_secrets_keys:\n" +
			"    - string\n" +
			"llm_evaluator:\n" +
			"  commit_hash_or_tag: commit_hash_or_tag\n" +
			"  playground_settings_id: playground_settings_id\n" +
			"  prompt_repo_handle: prompt_repo_handle\n" +
			"  variable_mapping: {}\n" +
			"name: name\n" +
			"type: llm\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"online-evaluators", "create",
		)
	})
}

func TestOnlineEvaluatorsUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"online-evaluators", "update",
			"--evaluator-id", "evaluator_id",
			"--code-evaluator", "{advanced_features_enabled: true, code: code, code_evaluator_input: thread, dependencies: dependencies, language: language, managed_code_evaluator_settings: {foo: {is_enabled: true, key_name: key_name}}, require_attachments: true, workspace_secrets_keys: [string]}",
			"--llm-evaluator", "{commit_hash_or_tag: commit_hash_or_tag, num_few_shot_examples: 0, playground_settings_id: playground_settings_id, prompt_repo_handle: prompt_repo_handle, use_corrections_dataset: true, variable_mapping: {}}",
			"--name", "name",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(onlineEvaluatorsUpdate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"online-evaluators", "update",
			"--evaluator-id", "evaluator_id",
			"--code-evaluator.advanced-features-enabled=true",
			"--code-evaluator.code", "code",
			"--code-evaluator.code-evaluator-input", "thread",
			"--code-evaluator.dependencies", "dependencies",
			"--code-evaluator.language", "language",
			"--code-evaluator.managed-code-evaluator-settings", "{foo: {is_enabled: true, key_name: key_name}}",
			"--code-evaluator.require-attachments=true",
			"--code-evaluator.workspace-secrets-keys", "[string]",
			"--llm-evaluator.commit-hash-or-tag", "commit_hash_or_tag",
			"--llm-evaluator.num-few-shot-examples", "0",
			"--llm-evaluator.playground-settings-id", "playground_settings_id",
			"--llm-evaluator.prompt-repo-handle", "prompt_repo_handle",
			"--llm-evaluator.use-corrections-dataset=true",
			"--llm-evaluator.variable-mapping", "{}",
			"--name", "name",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"code_evaluator:\n" +
			"  advanced_features_enabled: true\n" +
			"  code: code\n" +
			"  code_evaluator_input: thread\n" +
			"  dependencies: dependencies\n" +
			"  language: language\n" +
			"  managed_code_evaluator_settings:\n" +
			"    foo:\n" +
			"      is_enabled: true\n" +
			"      key_name: key_name\n" +
			"  require_attachments: true\n" +
			"  workspace_secrets_keys:\n" +
			"    - string\n" +
			"llm_evaluator:\n" +
			"  commit_hash_or_tag: commit_hash_or_tag\n" +
			"  num_few_shot_examples: 0\n" +
			"  playground_settings_id: playground_settings_id\n" +
			"  prompt_repo_handle: prompt_repo_handle\n" +
			"  use_corrections_dataset: true\n" +
			"  variable_mapping: {}\n" +
			"name: name\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"online-evaluators", "update",
			"--evaluator-id", "evaluator_id",
		)
	})
}

func TestOnlineEvaluatorsBulkDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"online-evaluators", "bulk-delete",
			"--evaluator-id", "string",
			"--delete-run-rules=true",
		)
	})
}

func TestOnlineEvaluatorsSpend(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"online-evaluators", "spend",
			"--period-start", "period_start",
			"--agent-id", "agent_id",
			"--dataset-id", "dataset_id",
			"--evaluator-id", "evaluator_id",
			"--feedback-key", "feedback_key",
			"--group-by", "group_by",
			"--resource-id", "string",
			"--session-id", "session_id",
			"--tag-value-id", "string",
			"--type", "type",
		)
	})
}
