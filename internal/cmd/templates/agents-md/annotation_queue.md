# AGENTS.md — LangSmith API surface for this app

This is an **annotation_queue** custom app. `render(data, root, metadata)`
receives `data = { queueId }` — that's the *only* context the host gives you.
You fetch everything else (run list, run detail, feedback, ...) yourself via
`window.langsmith.call`. `src/api.ts` already wraps every operation below —
read it before adding new API calls, you likely don't need to.

`render`'s third argument is `metadata`, an open/extensible object; v1 has
exactly one key, `metadata.mode` (`"dark"` | `"light"`). The sandbox sets
`html.dark` from `mode` before every render, so this Tailwind/token-based app
gets dark mode for **free** — no branching needed. Only apps that use **inline
styles** need to branch on `metadata.mode`. `mode` is re-sent (and `render`
re-called) whenever it changes.

## Calling the API

```ts
const runs = await window.langsmith.call(
  `GET /api/v1/annotation-queues/${queueId}/runs`,
  { params: { status: 'needs_my_review' } }
);
```

Generic passthrough, not a curated allowlist — any operation your API key
already permits can be called. The list below is the practically-relevant
subset for a queue-review UI; the full LangSmith API is also reachable if
you need more of it (see `AGENTS.md` for standalone apps, or the docs:
https://docs.langchain.com/langsmith/home).

## Annotation queues

- `GET /api/v1/annotation-queues/{queue_id}` — queue detail (rubric items, instructions, num_reviewers_per_item)
- `GET /api/v1/annotation-queues/{queue_id}/runs` — list runs in the queue (filter by `status`: `needs_my_review` | `needs_others_review` | `completed`)
- `PATCH /api/v1/annotation-queues/{queue_id}/runs/{queue_run_id}` — update a queue run (status/notes)
- `POST /api/v1/annotation-queues/status/{queue_run_id}` — set a queue run's review status (this app uses `{status: 'completed'}` to mark done)
- `GET /api/v1/annotation-queues/{queue_id}/runs/resolve/{queue_run_id}` — resolve a queue-run ID to its section (for deep links)
- `GET /api/v1/annotation-queues/{queue_id}/total_size` / `/total_archived` / `/size` — size counters (accept a `status` filter)

## Feedback

- `POST /api/v1/feedback` — submit feedback (`{key, run_id, score?, value?, comment?}`)
- `GET /api/v1/feedback?run={run_id}` — list feedback for a run
- `PATCH /api/v1/feedback/{feedback_id}` — update feedback
- `DELETE /api/v1/feedback/{feedback_id}` — delete feedback
- `GET /api/v1/feedback-configs?key={key}` — fetch the type/min/max/categories for one or more feedback keys (repeat the `key` param for multiple)

## What this app already does with these

- `src/api.ts` — thin wrappers over every operation above
- `src/App.tsx` — fetches the queue + its runs, tracks the selected run, h/l keyboard navigation, Cmd/Ctrl+Enter to complete
- `src/components/FeedbackPanel.tsx` — renders the rubric, saves/deletes feedback per key
- `src/components/ReviewerNotes.tsx` — a per-run comment thread built on the feedback API's reserved `"note"` key
