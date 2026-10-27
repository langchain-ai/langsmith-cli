// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestSessionsUpdate(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions", "update",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--default-dataset-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--description", "description",
			"--end-time", "'2019-12-27T18:11:19.117Z'",
			"--extra", "{foo: bar}",
			"--name", "name",
			"--trace-tier", "longlived",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"default_dataset_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"description: description\n" +
			"end_time: '2019-12-27T18:11:19.117Z'\n" +
			"extra:\n" +
			"  foo: bar\n" +
			"name: name\n" +
			"trace_tier: longlived\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions", "update",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestSessionsDashboard(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions", "dashboard",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--end-time", "'2019-12-27T18:11:19.117Z'",
			"--group-by", "{attribute: name, max_groups: 0, path: path}",
			"--omit-data=true",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--stride", "{days: 0, hours: 0, minutes: 0}",
			"--timezone", "timezone",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(sessionsDashboard)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions", "dashboard",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
			"--end-time", "'2019-12-27T18:11:19.117Z'",
			"--group-by.attribute", "name",
			"--group-by.max-groups", "0",
			"--group-by.path", "path",
			"--omit-data=true",
			"--start-time", "'2019-12-27T18:11:19.117Z'",
			"--stride.days", "0",
			"--stride.hours", "0",
			"--stride.minutes", "0",
			"--timezone", "timezone",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"end_time: '2019-12-27T18:11:19.117Z'\n" +
			"group_by:\n" +
			"  attribute: name\n" +
			"  max_groups: 0\n" +
			"  path: path\n" +
			"omit_data: true\n" +
			"start_time: '2019-12-27T18:11:19.117Z'\n" +
			"stride:\n" +
			"  days: 0\n" +
			"  hours: 0\n" +
			"  minutes: 0\n" +
			"timezone: timezone\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions", "dashboard",
			"--session-id", "182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e",
		)
	})
}

func TestSessionsResolve(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"sessions", "resolve",
			"--kind", "AGENT",
			"--id", "id",
			"--environment", "LOCAL",
		)
	})
}
