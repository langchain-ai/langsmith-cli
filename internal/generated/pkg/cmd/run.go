// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/apiquery"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
	"github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli/v3"
)

var runsCreate = cli.Command{
	Name:    "create",
	Usage:   "Queues a single run for ingestion. The request body must be a JSON-encoded run\nobject that follows the Run schema.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "id",
			BodyPath: "id",
		},
		&requestflag.Flag[string]{
			Name:     "agent-environment",
			Usage:    "Experimental. The Agent environment the run belongs to, case-insensitive;\nrequires agent_id. Only workspaces enabled for Agent addressing accept it;\nothers get a 403.",
			BodyPath: "agent_environment",
		},
		&requestflag.Flag[string]{
			Name:     "agent-id",
			Usage:    "Experimental. Addresses the run to an Agent, with agent_environment, in\nplace of session_id or session_name. Only workspaces enabled for Agent\naddressing accept it; others get a 403.",
			BodyPath: "agent_id",
		},
		&requestflag.Flag[string]{
			Name:     "dotted-order",
			BodyPath: "dotted_order",
		},
		&requestflag.Flag[string]{
			Name:     "end-time",
			BodyPath: "end_time",
		},
		&requestflag.Flag[string]{
			Name:     "error",
			BodyPath: "error",
		},
		&requestflag.Flag[[]map[string]any]{
			Name:     "event",
			BodyPath: "events",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "extra",
			BodyPath: "extra",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "input-attachments",
			BodyPath: "input_attachments",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "inputs",
			BodyPath: "inputs",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "output-attachments",
			BodyPath: "output_attachments",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "outputs",
			BodyPath: "outputs",
		},
		&requestflag.Flag[string]{
			Name:     "parent-run-id",
			BodyPath: "parent_run_id",
		},
		&requestflag.Flag[string]{
			Name:     "reference-example-id",
			BodyPath: "reference_example_id",
		},
		&requestflag.Flag[string]{
			Name:     "run-type",
			Usage:    `Allowed values: "tool", "chain", "llm", "retriever", "embedding", "prompt", "parser".`,
			BodyPath: "run_type",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "serialized",
			BodyPath: "serialized",
		},
		&requestflag.Flag[string]{
			Name:     "session-id",
			BodyPath: "session_id",
		},
		&requestflag.Flag[string]{
			Name:     "session-name",
			BodyPath: "session_name",
		},
		&requestflag.Flag[string]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
		&requestflag.Flag[string]{
			Name:     "status",
			BodyPath: "status",
		},
		&requestflag.Flag[[]string]{
			Name:     "tag",
			BodyPath: "tags",
		},
		&requestflag.Flag[string]{
			Name:     "trace-id",
			BodyPath: "trace_id",
		},
	},
	Action:          handleRunsCreate,
	HideHelpCommand: true,
}

var runsUpdate = cli.Command{
	Name:    "update",
	Usage:   "Updates a run identified by its ID. The body should contain only the fields to\nbe changed; unknown fields are ignored.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "run-id",
			Required:  true,
			PathParam: "run_id",
		},
		&requestflag.Flag[string]{
			Name:     "id",
			BodyPath: "id",
		},
		&requestflag.Flag[string]{
			Name:     "agent-environment",
			Usage:    "Experimental. The Agent environment the run belongs to, case-insensitive;\nrequires agent_id. Only workspaces enabled for Agent addressing accept it;\nothers get a 403.",
			BodyPath: "agent_environment",
		},
		&requestflag.Flag[string]{
			Name:     "agent-id",
			Usage:    "Experimental. Addresses the run to an Agent, with agent_environment, in\nplace of session_id or session_name. Only workspaces enabled for Agent\naddressing accept it; others get a 403.",
			BodyPath: "agent_id",
		},
		&requestflag.Flag[string]{
			Name:     "dotted-order",
			BodyPath: "dotted_order",
		},
		&requestflag.Flag[string]{
			Name:     "end-time",
			BodyPath: "end_time",
		},
		&requestflag.Flag[string]{
			Name:     "error",
			BodyPath: "error",
		},
		&requestflag.Flag[[]map[string]any]{
			Name:     "event",
			BodyPath: "events",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "extra",
			BodyPath: "extra",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "input-attachments",
			BodyPath: "input_attachments",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "inputs",
			BodyPath: "inputs",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "output-attachments",
			BodyPath: "output_attachments",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "outputs",
			BodyPath: "outputs",
		},
		&requestflag.Flag[string]{
			Name:     "parent-run-id",
			BodyPath: "parent_run_id",
		},
		&requestflag.Flag[string]{
			Name:     "reference-example-id",
			BodyPath: "reference_example_id",
		},
		&requestflag.Flag[string]{
			Name:     "run-type",
			Usage:    `Allowed values: "tool", "chain", "llm", "retriever", "embedding", "prompt", "parser".`,
			BodyPath: "run_type",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "serialized",
			BodyPath: "serialized",
		},
		&requestflag.Flag[string]{
			Name:     "session-id",
			BodyPath: "session_id",
		},
		&requestflag.Flag[string]{
			Name:     "session-name",
			BodyPath: "session_name",
		},
		&requestflag.Flag[string]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
		&requestflag.Flag[string]{
			Name:     "status",
			BodyPath: "status",
		},
		&requestflag.Flag[[]string]{
			Name:     "tag",
			BodyPath: "tags",
		},
		&requestflag.Flag[string]{
			Name:     "trace-id",
			BodyPath: "trace_id",
		},
	},
	Action:          handleRunsUpdate,
	HideHelpCommand: true,
}

