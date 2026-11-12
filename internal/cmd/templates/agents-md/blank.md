## Building blocks (`src/blocks`)

This starter ships tested components for the parts most apps need. Prefer them
over hand-rolled versions, and read `src/blocks/*.tsx` for every prop. They are
built from Macaw components and semantic tokens.

```tsx
import { Query, useQuery, callWithRetry, LoadMore, ThreadViewer, RunViewer, useRun, useThreadMessages, FeedbackForm, completeQueueItem } from './blocks';
```

- **`useQuery(operation, { params, body, enabled, itemsPath, paging })`** loads
  through `window.langsmith.call`, re-fetches when its inputs change, retries
  HTTP 429 with backoff and jitter, ignores stale responses, and pages one
  request at a time (`paging: 'cursor'` passes `next_cursor` back as `cursor`;
  `'offset'` uses `limit`/`offset`; `false` for single objects). It returns
  `{ data, items, loading, loadingMore, error, hasMore, loadMore, refresh }`.
- **`<Query operation=… emptyText=… skeleton="table">{(r) => …}</Query>`** wraps
  it with a skeleton while loading, an error banner with Retry, an empty state,
  and a Load more button. `callWithRetry(operation, { params, body })` is the
  same retrying call for one-off reads and writes.
- **`<ThreadViewer messages={…} />`** renders LangChain, OpenAI or Anthropic
  messages by role, with tool calls grouped and collapsible.
  `useThreadMessages({ projectId, threadId })` loads a thread through
  `POST /v1/trajectory`, oldest first, following up to ten pages; `truncated`
  is true when the thread has more.
- **`<RunViewer run={…} />`** shows a run's inputs, outputs and error readably.
  `useRun({ runId, projectId, startTime })` fetches `GET /v2/runs/{id}` with
  `RUN_SELECTS`.
- **`<FeedbackForm runId|threadId projectId rubric={queue.rubric_items} onSubmitted={…} />`**
  renders a queue rubric. Each item's pinned `feedback_config` sets its type
  (categories, a continuous score checked against the config's bounds, or free
  text), and the rubric's descriptions label the categories; an item without a
  config shows as unavailable. It validates required items and posts one
  `POST /api/v1/feedback` per answer with the item's `feedback_config_id` and a
  client `id`. A retry skips answers that already saved and resends the rest
  with the same ids, and an `onSubmitted` error is reported apart from save
  errors. Pass the item's `project_id` as `projectId`: thread feedback is
  rejected without it. For a RUN item, also pass the run's `trace_id` and
  `start_time` from `useRun` as `traceId` and `startTime`, as LangSmith's own
  review form does: run feedback without them may never be saved.
  Multi-select categories and `visible_if`/`required_if` rules are not
  supported (an item with `visible_if` always shows and is optional), and
  `regex_validator` is not enforced. The form only creates feedback: it doesn't
  load or edit a reviewer's earlier answers. Key it by queue item
  (`key={item.id}`) so each item starts fresh. `completeQueueItem(itemId)`
  records your completion of an item; the item moves to Completed once the
  queue's review requirements are met.

Queue lists omit `rubric_items`, so fetch the queue by id for its rubric and
pinned configs:

```tsx
const queue = useQuery<Queue>(`GET /api/v1/annotation-queues/${queueId}`, { paging: false });
const thread = useThreadMessages({ projectId: item.project_id, threadId: item.thread_id });
return (
  <>
    <ThreadViewer messages={thread.messages} />
    <FeedbackForm
      key={item.id}
      threadId={item.thread_id}
      projectId={item.project_id}
      rubric={queue.data?.rubric_items ?? []}
      onSubmitted={async () => {
        await completeQueueItem(item.id);
        next();
      }}
    />
  </>
);
```
