# AGENTS.md — LangSmith API surface for this app

This is a **standalone** custom app (`context_type: none`) — genuinely
open-ended. `render(data, root, metadata)` receives `data = {}` (or whatever
you push yourself via `window.langsmith.setData(patch)`); there's no bound
context. What to show and what data to fetch is entirely up to you.

## Theme (`metadata`)

`render`'s third argument is `metadata`, an open/extensible object; v1 has
exactly one key, `metadata.mode` (`"dark"` | `"light"`). The sandbox sets
`document.documentElement.classList` `dark` from `mode` before every render,
so Tailwind/token-based apps (like this starter's `index.css`) get dark mode
for **free** — no branching needed. Only apps that use **inline styles** need
to branch on `metadata.mode` themselves. `mode` is re-sent (and `render`
re-called) whenever it changes, so react to it on each render, not just once.

## Calling the API

Everything goes through `window.langsmith.call(operation, args)` — see
README.md for the exact contract. `operation` is a `"<METHOD> <path>"`
string; use the full path including its prefix (`/api/v1/...` for
Python-hosted endpoints, `/v1/platform/...` for Go-hosted ones) exactly as
shown below.

```ts
const projects = await window.langsmith.call('GET /api/v1/sessions', {
  params: { limit: '20' },
});
```

This is a generic passthrough, not a curated allowlist — any operation your
API key already has permission for can be called; nothing here narrows that.
Full reference: https://docs.langchain.com/langsmith/home. Base URL:
`https://api.smith.langchain.com` (or your self-hosted instance's URL).

## Common resource groups (starting points, not exhaustive)

**Runs**
- `POST /api/v1/runs` — create a run
- `PATCH /api/v1/runs/{run_id}` — update a run
- `POST /v2/runs/query` — query runs
- `GET /v2/runs/{run_id}` — fetch a single run

**Projects (tracing sessions)**
- `GET /api/v1/sessions` — list projects
- `GET /api/v1/sessions/{session_id}` — get a project

**Datasets & examples**
- `GET /api/v1/datasets` — list datasets
- `POST /v1/platform/datasets/{dataset_id}/examples` — create examples
- `PATCH /v1/platform/datasets/{dataset_id}/examples` — bulk-update examples

**Experiments**
- `POST /v1/platform/datasets/{dataset_id}/experiments/grouped` — grouped experiment results for a dataset
- `POST /v2/datasets/{dataset_id}/experiment-runs` — v2 experiment runs

**Feedback**
- `POST /api/v1/feedback` — create feedback
- `GET /api/v1/feedback?run={run_id}` — list feedback for a run

**Annotation queues**
- `GET /api/v1/annotation-queues` — list queues
- `GET /api/v1/annotation-queues/{queue_id}/runs` — list runs in a queue

**Prompts (Hub)**
- `GET /api/v1/commits/{owner}/{repo}` — list commits for a prompt repo

**Threads**
- `POST /v2/threads/query` — query threads
- `GET /v2/threads/{thread_id}/traces` — get a thread's traces

If you need something not listed here, check the docs — almost everything
in the LangSmith API is reachable this way. If a call fails with a
permission error, that's a real limitation of the underlying API key/token,
not something to work around client-side.
