# {{.Name}}

{{.Description}}

A blank LangSmith custom app, scaffolded by `langsmith apps init --type standalone`.
**Read `AGENTS.md` first** — it documents the LangSmith API surface this app
can call.

## What this is

A custom app is a small UI rendered inside LangSmith in a locked-down
sandbox (`sandbox="allow-scripts"`, no `allow-same-origin`). It never gets
direct network access or a real credential — everything goes through a
`postMessage` bridge to the host page (`window.langsmith.call`), which
proxies to the real LangSmith API under the viewer's own session (or your
local `LANGSMITH_API_KEY` when running `langsmith apps dev`).

Since the sandbox has no bundler or npm access at runtime, `src/index.js`
is bundled locally with esbuild into a single dependency-free file
(`dist/bundle.js`) before it's pushed. Use real JS and npm dependencies
freely — they all get inlined at build time.

## Develop

`langsmith apps init` already ran `npm install` and `npm run build` for you,
so `dist/bundle.js` exists and `langsmith apps dev` works immediately. To
keep iterating:

```bash
npm run watch      # rebuilds dist/bundle.js on change
langsmith apps dev # in another terminal
```

`apps dev` runs the app inside a real sandboxed iframe, identical
restrictions to production, and reloads automatically whenever
`dist/bundle.js` changes. If this app is linked (via `.langsmith/app.json`,
written by `apps push`) as `context_type: annotation_queue`, pass
`--queue-id <a real queue ID>` so `data.queueId` is populated.

## The bridge contract

Your entrypoint must export (or `module.exports`) a `render(data, root)`
function:

```js
module.exports = {
  render(data, root) {
    // data: {} for a standalone app, or { queueId } for an
    // annotation-queue app. root: an empty <div> to render into.
  },
};
```

`window.langsmith`, injected by the host page, gives you:

- `window.langsmith.call(operation, args)` — `operation` is a
  `"<METHOD> <path>"` string (e.g. `"GET /api/v1/annotation-queues/{id}/runs"`),
  forwarded as-is to the real LangSmith API. Not a curated allowlist — see
  `AGENTS.md` for the available surface. Returns a Promise.
- `window.langsmith.setData(patch)` — push a data mutation out for the host
  to persist.
- `window.langsmith.feedback.create(args)` — sugar over
  `call('POST /api/v1/feedback', {body: args})`.

## Deploy

```bash
npm run build
langsmith apps push
```

The first `push` creates the app and writes `.langsmith/app.json` (the
app's ID) into this directory — commit it so teammates' pushes update the
same app instead of creating a new one. Every push after that updates it in
place.
