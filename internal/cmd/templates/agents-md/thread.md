## This template: thread context

This app declares **context type `thread`**: it's embedded in a tracing
project's thread view, and the host hands it `{ threadId, projectId }` as
`render`'s `data` (the first argument). Everything else it fetches itself
through `window.langsmith.call`.

`data` is `{ threadId: string; projectId: string }`. Under
`langsmith apps dev` both are empty strings unless you pass
`--thread-id <id> --project-id <id>` — handle that empty case (this starter
shows a hint instead of erroring).

The starter loads the thread's chronological messages with:

```ts
await window.langsmith.call('POST /v1/trajectory', {
  body: { project_id: projectId, thread_id: threadId, format: 'messages' },
});
```

which returns `{ messages, next_cursor }`; page with `cursor` until
`next_cursor` is null. (Don't use `GET /v2/threads/{id}/messages` — it's
SSE-only.) See `src/App.tsx`.

Context type is fixed when the app is first pushed and cannot be changed. To
switch an app between context types, delete and recreate it.

This is just a starting point, not a spec. Change the layout, fetch different
thread-scoped data, or build whatever thread-view app you actually want.
