// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
)

func TestSandboxesRegistriesCreate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:registries", "create",
			"--name", "name",
			"--url", "url",
			"--auth-type", "DOCKER_CONFIG",
			"--aws-role-arn", "aws_role_arn",
			"--password", "password",
			"--username", "username",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"name: name\n" +
			"url: url\n" +
			"auth_type: DOCKER_CONFIG\n" +
			"aws_role_arn: aws_role_arn\n" +
			"password: password\n" +
			"username: username\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:registries", "create",
		)
	})
}

func TestSandboxesRegistriesRetrieve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:registries", "retrieve",
			"--name", "name",
		)
	})
}

func TestSandboxesRegistriesList(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:registries", "list",
			"--cursor", "cursor",
			"--limit", "0",
			"--name-contains", "name_contains",
			"--offset", "0",
			"--page-size", "1",
		)
	})
}

func TestSandboxesRegistriesDelete(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sandboxes:registries", "delete",
			"--name", "name",
		)
	})
}
