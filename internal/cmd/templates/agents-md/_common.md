# AGENTS.md — building a LangSmith custom app

This is a **standalone** custom app: a single dependency-free bundle the
LangSmith sandbox loads and calls as `render(data, root, metadata)`. The host
gives it no bound context — `data = {}` unless you push your own via
`window.langsmith.setData(patch)` — so what to show and what to fetch is up to
the app. It has **no network of its own**: every API call goes through
`window.langsmith.call` (below); `fetch`/XHR will not work.

## The render contract

`src/entry.tsx` default-exports `{ render(data, root, metadata) }` and injects
its CSS inline (`import css from './index.css?inline'`). Keep that shape — the
sandbox depends on it.

## Theme (`metadata.mode`)

`render`'s third argument is `metadata`, an open object; v1 has one key,
`metadata.mode` (`"dark"` | `"light"`). The sandbox sets `html.dark` from it
before every render, so Tailwind/token-based UIs theme for **free** — no
branching. Only apps using **inline styles** branch on `metadata.mode`. `mode`
is re-sent (and `render` re-called) whenever it changes, so react to it each
render, not once.

## Calling the LangSmith API

Everything goes through `window.langsmith.call(operation, args)` — see
README.md for the exact contract. `operation` is a `"<METHOD> <path>"` string;
use the full path including its prefix (`/api/v1/...` for Python-hosted
endpoints, `/v1/platform/...` and `/v2/...` for Go-hosted ones).

```ts
const projects = await window.langsmith.call('GET /api/v1/sessions', {
  params: { limit: '20' },
});
```

`args` carries `params` (query string) and/or `body` (JSON). This is a generic
passthrough, not a curated allowlist — any operation your API key already
permits works; a permission error is a real limit of the key, not something to
work around client-side. Full reference:
https://docs.langchain.com/langsmith/home. Base URL:
`https://api.smith.langchain.com` (or your self-hosted instance's URL).

## Filter DSL for metadata equality

Query endpoints take a `filter` string. Metadata equality is **two paired
clauses**, not `eq(metadata.key, ...)`:

```
and(eq(metadata_key, "ls_agent_purpose"), eq(metadata_value, "coding"))
```

Combine with `and(...)` / `or(...)`; other examples: `has(tags, "prod")`,
`eq(name, "ChatOpenAI")`, `eq(is_root, true)`.

<!-- TEMPLATE-SPECIFIC -->

## More of the LangSmith API (starting points, not exhaustive)

**Runs**
- `POST /api/v1/runs/query` — query runs (body: `session`, `filter`, `is_root`, `run_type`, `start_time`, `limit`, `select`); returns `{ runs, cursor }`
- `GET /api/v1/runs/{run_id}` — fetch one full run (all fields + inputs/outputs)
- `POST /api/v1/runs` / `PATCH /api/v1/runs/{run_id}` — create / update a run

**Projects (tracing sessions)**
- `GET /api/v1/sessions` — list projects
- `GET /api/v1/sessions/{session_id}` — get a project

**Datasets & experiments**
- `GET /api/v1/datasets` — list datasets
- `POST /api/v1/datasets/{dataset_id}/runs` — per-example rows across experiments
- `POST /v1/platform/datasets/{dataset_id}/examples` — create examples

**Feedback**
- `POST /api/v1/feedback` — create feedback
- `GET /api/v1/feedback?run={run_id}` — list feedback for a run
- `GET /api/v1/feedback-configs?key={key}` — a feedback key's type / direction

**Annotation queues**
- `GET /api/v1/annotation-queues` — list queues
- `GET /api/v1/annotation-queues/{queue_id}/runs` — list runs in a queue

**Threads**
- `POST /v2/threads/query` — query threads
- `GET /v2/threads/{thread_id}/traces` — get a thread's traces

If you need something not listed, check the docs — almost everything in the
LangSmith API is reachable this way.