var runsGetURL = cli.Command{
	Name:    "get-url",
	Usage:   "Returns the URL to view a specific run in the LangSmith UI. The caller must\nsupply the run's project_id and trace_id as query parameters; start_time is\noptional.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "run-id",
			Required:  true,
			PathParam: "run_id",
		},
		&requestflag.Flag[string]{
			Name:      "project-id",
			Usage:     "Project (session) UUID",
			Required:  true,
			QueryPath: "project_id",
		},
		&requestflag.Flag[string]{
			Name:      "trace-id",
			Usage:     "Trace UUID",
			Required:  true,
			QueryPath: "trace_id",
		},
		&requestflag.Flag[string]{
			Name:      "start-time",
			Usage:     "Run start time in RFC3339 format; omit if unknown",
			QueryPath: "start_time",
		},
	},
	Action:          handleRunsGetURL,
	HideHelpCommand: true,
}

var runsIngestBatch = requestflag.WithInnerFlags(cli.Command{
	Name:    "ingest-batch",
	Usage:   "Ingests a batch of runs in a single JSON payload. The payload must have `post`\nand/or `patch` arrays containing run objects. Prefer this endpoint over\nsingle‑run ingestion when submitting hundreds of runs, but `/runs/multipart`\noffers better handling for very large fields and attachments.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[[]map[string]any]{
			Name:     "patch",
			BodyPath: "patch",
		},
		&requestflag.Flag[[]map[string]any]{
			Name:     "post",
			BodyPath: "post",
		},
	},
	Action:          handleRunsIngestBatch,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"patch": {
		&requestflag.InnerFlag[string]{
			Name:       "patch.id",
			InnerField: "id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.agent-environment",
			Usage:      "Experimental. The Agent environment the run belongs to, case-insensitive;\nrequires agent_id. Only workspaces enabled for Agent addressing accept it;\nothers get a 403.",
			InnerField: "agent_environment",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.agent-id",
			Usage:      "Experimental. Addresses the run to an Agent, with agent_environment, in\nplace of session_id or session_name. Only workspaces enabled for Agent\naddressing accept it; others get a 403.",
			InnerField: "agent_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.dotted-order",
			InnerField: "dotted_order",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.end-time",
			InnerField: "end_time",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.error",
			InnerField: "error",
		},
		&requestflag.InnerFlag[[]map[string]any]{
			Name:       "patch.events",
			InnerField: "events",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.extra",
			InnerField: "extra",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.input-attachments",
			InnerField: "input_attachments",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.inputs",
			InnerField: "inputs",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.name",
			InnerField: "name",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.output-attachments",
			InnerField: "output_attachments",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.outputs",
			InnerField: "outputs",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.parent-run-id",
			InnerField: "parent_run_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.reference-example-id",
			InnerField: "reference_example_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.run-type",
			Usage:      `Allowed values: "tool", "chain", "llm", "retriever", "embedding", "prompt", "parser".`,
			InnerField: "run_type",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "patch.serialized",
			InnerField: "serialized",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.session-id",
			InnerField: "session_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.session-name",
			InnerField: "session_name",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.start-time",
			InnerField: "start_time",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.status",
			InnerField: "status",
		},
		&requestflag.InnerFlag[[]string]{
			Name:       "patch.tags",
			InnerField: "tags",
		},
		&requestflag.InnerFlag[string]{
			Name:       "patch.trace-id",
			InnerField: "trace_id",
		},
	},
	"post": {
		&requestflag.InnerFlag[string]{
			Name:       "post.id",
			InnerField: "id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.agent-environment",
			Usage:      "Experimental. The Agent environment the run belongs to, case-insensitive;\nrequires agent_id. Only workspaces enabled for Agent addressing accept it;\nothers get a 403.",
			InnerField: "agent_environment",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.agent-id",
			Usage:      "Experimental. Addresses the run to an Agent, with agent_environment, in\nplace of session_id or session_name. Only workspaces enabled for Agent\naddressing accept it; others get a 403.",
			InnerField: "agent_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.dotted-order",
			InnerField: "dotted_order",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.end-time",
			InnerField: "end_time",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.error",
			InnerField: "error",
		},
		&requestflag.InnerFlag[[]map[string]any]{
			Name:       "post.events",
			InnerField: "events",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.extra",
			InnerField: "extra",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.input-attachments",
			InnerField: "input_attachments",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.inputs",
			InnerField: "inputs",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.name",
			InnerField: "name",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.output-attachments",
			InnerField: "output_attachments",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.outputs",
			InnerField: "outputs",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.parent-run-id",
			InnerField: "parent_run_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.reference-example-id",
			InnerField: "reference_example_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.run-type",
			Usage:      `Allowed values: "tool", "chain", "llm", "retriever", "embedding", "prompt", "parser".`,
			InnerField: "run_type",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "post.serialized",
			InnerField: "serialized",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.session-id",
			InnerField: "session_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.session-name",
			InnerField: "session_name",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.start-time",
			InnerField: "start_time",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.status",
			InnerField: "status",
		},
		&requestflag.InnerFlag[[]string]{
			Name:       "post.tags",
			InnerField: "tags",
		},
		&requestflag.InnerFlag[string]{
			Name:       "post.trace-id",
			InnerField: "trace_id",
		},
	},
})

