// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestSandboxesBoxesCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "create",
			"--access-delegation", "{mode: INHERIT, permissions: [string]}",
			"--cpu-millicores", "0",
			"--delete-after-stop-seconds", "0",
			"--env-vars", "{foo: string}",
			"--fs-capacity-bytes", "0",
			"--idle-ttl-seconds", "0",
			"--labels", "{foo: string}",
			"--mem-bytes", "0",
			"--mount-config", "{auth: {aws: {role_arn: x}, gcp: {service_account_json: {type: plaintext, is_set: true, value: value}}}, mounts: [{id: id, mount_path: mount_path, s3: {bucket: bucket, region: region, endpoint_url: endpoint_url, path_style: true, prefix: prefix}, type: s3, cache: {max_size_bytes: 0, writeback_seconds: 0}, contexthub: {repo: repo, initial_pull_only: true}, gcs: {bucket: bucket, prefix: prefix}, git: {remote_url: remote_url, ref: {name: name, type: branch}, refresh_interval_seconds: 1}, read_only: true}]}",
			"--name", "name",
			"--preserve-memory-on-stop=true",
			"--proxy-config", "{access_control: {allow_list: [string], deny_list: [string]}, callbacks: [{match_hosts: [string], ttl_seconds: 60, url: url, full_request: true, request_headers: [{name: name, type: plaintext, is_set: true, value: value}]}], description: description, no_proxy: [string], rules: [{name: name, aws: {role_arn: x}, description: description, enabled: true, env_vars: {foo: string}, gcp: {scopes: [string], service_account_json: {type: plaintext, is_set: true, value: value}}, headers: [{name: name, type: plaintext, is_set: true, value: value}], match_hosts: [string], match_paths: [string], type: type}]}",
			"--restore-memory=true",
			"--run-config", "{env_vars: {foo: string}, user: user, work_dir: work_dir}",
			"--snapshot", "snapshot",
			"--snapshot-id", "snapshot_id",
			"--snapshot-name", "snapshot_name",
			"--tag-value-id", "string",
			"--vcpus", "0",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(sandboxesBoxesCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "create",
			"--access-delegation.mode", "INHERIT",
			"--access-delegation.permissions", "[string]",
			"--cpu-millicores", "0",
			"--delete-after-stop-seconds", "0",
			"--env-vars", "{foo: string}",
			"--fs-capacity-bytes", "0",
			"--idle-ttl-seconds", "0",
			"--labels", "{foo: string}",
			"--mem-bytes", "0",
			"--mount-config.auth", "{aws: {role_arn: x}, gcp: {service_account_json: {type: plaintext, is_set: true, value: value}}}",
			"--mount-config.mounts", "[{id: id, mount_path: mount_path, s3: {bucket: bucket, region: region, endpoint_url: endpoint_url, path_style: true, prefix: prefix}, type: s3, cache: {max_size_bytes: 0, writeback_seconds: 0}, contexthub: {repo: repo, initial_pull_only: true}, gcs: {bucket: bucket, prefix: prefix}, git: {remote_url: remote_url, ref: {name: name, type: branch}, refresh_interval_seconds: 1}, read_only: true}]",
			"--name", "name",
			"--preserve-memory-on-stop=true",
			"--proxy-config.access-control", "{allow_list: [string], deny_list: [string]}",
			"--proxy-config.callbacks", "[{match_hosts: [string], ttl_seconds: 60, url: url, full_request: true, request_headers: [{name: name, type: plaintext, is_set: true, value: value}]}]",
			"--proxy-config.description", "description",
			"--proxy-config.no-proxy", "[string]",
			"--proxy-config.rules", "[{name: name, aws: {role_arn: x}, description: description, enabled: true, env_vars: {foo: string}, gcp: {scopes: [string], service_account_json: {type: plaintext, is_set: true, value: value}}, headers: [{name: name, type: plaintext, is_set: true, value: value}], match_hosts: [string], match_paths: [string], type: type}]",
			"--restore-memory=true",
			"--run-config.env-vars", "{foo: string}",
			"--run-config.user", "user",
			"--run-config.work-dir", "work_dir",
			"--snapshot", "snapshot",
			"--snapshot-id", "snapshot_id",
			"--snapshot-name", "snapshot_name",
			"--tag-value-id", "string",
			"--vcpus", "0",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"access_delegation:\n" +
			"  mode: INHERIT\n" +
			"  permissions:\n" +
			"    - string\n" +
			"cpu_millicores: 0\n" +
			"delete_after_stop_seconds: 0\n" +
			"env_vars:\n" +
			"  foo: string\n" +
			"fs_capacity_bytes: 0\n" +
			"idle_ttl_seconds: 0\n" +
			"labels:\n" +
			"  foo: string\n" +
			"mem_bytes: 0\n" +
			"mount_config:\n" +
			"  auth:\n" +
			"    aws:\n" +
			"      role_arn: x\n" +
			"    gcp:\n" +
			"      service_account_json:\n" +
			"        type: plaintext\n" +
			"        is_set: true\n" +
			"        value: value\n" +
			"  mounts:\n" +
			"    - id: id\n" +
			"      mount_path: mount_path\n" +
			"      s3:\n" +
			"        bucket: bucket\n" +
			"        region: region\n" +
			"        endpoint_url: endpoint_url\n" +
			"        path_style: true\n" +
			"        prefix: prefix\n" +
			"      type: s3\n" +
			"      cache:\n" +
			"        max_size_bytes: 0\n" +
			"        writeback_seconds: 0\n" +
			"      contexthub:\n" +
			"        repo: repo\n" +
			"        initial_pull_only: true\n" +
			"      gcs:\n" +
			"        bucket: bucket\n" +
			"        prefix: prefix\n" +
			"      git:\n" +
			"        remote_url: remote_url\n" +
			"        ref:\n" +
			"          name: name\n" +
			"          type: branch\n" +
			"        refresh_interval_seconds: 1\n" +
			"      read_only: true\n" +
			"name: name\n" +
			"preserve_memory_on_stop: true\n" +
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
			"      description: description\n" +
			"      enabled: true\n" +
			"      env_vars:\n" +
			"        foo: string\n" +
			"      gcp:\n" +
			"        scopes:\n" +
			"          - string\n" +
			"        service_account_json:\n" +
			"          type: plaintext\n" +
			"          is_set: true\n" +
			"          value: value\n" +
			"      headers:\n" +
			"        - name: name\n" +
			"          type: plaintext\n" +
			"          is_set: true\n" +
			"          value: value\n" +
			"      match_hosts:\n" +
			"        - string\n" +
			"      match_paths:\n" +
			"        - string\n" +
			"      type: type\n" +
			"restore_memory: true\n" +
			"run_config:\n" +
			"  env_vars:\n" +
			"    foo: string\n" +
			"  user: user\n" +
			"  work_dir: work_dir\n" +
			"snapshot: snapshot\n" +
			"snapshot_id: snapshot_id\n" +
			"snapshot_name: snapshot_name\n" +
			"tag_value_ids:\n" +
			"  - string\n" +
			"vcpus: 0\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "create",
		)
	})
}

func TestSandboxesBoxesRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "retrieve",
			"--name", "name",
		)
	})
}

func TestSandboxesBoxesUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "update",
			"--name", "name",
			"--cpu-millicores", "0",
			"--delete-after-stop-seconds", "0",
			"--fs-capacity-bytes", "0",
			"--idle-ttl-seconds", "0",
			"--mem-bytes", "0",
			"--name", "name",
			"--proxy-config", "{access_control: {allow_list: [string], deny_list: [string]}, callbacks: [{match_hosts: [string], ttl_seconds: 60, url: url, full_request: true, request_headers: [{name: name, type: plaintext, is_set: true, value: value}]}], description: description, no_proxy: [string], rules: [{name: name, aws: {role_arn: x}, description: description, enabled: true, env_vars: {foo: string}, gcp: {scopes: [string], service_account_json: {type: plaintext, is_set: true, value: value}}, headers: [{name: name, type: plaintext, is_set: true, value: value}], match_hosts: [string], match_paths: [string], type: type}]}",
			"--run-config", "{env_vars: {foo: string}, user: user, work_dir: work_dir}",
			"--tag-value-id", "string",
			"--vcpus", "0",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(sandboxesBoxesUpdate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "update",
			"--name", "name",
			"--cpu-millicores", "0",
			"--delete-after-stop-seconds", "0",
			"--fs-capacity-bytes", "0",
			"--idle-ttl-seconds", "0",
			"--mem-bytes", "0",
			"--name", "name",
			"--proxy-config.access-control", "{allow_list: [string], deny_list: [string]}",
			"--proxy-config.callbacks", "[{match_hosts: [string], ttl_seconds: 60, url: url, full_request: true, request_headers: [{name: name, type: plaintext, is_set: true, value: value}]}]",
			"--proxy-config.description", "description",
			"--proxy-config.no-proxy", "[string]",
			"--proxy-config.rules", "[{name: name, aws: {role_arn: x}, description: description, enabled: true, env_vars: {foo: string}, gcp: {scopes: [string], service_account_json: {type: plaintext, is_set: true, value: value}}, headers: [{name: name, type: plaintext, is_set: true, value: value}], match_hosts: [string], match_paths: [string], type: type}]",
			"--run-config.env-vars", "{foo: string}",
			"--run-config.user", "user",
			"--run-config.work-dir", "work_dir",
			"--tag-value-id", "string",
			"--vcpus", "0",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"cpu_millicores: 0\n" +
			"delete_after_stop_seconds: 0\n" +
			"fs_capacity_bytes: 0\n" +
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
			"      description: description\n" +
			"      enabled: true\n" +
			"      env_vars:\n" +
			"        foo: string\n" +
			"      gcp:\n" +
			"        scopes:\n" +
			"          - string\n" +
			"        service_account_json:\n" +
			"          type: plaintext\n" +
			"          is_set: true\n" +
			"          value: value\n" +
			"      headers:\n" +
			"        - name: name\n" +
			"          type: plaintext\n" +
			"          is_set: true\n" +
			"          value: value\n" +
			"      match_hosts:\n" +
			"        - string\n" +
			"      match_paths:\n" +
			"        - string\n" +
			"      type: type\n" +
			"run_config:\n" +
			"  env_vars:\n" +
			"    foo: string\n" +
			"  user: user\n" +
			"  work_dir: work_dir\n" +
			"tag_value_ids:\n" +
			"  - string\n" +
			"vcpus: 0\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "update",
			"--name", "name",
		)
	})
}

