// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/mocktest"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
)

func TestChartsPreview(t *testing.T) {
	t.Skip("Mock server tests are disabled")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"charts", "preview",
			"--bucket-info", "{end_time: '2019-12-27T18:11:19.117Z', omit_data: true, start_time: '2019-12-27T18:11:19.117Z', stride: {days: 0, hours: 0, minutes: 0}, timezone: timezone}",
			"--chart", "{series: [{id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, name: name, feedback_key: feedback_key, filter_definition: {project_ids: [182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e], source_type: tracing_project, run_filter: run_filter, trace_filter: trace_filter, tree_filter: tree_filter}, filters: {filter: filter, session: [182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e], trace_filter: trace_filter, tree_filter: tree_filter}, group_by: {attribute: name, max_groups: 0, path: path, set_by: section}, group_by_definitions: [{attribute: name}], metadata: {foo: bar}, metric: run_count, metric_definition: {entity: feedback, params: {feedback_key: feedback_key}, filter: filter, type: count}, project_metric: memory_usage, workspace_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e}], common_filters: {filter: filter, session: [182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e], trace_filter: trace_filter, tree_filter: tree_filter}}",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(chartsPreview)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"--tenant-id", "string",
			"charts", "preview",
			"--bucket-info.end-time", "2019-12-27T18:11:19.117Z",
			"--bucket-info.omit-data=true",
			"--bucket-info.start-time", "2019-12-27T18:11:19.117Z",
			"--bucket-info.stride", "{days: 0, hours: 0, minutes: 0}",
			"--bucket-info.timezone", "timezone",
			"--chart.series", "[{id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e, name: name, feedback_key: feedback_key, filter_definition: {project_ids: [182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e], source_type: tracing_project, run_filter: run_filter, trace_filter: trace_filter, tree_filter: tree_filter}, filters: {filter: filter, session: [182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e], trace_filter: trace_filter, tree_filter: tree_filter}, group_by: {attribute: name, max_groups: 0, path: path, set_by: section}, group_by_definitions: [{attribute: name}], metadata: {foo: bar}, metric: run_count, metric_definition: {entity: feedback, params: {feedback_key: feedback_key}, filter: filter, type: count}, project_metric: memory_usage, workspace_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e}]",
			"--chart.common-filters", "{filter: filter, session: [182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e], trace_filter: trace_filter, tree_filter: tree_filter}",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"bucket_info:\n" +
			"  end_time: '2019-12-27T18:11:19.117Z'\n" +
			"  omit_data: true\n" +
			"  start_time: '2019-12-27T18:11:19.117Z'\n" +
			"  stride:\n" +
			"    days: 0\n" +
			"    hours: 0\n" +
			"    minutes: 0\n" +
			"  timezone: timezone\n" +
			"chart:\n" +
			"  series:\n" +
			"    - id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"      name: name\n" +
			"      feedback_key: feedback_key\n" +
			"      filter_definition:\n" +
			"        project_ids:\n" +
			"          - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"        source_type: tracing_project\n" +
			"        run_filter: run_filter\n" +
			"        trace_filter: trace_filter\n" +
			"        tree_filter: tree_filter\n" +
			"      filters:\n" +
			"        filter: filter\n" +
			"        session:\n" +
			"          - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"        trace_filter: trace_filter\n" +
			"        tree_filter: tree_filter\n" +
			"      group_by:\n" +
			"        attribute: name\n" +
			"        max_groups: 0\n" +
			"        path: path\n" +
			"        set_by: section\n" +
			"      group_by_definitions:\n" +
			"        - attribute: name\n" +
			"      metadata:\n" +
			"        foo: bar\n" +
			"      metric: run_count\n" +
			"      metric_definition:\n" +
			"        entity: feedback\n" +
			"        params:\n" +
			"          feedback_key: feedback_key\n" +
			"        filter: filter\n" +
			"        type: count\n" +
			"      project_metric: memory_usage\n" +
			"      workspace_id: 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"  common_filters:\n" +
			"    filter: filter\n" +
			"    session:\n" +
			"      - 182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e\n" +
			"    trace_filter: trace_filter\n" +
			"    tree_filter: tree_filter\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"--tenant-id", "string",
			"charts", "preview",
		)
	})
}
