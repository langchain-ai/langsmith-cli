# {{.Name}}

{{.Description}}

A LangSmith **thread-context** custom app, scaffolded by `langsmith apps init
--template thread`. It's embedded in a tracing project's thread view and is
handed `{ threadId, projectId }` as `render`'s `data`; it fetches the thread's
messages itself and lists them. **Read `AGENTS.md` first** — it documents the
LangSmith API surface this app can call.

## What this is

A custom app is a small React/TS UI rendered inside LangSmith in a
locked-down sandbox (`sandbox="allow-scripts"`, no `allow-same-origin`). It
never gets direct network access or a real credential — everything goes
through a `postMessage` bridge to the host page (`window.langsmith.call`),
which proxies to the real LangSmith API under the viewer's own session (or
your local `LANGSMITH_API_KEY` when running `langsmith apps dev`).

Because this app declares **context type `thread`**, the host binds it to a
specific thread at render time and passes `{ threadId, projectId }` as the
first argument to `render(data, root, metadata)`. Context type is fixed when
the app is first pushed and cannot be changed later.

Since the sandbox has no bundler or npm access at runtime, this app is built
with Vite in **library mode**: `npm run build` bundles everything (React
included) into a single dependency-free file (`dist/bundle.js`) before it's
pushed. Use npm dependencies freely — they all get inlined at build time.

## Develop

Install dependencies, then start the dev server with a real thread to bind to:

```bash
npm install
langsmith apps dev --thread-id <thread-id> --project-id <project-id>
```

`apps dev` runs the app inside a real sandboxed iframe, identical restrictions
to production, and automatically starts `npm run watch` for you — edit
`src/App.tsx`, save, and the preview reloads on its own. Without
`--thread-id`/`--project-id` the app renders with empty context and shows how
to pass them.

## The bridge contract

`src/entry.tsx` exports a `render(data, root, metadata)` function that the
host calls once on load and again whenever `data` or `metadata` changes. For a
thread app, `data` is `{ threadId, projectId }` (both empty strings in dev
until you pass the flags). `metadata.mode` is `"dark"` or `"light"`; the
sandbox sets `html.dark` from it, so this token-based UI themes automatically.
`src/App.tsx` is the actual UI; edit it freely.

`window.langsmith`, injected by the host page, gives you:

- `window.langsmith.call(operation, args)` — `operation` is a
  `"<METHOD> <path>"` string (e.g. `"POST /v1/trajectory"`), forwarded as-is
  to the real LangSmith API. See `AGENTS.md` for the available surface.
- `window.langsmith.setData(patch)` — push a data mutation out for the host
  to persist.
- `window.langsmith.feedback.create(args)` — sugar over
  `call('POST /api/v1/feedback', {body: args})`.

This starter loads the thread's messages via `POST /v1/trajectory`
(`format: "messages"`) using the `projectId` and `threadId` it's given — see
`src/App.tsx`.

## Deploy

```bash
npm run build
langsmith apps push
```

The first `push` creates the app with `context_type: thread` (recorded in
`.langsmith/app.json`) and writes the app's ID there — commit it so teammates'
pushes update the same app. Every push after that updates it in place; the
context type is fixed and cannot change.
