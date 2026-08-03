# {{.Name}}

{{.Description}}

A LangSmith custom app, scaffolded by `langsmith apps init` — a real,
working annotation-queue review UI rendered as a **spreadsheet**: one row per
queue item (RUN or THREAD), one column per rubric key, cells edited inline
and saved as you go, `Mark Completed` to complete a row. Meant to vibe-code
from here, not a toy example. **Read `AGENTS.md` first** — it documents the
LangSmith API surface this app can call.

## What this is

A custom app is a small React/TS UI rendered inside LangSmith in a
locked-down sandbox (`sandbox="allow-scripts"`, no `allow-same-origin`). It
never gets direct network access or a real credential — everything goes
through a `postMessage` bridge to the host page (`window.langsmith.call`),
which proxies to the real LangSmith API under the viewer's own session (or
your local `LANGSMITH_API_KEY` when running `langsmith apps dev`).

Since the sandbox has no bundler or npm access at runtime, this app is built
with Vite in **library mode** into a single dependency-free CJS file
(`dist/bundle.js`) — see `vite.config.ts` and `src/entry.tsx`. Use real
React/TS and npm dependencies freely; they all get inlined at build time.
Tailwind's compiled CSS is inlined too (`src/index.css` imported with
`?inline` in `entry.tsx`) and injected via a `<style>` tag at first render —
there's no way to `<link>` a second file into this sandbox.

Apps are uniform — the host hands you no bound context (`data` is always
`{}`). This app picks its own annotation queue: the top bar lists your
workspace's queues (`GET /api/v1/annotation-queues`) and, once one is
selected, everything else (queue items via `/items`, RUN/THREAD hydrate on
expand, existing feedback, marking an item complete) is fetched via
`window.langsmith.call`. See `src/api.ts` and `src/components/QueueBar.tsx`.

## Develop

Install dependencies, then start the dev server (it builds on the first run):

```bash
npm install
langsmith apps dev
```

`apps dev` runs the app inside a real sandboxed iframe, identical
restrictions to production, and automatically starts `npm run watch` for you
(it detects the script in `package.json`) — edit the app, save, and the
preview reloads on its own. Pick a queue from the in-app dropdown once it loads (it calls the
real API using your local `LANGSMITH_API_KEY`, so it lists queues that
actually exist in your workspace).

## The bridge contract

`src/entry.tsx` exports a `render(data, root, metadata)` function — the
sandbox calls it once when `data` (`{}`) arrives, and again if `data` or
`metadata` changes. `metadata.mode` is `"dark"` or `"light"`; the sandbox
sets `html.dark` from it, so this Tailwind/token UI themes automatically
(branch on `metadata.mode` only if you use inline styles):

```ts
export default {
  render(data: {}, root: HTMLElement, metadata: { mode: 'dark' | 'light' }) {
    // mount your app into root
  },
};
```

`window.langsmith`, injected by the host page, gives you:

- `window.langsmith.call(operation, args)` — `operation` is a
  `"<METHOD> <path>"` string (e.g. `"GET /api/v1/platform/annotation-queues/{id}/items"`),
  forwarded as-is to the real LangSmith API. Not a curated allowlist — see
  `AGENTS.md` for the available surface. Returns a Promise.
- `window.langsmith.setData(patch)` — push a data mutation out for the host
  to persist (rarely needed; most apps just call the API directly).
- `window.langsmith.feedback.create(args)` — sugar over
  `call('POST /api/v1/feedback', {body: args})`, since that's extremely common.

## Deploy

```bash
npm run build
langsmith apps push
```

The first `push` creates the app and writes `.langsmith/app.json` (the app's
ID) into this directory — commit it so teammates' pushes update the same
app instead of creating a new one. Every push after that updates it in
place.
