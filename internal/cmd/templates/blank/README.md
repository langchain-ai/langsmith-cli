# {{.Name}}

{{.Description}}

A standalone LangSmith custom app, scaffolded by `langsmith apps init --type
standalone`. It starts as a small, genuinely working example — a list of
your workspace's most recently active tracing projects — not a blank page.
**Read `AGENTS.md` first** — it documents the LangSmith API surface this
app can call.

## What this is

A custom app is a small React/TS UI rendered inside LangSmith in a
locked-down sandbox (`sandbox="allow-scripts"`, no `allow-same-origin`). It
never gets direct network access or a real credential — everything goes
through a `postMessage` bridge to the host page (`window.langsmith.call`),
which proxies to the real LangSmith API under the viewer's own session (or
your local `LANGSMITH_API_KEY` when running `langsmith apps dev`).

Since the sandbox has no bundler or npm access at runtime, this app is built
with Vite in **library mode**: `npm run build` bundles everything (React
included) into a single dependency-free file (`dist/bundle.js`) before it's
pushed. Use npm dependencies freely — they all get inlined at build time.

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
`dist/bundle.js` changes.

## The bridge contract

`src/entry.tsx` exports a `render(data, root)` function that the host calls
once on load and again whenever `data` changes — for a standalone app,
`data` is always `{}`. `src/App.tsx` is the actual UI; edit it freely, it's
just a React component.

`window.langsmith`, injected by the host page, gives you:

- `window.langsmith.call(operation, args)` — `operation` is a
  `"<METHOD> <path>"` string (e.g. `"GET /api/v1/sessions"`), forwarded
  as-is to the real LangSmith API. Not a curated allowlist — see
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
