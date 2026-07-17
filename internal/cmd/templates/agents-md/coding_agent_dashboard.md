## This app: coding-agent dashboard

Charts runs whose metadata `ls_agent_purpose == "coding"` for one project (or
scans every project), broken down by integration, model, tool, subagent, cost,
cache, errors, and time. It picks its own project via `GET /api/v1/sessions`
(`src/components/ProjectBar.tsx`) because the runs query is project-scoped.
`src/api.ts` wraps every call. Chart colors use `src/lib/palette.ts` tokens,
validated light + dark; status meaning uses semantic tokens, not the palette.

## The four-query architecture (`src/api.ts`)

`fetchProjectRuns` runs four scoped `POST /api/v1/runs/query` calls in parallel,
all with the coding filter (`ls_agent_purpose == "coding"`). `run_type` is
honored server-side, so child runs come from a flat query — no per-trace walk:

| Query | Scope | Powers |
|-------|-------|--------|
| roots | `is_root: true` | turns, tokens, cost, cache, errors, timing, threads, repos, versions, contributors |
| llm   | `run_type: "llm"` | model & provider breakdown, stop reasons |
| tool  | `run_type: "tool"` | tool-usage breakdown |
| chain | `run_type: "chain"` | subagents (filtered client-side to `ls_agent_type == "subagent"`) |

`fetchProjectsSummary` powers the "All projects" scan: one `is_root` query per
project (no coding filter), counting `ls_agent_purpose == "coding"` vs other.

### The 100-run cap

Every query here returns **at most the 100 most-recent matching runs**; the
response `cursor` is ignored. On busy projects the totals undercount. To report
true totals, follow the cursor or raise the limit — and surface that a cap is
in effect rather than presenting a partial count as complete.

## Where the data lives on a run

- Custom metadata: `run.extra.metadata`. SDK/runtime info: `run.extra.runtime`.
- **Token & cost totals roll up onto the root run** (`total_tokens`,
  `prompt_tokens`, `completion_tokens`, `total_cost`,
  `prompt_token_details.cache_read` / `.cache_creation`). Read economics from
  roots only — child totals are already included, so summing them double-counts.
- **Model & provider live on `llm` child runs**, not roots.
- A non-null `run.error` counts as a failed run.

## Verified metadata keys

Only these keys are relied on (confirmed present on real coding-agent traces):

- **Every run:** `ls_agent_purpose`, `ls_integration`, `ls_agent_type`
  (`"root"` | `"subagent"`), `ls_trace_schema_version`, `ls_agent_runtime`,
  `ls_agent_runtime_version`, `ls_integration_version`, `thread_id`,
  `turn_number`, `repository_name`, `repository_provider`, `repository_url`,
  `git_branch`, `git_commit_sha`, `cwd`.
- **`llm` runs:** `ls_model_name`, `ls_provider`, `stop_reason`, `usage_metadata`.
- **`tool` runs:** the tool name (key varies — see normalization).
- **`subagent` runs:** `ls_subagent_type`, `ls_subagent_id`.

## Field normalization (`src/lib/normalize.ts`)

Field names differ by integration, so entity accessors fall back through the
known aliases — always read through these, never a single raw key:

- **model:** `ls_model_name` → `model`
- **tool name:** `ls_tool_name` → `tool_name` → `toolName` → run `name`
- **user:** `user_name` → `user_email` → `local_username`

`user`-family keys and `ls_provider` are not emitted by every integration;
treat them as best-effort and fall back to `unknown`.

## What this app renders

- `src/api.ts` — `fetchProjects`, `fetchProjectRuns`, `fetchProjectsSummary`
- `src/components/ProjectBar.tsx` — project picker + "All projects" scan option
- `src/components/AllProjectsView.tsx` — cross-project coding-vs-other scan
- `src/lib/normalize.ts` / `aggregate.ts` / `format.ts` — metadata reads,
  grouping, formatting
- `src/lib/palette.ts` — categorical colors (light + dark validated)
- `src/components/primitives.tsx` — `StatTile`, `BarList`, `StackedBar`,
  `ColumnChart`, `Legend`, `Section`
- `src/components/PieChart.tsx` — hand-drawn SVG pie (no chart dependency)
- Panels: `OverviewPanel`, `CompositionPanel`, `EconomicsPanel`,
  `BehaviorPanel`, `ActivityPanel`, `ContextPanel`, plus `IntegrationBreakdown`

To add a panel: pick the query whose runs carry your field (roots for
economics, `llm` for models, `tool` for tools, `chain` for subagents), read it
through `normalize.ts`, aggregate with `aggregate.ts`, render with a primitive.
For per-turn detail, `GET /api/v1/runs/{id}` returns the full run.
