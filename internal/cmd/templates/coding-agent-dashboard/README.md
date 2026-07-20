# {{.Name}}

{{.Description}}

A LangSmith custom app, scaffolded by `langsmith apps init --template
coding-agent-dashboard`. It charts your coding-agent runs: pick a project,
and it shows the share of each `ls_integration` as a pie plus a per-model
breakdown (with run counts and error rates). **Read `AGENTS.md` first** — it
documents the LangSmith API surface this app can call.

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
so `dist/bundle.js` exists and `langsmith apps dev` works immediately:

```bash
langsmith apps dev
```

`apps dev` runs the app inside a real sandboxed iframe, identical
restrictions to production, and automatically starts `npm run watch` for
you (it detects the script in `package.json`) — edit `src/App.tsx`, save,
and the preview reloads on its own. Pass `--no-watch` if you'd rather drive
the build yourself. Pick a project from the in-app dropdown once it loads
(it calls the real API using your local `LANGSMITH_API_KEY`).

## The bridge contract

`src/entry.tsx` exports a `render(data, root, metadata)` function that the
host calls once on load and again whenever `data` or `metadata` changes —
`data` is always `{}` (apps fetch whatever they need themselves).
`metadata.mode` is `"dark"` or
`"light"`; the sandbox sets `html.dark` from it, so this token-based UI themes
automatically (branch on `metadata.mode` only if you use inline styles).
`src/App.tsx` is the actual UI; edit it freely, it's just a React component.

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
