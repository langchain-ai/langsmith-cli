# AGENTS.md — LangSmith API surface for this app

This is a **coding-agent dashboard**: it charts runs whose metadata
`ls_agent_purpose == "coding"` for one project (or scans every project), broken
down by integration, model, tool, subagent, cost, cache, errors, and time.
Apps are uniform — the host gives you no bound context, so
`render(data, root, metadata)` receives `data = {}`. The app picks its own
project (`src/components/ProjectBar.tsx`, via `GET /api/v1/sessions`) because
the runs query is project-scoped. Everything is fetched via
`window.langsmith.call`; `src/api.ts` wraps every call.

## Theme (`metadata`)

`render`'s third argument is `metadata`; v1 has one key, `metadata.mode`
(`"dark"` | `"light"`). The sandbox sets `html.dark` from it before every
render, so this Tailwind/token UI themes for free — no branching. Chart colors
use `index.css` tokens that flip under `html.dark` (`src/lib/palette.ts`),
validated for both themes; status meaning uses semantic tokens, not the
categorical palette.

## Calling the API

```ts
const projects = await window.langsmith.call('GET /api/v1/sessions', {
  params: { limit: '100' },
});
```

Generic passthrough, not a curated allowlist — any operation your API key
permits works. Full reference: https://docs.langchain.com/langsmith/home.

## Endpoints this app uses

- `GET /api/v1/sessions` — list tracing projects (the project picker)
- `POST /api/v1/runs/query` — query runs; returns `{ runs: [...] }`. Body fields
  used here:
  - `session: ["<project-uuid>"]` — required, scopes the query
  - `filter` — the filter DSL string (see below)
  - `is_root: true` — top-level turns only
  - `run_type: "llm" | "tool" | "chain"` — **honored server-side**, so child
    runs are fetched with a flat query, not by walking each trace
  - `start_time: "<ISO>"` — recent window (this app uses last 7 days)
  - `limit: 100`, `select: [...]`
- `GET /api/v1/runs/{id}` — one full run (all fields + `inputs`/`outputs`).
  Not used by the dashboard, but the way to add a per-turn drill-down.

### The 100-run cap

Every `runs/query` here returns **at most the 100 most-recent matching runs**;
the response `cursor` is ignored. On busy projects the totals undercount. To
report true totals, follow the cursor or raise the limit — and surface that a
cap is in effect rather than presenting a partial count as complete.

## The four-query architecture (`src/api.ts`)

`fetchProjectRuns` runs four scoped `runs/query` calls in parallel, all with the
coding filter:

| Query | Scope | Powers |
|-------|-------|--------|
| roots | `is_root: true` | turns, tokens, cost, cache, errors, timing, threads, repos, versions, contributors |
| llm   | `run_type: "llm"` | model & provider breakdown, stop reasons |
| tool  | `run_type: "tool"` | tool-usage breakdown |
| chain | `run_type: "chain"` | subagents (filtered client-side to `ls_agent_type == "subagent"`) |

`fetchProjectsSummary` powers the "All projects" scan: one `is_root` query per
project (no coding filter), counting `ls_agent_purpose == "coding"` vs other in
the sample.

## Filter DSL for metadata equality

Metadata equality is **two paired clauses**, not `eq(metadata.key, ...)`:

```
and(eq(metadata_key, "ls_agent_purpose"), eq(metadata_value, "coding"))
```

Combine with `and(...)` / `or(...)`; other examples: `has(tags, "prod")`,
`eq(name, "ChatOpenAI")`, `eq(is_root, true)`.

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
- **`llm` runs:** `ls_model_name`, `ls_provider`, `stop_reason`,
  `usage_metadata`.
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
  grouping, and display formatting
- `src/lib/palette.ts` — categorical colors (light + dark validated)
- `src/components/primitives.tsx` — `StatTile`, `BarList`, `StackedBar`,
  `ColumnChart`, `Legend`, `Section`
- `src/components/PieChart.tsx` — hand-drawn SVG pie (no chart dependency)
- Panels: `OverviewPanel`, `CompositionPanel`, `EconomicsPanel`,
  `BehaviorPanel`, `ActivityPanel`, `ContextPanel`, plus
  `IntegrationBreakdown` (models per integration)

To add a panel: pick the query whose runs carry your field (roots for
economics, `llm` for models, `tool` for tools, `chain` for subagents), read it
through `normalize.ts`, aggregate with `aggregate.ts`, and render with a
primitive. For per-turn detail, `GET /api/v1/runs/{id}` returns the full run.
