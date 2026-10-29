// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestSandboxesBrowsersCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers", "create",
			"--hidpi=true",
			"--idle-ttl-seconds", "0",
			"--mem-bytes", "0",
			"--name", "name",
			"--proxy-config", "{access_control: {allow_list: [string], deny_list: [string]}, callbacks: [{match_hosts: [string], ttl_seconds: 60, url: url, full_request: true, request_headers: [{name: name, type: plaintext, is_set: true, value: value}]}], description: description, no_proxy: [string], rules: [{name: name, aws: {role_arn: x, subject_scope: tag:2fdcD1Dc-bbDb-2BBD-0Afa-1A3C33cFaADc}, azure: {client_id: E1CB97d8-Ddf3-aAae-0Afa-1A3C33cFaADc, tenant_id: E1CB97d8-Ddf3-aAae-0Afa-1A3C33cFaADc, resources: [https://storage.azure.com], subject_scope: tag:2fdcD1Dc-bbDb-2BBD-0Afa-1A3C33cFaADc}, description: description, enabled: true, env_vars: {foo: string}, gcp: {workload_identity_provider: x, scopes: [string], service_account_email: service_account_email, subject_scope: tag:2fdcD1Dc-bbDb-2BBD-0Afa-1A3C33cFaADc}, headers: [{name: name, type: plaintext, is_set: true, value: value}], match_headers: [string], match_hosts: [string], match_paths: [string], relay_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, rewrite: {port: 1, credentials: strip, host: host, sandbox: sandbox}, type: type}]}",
			"--vcpus", "0",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(sandboxesBrowsersCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers", "create",
			"--hidpi=true",
			"--idle-ttl-seconds", "0",
			"--mem-bytes", "0",
			"--name", "name",
			"--proxy-config.access-control", "{allow_list: [string], deny_list: [string]}",
			"--proxy-config.callbacks", "[{match_hosts: [string], ttl_seconds: 60, url: url, full_request: true, request_headers: [{name: name, type: plaintext, is_set: true, value: value}]}]",
			"--proxy-config.description", "description",
			"--proxy-config.no-proxy", "[string]",
			"--proxy-config.rules", "[{name: name, aws: {role_arn: x, subject_scope: tag:2fdcD1Dc-bbDb-2BBD-0Afa-1A3C33cFaADc}, azure: {client_id: E1CB97d8-Ddf3-aAae-0Afa-1A3C33cFaADc, tenant_id: E1CB97d8-Ddf3-aAae-0Afa-1A3C33cFaADc, resources: [https://storage.azure.com], subject_scope: tag:2fdcD1Dc-bbDb-2BBD-0Afa-1A3C33cFaADc}, description: description, enabled: true, env_vars: {foo: string}, gcp: {workload_identity_provider: x, scopes: [string], service_account_email: service_account_email, subject_scope: tag:2fdcD1Dc-bbDb-2BBD-0Afa-1A3C33cFaADc}, headers: [{name: name, type: plaintext, is_set: true, value: value}], match_headers: [string], match_hosts: [string], match_paths: [string], relay_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, rewrite: {port: 1, credentials: strip, host: host, sandbox: sandbox}, type: type}]",
			"--vcpus", "0",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"hidpi: true\n" +
			"idle_ttl_seconds: 0\n" +
			"mem_bytes: 0\n" +
			"name: name\n" +
			"proxy_config:\n" +
			"  access_control:\n" +
			"    allow_list:\n" +
			"      - string\n" +
			"    deny_list:\n" +
			"      - string\n" +
			"  callbacks:\n" +
			"    - match_hosts:\n" +
			"        - string\n" +
			"      ttl_seconds: 60\n" +
			"      url: url\n" +
			"      full_request: true\n" +
			"      request_headers:\n" +
			"        - name: name\n" +
			"          type: plaintext\n" +
			"          is_set: true\n" +
			"          value: value\n" +
			"  description: description\n" +
			"  no_proxy:\n" +
			"    - string\n" +
			"  rules:\n" +
			"    - name: name\n" +
			"      aws:\n" +
			"        role_arn: x\n" +
			"        subject_scope: tag:2fdcD1Dc-bbDb-2BBD-0Afa-1A3C33cFaADc\n" +
			"      azure:\n" +
			"        client_id: E1CB97d8-Ddf3-aAae-0Afa-1A3C33cFaADc\n" +
			"        tenant_id: E1CB97d8-Ddf3-aAae-0Afa-1A3C33cFaADc\n" +
			"        resources:\n" +
			"          - https://storage.azure.com\n" +
			"        subject_scope: tag:2fdcD1Dc-bbDb-2BBD-0Afa-1A3C33cFaADc\n" +
			"      description: description\n" +
			"      enabled: true\n" +
			"      env_vars:\n" +
			"        foo: string\n" +
			"      gcp:\n" +
			"        workload_identity_provider: x\n" +
			"        scopes:\n" +
			"          - string\n" +
			"        service_account_email: service_account_email\n" +
			"        subject_scope: tag:2fdcD1Dc-bbDb-2BBD-0Afa-1A3C33cFaADc\n" +
			"      headers:\n" +
			"        - name: name\n" +
			"          type: plaintext\n" +
			"          is_set: true\n" +
			"          value: value\n" +
			"      match_headers:\n" +
			"        - string\n" +
			"      match_hosts:\n" +
			"        - string\n" +
			"      match_paths:\n" +
			"        - string\n" +
			"      relay_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"      rewrite:\n" +
			"        port: 1\n" +
			"        credentials: strip\n" +
			"        host: host\n" +
			"        sandbox: sandbox\n" +
			"      type: type\n" +
			"vcpus: 0\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers", "create",
		)
	})
}

func TestSandboxesBrowsersRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers", "retrieve",
			"--name", "name",
		)
	})
}

func TestSandboxesBrowsersLoadExtensions(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:browsers", "load-extensions",
			"--name", "name",
		)
	})
}
