// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestSandboxesSnapshotsCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:snapshots", "create",
			"--docker-image", "docker_image",
			"--fs-capacity-bytes", "0",
			"--name", "name",
			"--description", "description",
			"--labels", "{foo: string}",
			"--registry-id", "registry_id",
			"--run-config", "{env_vars: {foo: string}, user: user, work_dir: work_dir}",
			"--tag", "tag",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(sandboxesSnapshotsCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:snapshots", "create",
			"--docker-image", "docker_image",
			"--fs-capacity-bytes", "0",
			"--name", "name",
			"--description", "description",
			"--labels", "{foo: string}",
			"--registry-id", "registry_id",
			"--run-config.env-vars", "{foo: string}",
			"--run-config.user", "user",
			"--run-config.work-dir", "work_dir",
			"--tag", "tag",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"docker_image: docker_image\n" +
			"fs_capacity_bytes: 0\n" +
			"name: name\n" +
			"description: description\n" +
			"labels:\n" +
			"  foo: string\n" +
			"registry_id: registry_id\n" +
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
			"sandboxes:snapshots", "create",
		)
	})
}

func TestSandboxesSnapshotsRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:snapshots", "retrieve",
			"--snapshot-id", "snapshot_id",
		)
	})
}

func TestSandboxesSnapshotsList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:snapshots", "list",
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
		)
	})
}

func TestSandboxesSnapshotsDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:snapshots", "delete",
			"--snapshot-id", "snapshot_id",
		)
	})
}

func TestSandboxesSnapshotsRetrieveByName(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:snapshots", "retrieve-by-name",
			"--name", "name",
		)
	})
}