var runsQueryV1 = cli.Command{
	Name:    "query-v1",
	Usage:   "Query Runs",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[any]{
			Name:     "id",
			BodyPath: "id",
		},
		&requestflag.Flag[*string]{
			Name:     "cursor",
			BodyPath: "cursor",
		},
		&requestflag.Flag[*string]{
			Name:     "data-source-type",
			Usage:    "Enum for run data source types.",
			BodyPath: "data_source_type",
		},
		&requestflag.Flag[any]{
			Name:     "end-time",
			BodyPath: "end_time",
		},
		&requestflag.Flag[*bool]{
			Name:     "error",
			BodyPath: "error",
		},
		&requestflag.Flag[*int64]{
			Name:     "execution-order",
			BodyPath: "execution_order",
		},
		&requestflag.Flag[*string]{
			Name:     "filter",
			BodyPath: "filter",
		},
		&requestflag.Flag[*bool]{
			Name:     "is-root",
			BodyPath: "is_root",
		},
		&requestflag.Flag[int64]{
			Name:     "limit",
			Usage:    "Maximum number of runs to return. Not applied when trace is set — all runs in the trace are returned in a single response.",
			Default:  100,
			BodyPath: "limit",
		},
		&requestflag.Flag[string]{
			Name:     "order",
			Usage:    "Enum for run start date order.",
			Default:  "desc",
			BodyPath: "order",
		},
		&requestflag.Flag[*string]{
			Name:     "parent-run",
			BodyPath: "parent_run",
		},
		&requestflag.Flag[*string]{
			Name:     "query",
			BodyPath: "query",
		},
		&requestflag.Flag[any]{
			Name:     "reference-example",
			BodyPath: "reference_example",
		},
		&requestflag.Flag[*string]{
			Name:     "run-type",
			Usage:    "Enum for run types.",
			BodyPath: "run_type",
		},
		&requestflag.Flag[*string]{
			Name:     "search-filter",
			BodyPath: "search_filter",
		},
		&requestflag.Flag[[]string]{
			Name:     "select",
			Default:  []string{"id", "name", "run_type", "start_time", "end_time", "status", "error", "extra", "events", "inputs", "outputs", "parent_run_id", "manifest_id", "manifest_s3_id", "manifest", "session_id", "serialized", "reference_example_id", "reference_dataset_id", "total_tokens", "prompt_tokens", "prompt_token_details", "completion_tokens", "completion_token_details", "total_cost", "prompt_cost", "prompt_cost_details", "completion_cost", "completion_cost_details", "price_model_id", "first_token_time", "trace_id", "dotted_order", "last_queued_at", "feedback_stats", "parent_run_ids", "tags", "in_dataset", "app_path", "share_token", "trace_tier", "trace_first_received_at", "ttl_seconds", "trace_upgrade", "thread_id"},
			BodyPath: "select",
		},
		&requestflag.Flag[any]{
			Name:     "session",
			BodyPath: "session",
		},
		&requestflag.Flag[*bool]{
			Name:     "skip-pagination",
			BodyPath: "skip_pagination",
		},
		&requestflag.Flag[bool]{
			Name:     "skip-prev-cursor",
			Default:  false,
			BodyPath: "skip_prev_cursor",
		},
		&requestflag.Flag[any]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
		&requestflag.Flag[*string]{
			Name:     "trace",
			Usage:    "Filter runs by trace ID. When set, limit and cursor-based pagination are not applied — all runs in the trace are returned in a single response.",
			BodyPath: "trace",
		},
		&requestflag.Flag[*string]{
			Name:     "trace-filter",
			BodyPath: "trace_filter",
		},
		&requestflag.Flag[*string]{
			Name:     "tree-filter",
			BodyPath: "tree_filter",
		},
		&requestflag.Flag[bool]{
			Name:     "use-experimental-search",
			Default:  false,
			BodyPath: "use_experimental_search",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleRunsQueryV1,
	HideHelpCommand: true,
}

var runsQueryV2 = cli.Command{
	Name:    "query-v2",
	Usage:   "Returns a paginated list of runs for the given projects within min/max\nstart_time. Supports filters, cursor pagination, and `selects` to select fields\nto return.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "cursor",
			Usage:    "`cursor` is the opaque string from a previous response's `next_cursor`. Treat it as opaque and pass it back unmodified.",
			BodyPath: "cursor",
		},
		&requestflag.Flag[string]{
			Name:     "filter",
			Usage:    "`filter` narrows results to runs matching this LangSmith filter expression, evaluated against each individual run.\nFor example: and(eq(run_type, \"llm\"), gt(latency, 5)) or eq(status, \"error\").\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "filter",
		},
		&requestflag.Flag[bool]{
			Name:     "has-error",
			Usage:    "`has_error` filters to runs that errored (true) or completed without error (false).",
			BodyPath: "has_error",
		},
		&requestflag.Flag[[]string]{
			Name:     "id",
			Usage:    "`ids` optionally limits the request to these run UUIDs.",
			BodyPath: "ids",
		},
		&requestflag.Flag[bool]{
			Name:     "is-root",
			Usage:    "`is_root` returns only root runs (true) or only non-root runs (false).",
			BodyPath: "is_root",
		},
		&requestflag.Flag[any]{
			Name:     "max-start-time",
			Usage:    "`max_start_time` is the upper bound for run `start_time` (RFC3339). Defaults to now.",
			BodyPath: "max_start_time",
		},
		&requestflag.Flag[any]{
			Name:     "min-start-time",
			Usage:    "`min_start_time` is the lower bound for run `start_time` (RFC3339). Defaults to 1 day ago.",
			BodyPath: "min_start_time",
		},
		&requestflag.Flag[int64]{
			Name:     "page-size",
			Usage:    "`page_size` is the maximum number of runs to return in this response. Defaults to 100 when omitted; must be between 1 and 1000 inclusive when set.",
			Default:  100,
			BodyPath: "page_size",
		},
		&requestflag.Flag[[]string]{
			Name:     "project-id",
			Usage:    "`project_ids` lists tracing project UUIDs to query.\nRequired unless `reference_dataset_id` is set. Mutually exclusive with `reference_dataset_id` — set exactly one of them.",
			BodyPath: "project_ids",
		},
		&requestflag.Flag[string]{
			Name:     "reference-dataset-id",
			Usage:    "`reference_dataset_id` resolves session IDs server-side from the dataset.\nRequired unless `project_ids` is set. Mutually exclusive with `project_ids` — set exactly one of them.\nWhen provided and `min_start_time` is omitted, the server derives it from the earliest session creation date.",
			BodyPath: "reference_dataset_id",
		},
		&requestflag.Flag[[]string]{
			Name:     "reference-example",
			Usage:    "`reference_examples` optionally limits to runs linked to these dataset example UUIDs.",
			BodyPath: "reference_examples",
		},
		&requestflag.Flag[string]{
			Name:     "run-type",
			Usage:    `Allowed values: "TOOL", "CHAIN", "LLM", "RETRIEVER", "EMBEDDING", "PROMPT", "PARSER".`,
			BodyPath: "run_type",
		},
		&requestflag.Flag[[]string]{
			Name:     "select",
			Usage:    "`selects` lists which properties to include on each returned run. If omitted, only `id` is returned. Properties not listed are omitted from each run object.",
			BodyPath: "selects",
		},
		&requestflag.Flag[string]{
			Name:     "trace-filter",
			Usage:    "`trace_filter` narrows results to runs whose root trace matches this LangSmith filter expression.\nUse this to filter by properties of the trace's root run — for example eq(status, \"success\") to include only traces that completed without error.\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "trace_filter",
		},
		&requestflag.Flag[string]{
			Name:     "trace-id",
			Usage:    "`trace_id` optionally limits results to runs belonging to this trace UUID.",
			BodyPath: "trace_id",
		},
		&requestflag.Flag[string]{
			Name:     "tree-filter",
			Usage:    "`tree_filter` narrows results to runs that belong to a trace containing at least one run matching this LangSmith filter expression anywhere in the run tree (not just the root).\nUse this to find runs inside traces that involved a specific tool, tag, or model — for example has(tags, \"production\") or eq(name, \"my_tool\").\nSee https://docs.langchain.com/langsmith/trace-query-syntax#filter-query-language for syntax.",
			BodyPath: "tree_filter",
		},
		&requestflag.Flag[string]{
			Name:       "accept",
			HeaderPath: "Accept",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleRunsQueryV2,
	HideHelpCommand: true,
}

var runsRetrieveV1 = cli.Command{
	Name:    "retrieve-v1",
	Usage:   "Get a specific run.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "run-id",
			Required:  true,
			PathParam: "run_id",
		},
		&requestflag.Flag[bool]{
			Name:      "exclude-s3-stored-attributes",
			Default:   false,
			QueryPath: "exclude_s3_stored_attributes",
		},
		&requestflag.Flag[bool]{
			Name:      "exclude-serialized",
			Default:   false,
			QueryPath: "exclude_serialized",
		},
		&requestflag.Flag[bool]{
			Name:      "include-messages",
			Default:   false,
			QueryPath: "include_messages",
		},
		&requestflag.Flag[*string]{
			Name:      "session-id",
			QueryPath: "session_id",
		},
		&requestflag.Flag[any]{
			Name:      "start-time",
			QueryPath: "start_time",
		},
	},
	Action:          handleRunsRetrieveV1,
	HideHelpCommand: true,
}

