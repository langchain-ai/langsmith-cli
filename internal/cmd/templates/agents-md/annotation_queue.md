## This app: annotation-queue review

A queue-review UI (run list, inputs/outputs viewer, feedback rubric, reviewer
notes). It picks its own queue — `src/components/QueueBar.tsx` lists the
workspace's queues via `GET /api/v1/annotation-queues` and the user selects one
— then fetches everything else via `window.langsmith.call`. `src/api.ts` wraps
every operation below; read it before adding calls, you likely don't need to.

```ts
const runs = await window.langsmith.call(
  `GET /api/v1/annotation-queues/${queueId}/runs`,
  { params: { status: 'needs_my_review' } }
);
```

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

- `src/api.ts` — thin wrappers over every operation above (incl. `fetchQueues` for the picker)
- `src/components/QueueBar.tsx` — lists the workspace's queues and drives the selection
- `src/App.tsx` — owns the selected queue, fetches the queue + its runs, tracks the selected run, h/l keyboard navigation, Cmd/Ctrl+Enter to complete
- `src/components/FeedbackPanel.tsx` — renders the rubric, saves/deletes feedback per key
- `src/components/ReviewerNotes.tsx` — a per-run comment thread built on the feedback API's reserved `"note"` key