func TestSandboxesBoxesList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "list",
			"--max-items", "10",
			"--created-by", "created_by",
			"--cursor", "cursor",
			"--label", "string",
			"--limit", "0",
			"--name-contains", "name_contains",
			"--offset", "0",
			"--page-size", "0",
			"--sort-by", "sort_by",
			"--sort-direction", "sort_direction",
			"--sort-order", "sort_order",
			"--status", "status",
			"--tag-value-id", "string",
		)
	})
}

func TestSandboxesBoxesDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "delete",
			"--name", "name",
		)
	})
}

func TestSandboxesBoxesCreateSnapshot(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "create-snapshot",
			"--name", "name",
			"--name", "name",
			"--checkpoint", "checkpoint",
			"--description", "description",
			"--docker-image", "docker_image",
			"--fs-capacity-bytes", "0",
			"--include-memory=true",
			"--labels", "{foo: string}",
			"--run-config", "{env_vars: {foo: string}, user: user, work_dir: work_dir}",
			"--tag", "tag",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(sandboxesBoxesCreateSnapshot)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "create-snapshot",
			"--name", "name",
			"--name", "name",
			"--checkpoint", "checkpoint",
			"--description", "description",
			"--docker-image", "docker_image",
			"--fs-capacity-bytes", "0",
			"--include-memory=true",
			"--labels", "{foo: string}",
			"--run-config.env-vars", "{foo: string}",
			"--run-config.user", "user",
			"--run-config.work-dir", "work_dir",
			"--tag", "tag",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"name: name\n" +
			"checkpoint: checkpoint\n" +
			"description: description\n" +
			"docker_image: docker_image\n" +
			"fs_capacity_bytes: 0\n" +
			"include_memory: true\n" +
			"labels:\n" +
			"  foo: string\n" +
			"run_config:\n" +
			"  env_vars:\n" +
			"    foo: string\n" +
			"  user: user\n" +
			"  work_dir: work_dir\n" +
			"tag: tag\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "create-snapshot",
			"--name", "name",
		)
	})
}

func TestSandboxesBoxesDeleteServiceURL(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "delete-service-url",
			"--name", "name",
			"--port", "0",
		)
	})
}

func TestSandboxesBoxesGenerateDownloadURL(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "generate-download-url",
			"--name", "name",
			"--path", "path",
			"--content-disposition", "content_disposition",
			"--content-type", "content_type",
			"--csp-sandbox=true",
			"--csp-sandbox-flag", "allow-downloads",
			"--csp-source-bundle", "cdnjs",
			"--expires-in-seconds", "0",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"path: path\n" +
			"content_disposition: content_disposition\n" +
			"content_type: content_type\n" +
			"csp_sandbox: true\n" +
			"csp_sandbox_flags:\n" +
			"  - allow-downloads\n" +
			"csp_source_bundles:\n" +
			"  - cdnjs\n" +
			"expires_in_seconds: 0\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "generate-download-url",
			"--name", "name",
		)
	})
}

func TestSandboxesBoxesGenerateServiceURL(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "generate-service-url",
			"--name", "name",
			"--access", "restricted",
			"--expires-in-seconds", "0",
			"--port", "0",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"access: restricted\n" +
			"expires_in_seconds: 0\n" +
			"port: 0\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "generate-service-url",
			"--name", "name",
		)
	})
}

func TestSandboxesBoxesGetStatus(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "get-status",
			"--name", "name",
		)
	})
}

func TestSandboxesBoxesListServiceURLs(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "list-service-urls",
			"--max-items", "10",
			"--name", "name",
			"--cursor", "cursor",
			"--page-size", "0",
		)
	})
}

func TestSandboxesBoxesStart(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "start",
			"--name", "name",
		)
	})
}

func TestSandboxesBoxesStop(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:boxes", "stop",
			"--name", "name",
		)
	})
}