var runsRetrieveV2 = cli.Command{
	Name:    "retrieve-v2",
	Usage:   "Returns one run by ID for the given session. Use the `selects` query parameter\n(repeatable) to select fields to return.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "run-id",
			Required:  true,
			PathParam: "run_id",
		},
		&requestflag.Flag[string]{
			Name:      "project-id",
			Usage:     "`project_id` is the UUID of the tracing project that owns the run.",
			Required:  true,
			QueryPath: "project_id",
		},
		&requestflag.Flag[[]string]{
			Name:      "select",
			Usage:     "`selects` lists which properties to include on the returned run (repeatable query parameter). Accepts any value of the `RunSelectField` enum. If omitted, only `id` is returned.",
			QueryPath: "selects",
		},
		&requestflag.Flag[any]{
			Name:      "start-time",
			Usage:     "`start_time` is the run's `start_time` (RFC3339 date-time). Providing it speeds up retrieval.",
			QueryPath: "start_time",
		},
		&requestflag.Flag[string]{
			Name:       "accept",
			HeaderPath: "Accept",
		},
	},
	Action:          handleRunsRetrieveV2,
	HideHelpCommand: true,
}

var runsStats = requestflag.WithInnerFlags(cli.Command{
	Name:    "stats",
	Usage:   "Get all runs by query in body payload.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[[]string]{
			Name:     "session",
			Required: true,
			BodyPath: "session",
		},
		&requestflag.Flag[any]{
			Name:     "id",
			BodyPath: "id",
		},
		&requestflag.Flag[*string]{
			Name:     "data-source-type",
			Usage:    "Enum for run data source types.",
			BodyPath: "data_source_type",
		},
		&requestflag.Flag[any]{
			Name:     "end-time",
			BodyPath: "end_time",
		},
		&requestflag.Flag[*bool]{
			Name:     "error",
			BodyPath: "error",
		},
		&requestflag.Flag[*int64]{
			Name:     "execution-order",
			BodyPath: "execution_order",
		},
		&requestflag.Flag[*string]{
			Name:     "filter",
			BodyPath: "filter",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "group-by",
			Usage:    "Group by param for run stats.",
			BodyPath: "group_by",
		},
		&requestflag.Flag[any]{
			Name:     "group",
			BodyPath: "groups",
		},
		&requestflag.Flag[bool]{
			Name:     "include-details",
			Default:  false,
			BodyPath: "include_details",
		},
		&requestflag.Flag[*bool]{
			Name:     "is-root",
			BodyPath: "is_root",
		},
		&requestflag.Flag[*string]{
			Name:     "parent-run",
			BodyPath: "parent_run",
		},
		&requestflag.Flag[*string]{
			Name:     "query",
			BodyPath: "query",
		},
		&requestflag.Flag[*string]{
			Name:     "reference-dataset-id",
			BodyPath: "reference_dataset_id",
		},
		&requestflag.Flag[any]{
			Name:     "reference-example",
			BodyPath: "reference_example",
		},
		&requestflag.Flag[*string]{
			Name:     "run-type",
			Usage:    "Enum for run types.",
			BodyPath: "run_type",
		},
		&requestflag.Flag[*string]{
			Name:     "search-filter",
			BodyPath: "search_filter",
		},
		&requestflag.Flag[any]{
			Name:     "select",
			BodyPath: "select",
		},
		&requestflag.Flag[*bool]{
			Name:     "skip-pagination",
			BodyPath: "skip_pagination",
		},
		&requestflag.Flag[any]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
		&requestflag.Flag[*string]{
			Name:     "trace",
			Usage:    "Filter runs by trace ID. When set, limit and cursor-based pagination are not applied — all runs in the trace are returned in a single response.",
			BodyPath: "trace",
		},
		&requestflag.Flag[*string]{
			Name:     "trace-filter",
			BodyPath: "trace_filter",
		},
		&requestflag.Flag[*string]{
			Name:     "tree-filter",
			BodyPath: "tree_filter",
		},
		&requestflag.Flag[bool]{
			Name:     "use-experimental-search",
			Default:  false,
			BodyPath: "use_experimental_search",
		},
	},
	Action:          handleRunsStats,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"group-by": {
		&requestflag.InnerFlag[string]{
			Name:       "group-by.attribute",
			Usage:      `Allowed values: "name", "run_type", "tag", "metadata".`,
			InnerField: "attribute",
		},
		&requestflag.InnerFlag[int64]{
			Name:       "group-by.max-groups",
			InnerField: "max_groups",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "group-by.path",
			InnerField: "path",
		},
	},
})

