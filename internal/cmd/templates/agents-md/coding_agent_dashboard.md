# AGENTS.md — LangSmith API surface for this app

This is a **coding-agent dashboard**: it charts root runs whose metadata
`ls_agent_purpose == "coding"`, grouped by `ls_integration` and `ls_model_name`.
Apps are uniform — the host gives you no bound context, so
`render(data, root, metadata)` receives `data = {}`. The app picks its own
project (`src/components/ProjectBar.tsx`, via `GET /api/v1/sessions`) because
the runs query is project-scoped. Everything is fetched via
`window.langsmith.call`. `src/api.ts` already wraps every call below.

## Theme (`metadata`)

`render`'s third argument is `metadata`; v1 has one key, `metadata.mode`
(`"dark"` | `"light"`). The sandbox sets `html.dark` from it before every
render, so this Tailwind/token UI themes for free — no branching. Chart colors
use `index.css` tokens that flip under `html.dark` (`src/lib/palette.ts`), so
slices stay legible in both themes.

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
- `POST /api/v1/runs/query` — query runs; body fields used here:
  - `session: ["<project-uuid>"]` — required to scope the query
  - `is_root: true` — top-level agent runs only
  - `start_time: "<ISO>"` — recent window (this app uses last 7 days)
  - `limit: 100`
  - `filter` — the filter DSL string (see below)

## Filter DSL for metadata equality

Metadata equality is **two paired clauses**, not `eq(metadata.key, ...)`:

```
and(eq(metadata_key, "ls_agent_purpose"), eq(metadata_value, "coding"))
```

Combine with `and(...)` / `or(...)`; other examples: `has(tags, "prod")`,
`eq(name, "ChatOpenAI")`, `eq(is_root, true)`.

## Reading a run

`POST /api/v1/runs/query` returns `{ runs: [...] }`. Custom metadata lives at
`run.extra.metadata`. This app reads `ls_integration` there; for the model it
prefers `ls_model_name` and falls back to `model` (root runs carry `model`,
while `ls_model_name` sits on child LLM runs). A non-null `run.error` counts
as a failed run. No per-run follow-up calls are made.

## What this app already does

- `src/api.ts` — `fetchProjects` and `fetchCodingRuns` (the two calls above)
- `src/components/ProjectBar.tsx` — lists projects and drives the selection
- `src/App.tsx` — fetches runs, aggregates by integration/model, assigns colors
- `src/components/PieChart.tsx` — hand-drawn SVG pie (no chart dependency)
- `src/components/IntegrationBreakdown.tsx` — per-integration model counts + error rate
