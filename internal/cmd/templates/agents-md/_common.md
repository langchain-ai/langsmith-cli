# AGENTS.md — building a LangSmith custom app

This app runs in a sandboxed iframe with no network access of its own —
every LangSmith API call goes through `window.langsmith.call`. `src/entry.tsx`
exports `render(data, root, metadata)`; keep that shape, the sandbox depends
on it. `data` is normally `{}`; call `window.langsmith.setData(patch)` if you
need to push a mutation out for the host to persist.

## Calling the LangSmith API

```ts
const projects = await window.langsmith.call('GET /api/v1/sessions', {
  params: { limit: '20' },
});
```

`operation` is `"<METHOD> <path>"` — use the full path including its prefix
(`/api/v1/...` for Python-hosted endpoints, `/v1/platform/...` and `/v2/...`
for Go-hosted ones). `args` carries `params` (query string) and/or `body`
(JSON). This is a generic passthrough, not a curated allowlist — anything
your API key can already do works; a permission error is a real limit of the
key. Full reference: https://docs.langchain.com/langsmith/home. Base URL:
`https://api.smith.langchain.com` (or your self-hosted instance's URL).

While `langsmith apps dev` is running, the app's failed API calls (with status
codes and error messages) and uncaught errors stream to that terminal — read it
to debug without opening browser devtools. Add `--verbose` to also see every
successful call and all `console.*` output, or `--quiet` to silence it.

## Theme

`metadata.mode` is `"dark"` | `"light"`. The sandbox sets `html.dark` from it
before every render, so Tailwind/token-based UIs theme for free with no
branching. Only branch on it yourself if you're using inline styles — and
re-check it every render, since it can change without a remount.

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
- `POST /api/v1/runs/stats` — server-side aggregates over a filtered set of runs, no row limit (counts, error rate, latency percentiles, token/cost sums) — prefer this over paging through `runs/query` for any headline number
- `POST /api/v1/runs/group/stats` — same, grouped (e.g. `group_by: "conversation"` for a distinct thread count)
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
