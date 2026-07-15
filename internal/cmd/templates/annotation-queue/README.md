# {{.Name}}

{{.Description}}

A LangSmith custom app, scaffolded by `langsmith apps init` — a real,
working annotation-queue review UI (run list, inputs/outputs viewer,
feedback rubric, reviewer notes) you're meant to vibe-code from here, not a
toy example. **Read `AGENTS.md` first** — it documents the LangSmith API
surface this app can call.

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

For `context_type: annotation_queue` apps (what this template is), the
*only* context the host hands you is `{ queueId }` — everything else (which
runs are in the queue, a run's inputs/outputs, submitting feedback, marking
a run complete) this app fetches itself via `window.langsmith.call`. See
`src/api.ts`.

## Develop

`langsmith apps init` already ran `npm install` and `npm run build` for you,
so `dist/bundle.js` exists and `langsmith apps dev` works immediately:

```bash
langsmith apps dev --queue-id <id>
```

`--queue-id` should be a real annotation queue ID from your workspace (the
UUID in `smith.langchain.com/o/<org>/annotation-queues/<QUEUE_ID>`) — this
app calls the real API for that queue's runs/feedback, so it needs a queue
that actually exists. `apps dev` runs the app inside a real sandboxed
iframe, identical restrictions to production, and automatically starts
`npm run watch` for you (it detects the script in `package.json`) — edit
the app, save, and the preview reloads on its own. Pass `--no-watch` if
you'd rather drive the build yourself.

## The bridge contract

`src/entry.tsx` exports a `render(data, root)` function — the sandbox calls
it once when `data` (`{ queueId }`) arrives, and again if it changes:

```ts
export default {
  render(data: { queueId?: string }, root: HTMLElement) {
    // mount your app into root
  },
};
```

`window.langsmith`, injected by the host page, gives you:

- `window.langsmith.call(operation, args)` — `operation` is a
  `"<METHOD> <path>"` string (e.g. `"GET /api/v1/annotation-queues/{id}/runs"`),
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