var runsUpdate2 = cli.Command{
	Name:    "update-2",
	Usage:   "Updates a run identified by its ID. The body should contain only the fields to\nbe changed; unknown fields are ignored.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "run-id",
			Required:  true,
			PathParam: "run_id",
		},
		&requestflag.Flag[string]{
			Name:     "id",
			BodyPath: "id",
		},
		&requestflag.Flag[string]{
			Name:     "agent-environment",
			Usage:    "Experimental. The Agent environment the run belongs to, case-insensitive;\nrequires agent_id. Only workspaces enabled for Agent addressing accept it;\nothers get a 403.",
			BodyPath: "agent_environment",
		},
		&requestflag.Flag[string]{
			Name:     "agent-id",
			Usage:    "Experimental. Addresses the run to an Agent, with agent_environment, in\nplace of session_id or session_name. Only workspaces enabled for Agent\naddressing accept it; others get a 403.",
			BodyPath: "agent_id",
		},
		&requestflag.Flag[string]{
			Name:     "dotted-order",
			BodyPath: "dotted_order",
		},
		&requestflag.Flag[string]{
			Name:     "end-time",
			BodyPath: "end_time",
		},
		&requestflag.Flag[string]{
			Name:     "error",
			BodyPath: "error",
		},
		&requestflag.Flag[[]map[string]any]{
			Name:     "event",
			BodyPath: "events",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "extra",
			BodyPath: "extra",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "input-attachments",
			BodyPath: "input_attachments",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "inputs",
			BodyPath: "inputs",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "output-attachments",
			BodyPath: "output_attachments",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "outputs",
			BodyPath: "outputs",
		},
		&requestflag.Flag[string]{
			Name:     "parent-run-id",
			BodyPath: "parent_run_id",
		},
		&requestflag.Flag[string]{
			Name:     "reference-example-id",
			BodyPath: "reference_example_id",
		},
		&requestflag.Flag[string]{
			Name:     "run-type",
			Usage:    `Allowed values: "tool", "chain", "llm", "retriever", "embedding", "prompt", "parser".`,
			BodyPath: "run_type",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "serialized",
			BodyPath: "serialized",
		},
		&requestflag.Flag[string]{
			Name:     "session-id",
			BodyPath: "session_id",
		},
		&requestflag.Flag[string]{
			Name:     "session-name",
			BodyPath: "session_name",
		},
		&requestflag.Flag[string]{
			Name:     "start-time",
			BodyPath: "start_time",
		},
		&requestflag.Flag[string]{
			Name:     "status",
			BodyPath: "status",
		},
		&requestflag.Flag[[]string]{
			Name:     "tag",
			BodyPath: "tags",
		},
		&requestflag.Flag[string]{
			Name:     "trace-id",
			BodyPath: "trace_id",
		},
	},
	Action:          handleRunsUpdate2,
	HideHelpCommand: true,
}

