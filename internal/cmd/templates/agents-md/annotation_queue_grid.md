## This app: annotation-queue review, as a spreadsheet grid

A queue-review UI rendered as a **spreadsheet grid** (rows = queue runs,
columns = rubric keys). It picks its own queue — `src/components/QueueBar.tsx`
lists the workspace's queues via `GET /api/v1/annotation-queues` and the user
selects one — then fetches everything else via `window.langsmith.call`.
`src/api.ts` wraps every operation below; read it before adding calls, you
likely don't need to.

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
- `POST /api/v1/annotation-queues/status/{queue_run_id}` — set a queue run's review status (this app uses `{status: 'completed'}` to mark a row done)
- `GET /api/v1/annotation-queues/{queue_id}/runs/resolve/{queue_run_id}` — resolve a queue-run ID to its section (for deep links)
- `GET /api/v1/annotation-queues/{queue_id}/total_size` / `/total_archived` / `/size` — size counters (accept a `status` filter)

## Feedback

- `POST /api/v1/feedback` — submit feedback (`{key, run_id, score?, value?, comment?}`)
- `GET /api/v1/feedback?run={run_id}` — list feedback for a run
- `PATCH /api/v1/feedback/{feedback_id}` — update feedback
- `DELETE /api/v1/feedback/{feedback_id}` — delete feedback
- `GET /api/v1/feedback-configs?key={key}` — fetch the type/min/max/categories for one or more feedback keys (repeat the `key` param for multiple)

## How the grid is built from these

- `src/api.ts` — thin wrappers over every operation above (identical to the 3-pane annotation-queue template; reuse these, don't invent new calls), plus `fetchQueueRunsSize` for pagination.
- `src/hooks/useRunSection.ts` — pages through `needs_my_review` runs a page at a time (`GET /runs` + `GET /size`, since the bridge can't see the `/runs` response's total-count header), exposing `loadMore`/`hasMore`/`loadingMore` and dedupe-by-`queue_run_id`.
- `src/components/QueueBar.tsx` — lists the workspace's queues and drives the selection
- `src/App.tsx` — owns the selected queue, the paginated run list, derives the
  columns from `queue.rubric_items`, fetches each column key's config with
  `GET /feedback-configs`, and prefetches each newly-loaded row's existing
  feedback with `GET /feedback?run=...`. Owns ArrowUp/ArrowDown row
  navigation, row expand/collapse, and the optimistic row removal on Mark
  Completed (`POST /annotation-queues/status/{id}`).
- `src/components/DataGrid.tsx` — the table: sticky header, columns in order
  **Run Name | Inputs | Outputs | one per rubric key | Mark Completed**.
  Inputs/Outputs show a truncated one-line JSON preview by default; clicking
  a row's name expands a panel below it with the full pretty-printed
  inputs/outputs. An `IntersectionObserver` sentinel at the bottom of the
  scroll container triggers `loadMore` for infinite scroll.
- `src/components/GridCell.tsx` — one editable cell. The rubric item carries no
  type; the cell's editor (categorical `<select>` / continuous number input /
  freeform text) comes from that key's `feedback_config`, defaulting to
  freeform when unconfigured. Each cell saves as-you-go via
  `POST`/`PATCH`/`DELETE /feedback`, always passing the row's `trace_id`/
  `session_id`/`start_time` alongside `run_id` (without them the backend has
  to look the run up, which can silently miss instead of writing inline), so
  feedback persists before the row is marked Completed.

## Rubric flags this app honors

- **`is_required`** — required columns are marked with `*` and a row's `Mark
  Completed` button stays disabled until every required column has feedback
  (mirrors the 3-pane app's "all required filled" gate).
- **`is_assertion`** — assertion-flagged rubric items are pass/fail claims
  managed elsewhere (the run review panel's own Assertions section), not
  scored feedback keys — the real "Edit Annotation Queue" page excludes them
  from its Feedback Rubrics list for the same reason. Filtered out of the
  grid's columns entirely; this app has no other assertion-specific UI.