func handleRunsCreate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.New(ctx, params, options...)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "runs create",
		Transform:      transform,
	})
}

func handleRunsUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("run-id") && len(unusedArgs) > 0 {
		cmd.Set("run-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.Update(
		ctx,
		cmd.Value("run-id").(string),
		params,
		options...,
	)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "runs update",
		Transform:      transform,
	})
}

func handleRunsGetURL(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("run-id") && len(unusedArgs) > 0 {
		cmd.Set("run-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunGetURLParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.GetURL(
		ctx,
		cmd.Value("run-id").(string),
		params,
		options...,
	)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "runs get-url",
		Transform:      transform,
	})
}

func handleRunsIngestBatch(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunIngestBatchParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.IngestBatch(ctx, params, options...)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "runs ingest-batch",
		Transform:      transform,
	})
}

func handleRunsQueryV1(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunQueryV1Params{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Runs.QueryV1(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "runs query-v1",
			Transform:      transform,
		})
	} else {
		iter := client.Runs.QueryV1AutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "runs query-v1",
			Transform:      transform,
		})
	}
}

func handleRunsQueryV2(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunQueryV2Params{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Runs.QueryV2(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "runs query-v2",
			Transform:      transform,
		})
	} else {
		iter := client.Runs.QueryV2AutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "runs query-v2",
			Transform:      transform,
		})
	}
}

func handleRunsRetrieveV1(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("run-id") && len(unusedArgs) > 0 {
		cmd.Set("run-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunGetV1Params{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.GetV1(
		ctx,
		cmd.Value("run-id").(string),
		params,
		options...,
	)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "runs retrieve-v1",
		Transform:      transform,
	})
}

func handleRunsRetrieveV2(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("run-id") && len(unusedArgs) > 0 {
		cmd.Set("run-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunGetV2Params{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.GetV2(
		ctx,
		cmd.Value("run-id").(string),
		params,
		options...,
	)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "runs retrieve-v2",
		Transform:      transform,
	})
}

func handleRunsStats(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunStatsParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.Stats(ctx, params, options...)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "runs stats",
		Transform:      transform,
	})
}

func handleRunsUpdate2(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("run-id") && len(unusedArgs) > 0 {
		cmd.Set("run-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.RunUpdate2Params{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Runs.Update2(
		ctx,
		cmd.Value("run-id").(string),
		params,
		options...,
	)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "runs update-2",
		Transform:      transform,
	})
}
