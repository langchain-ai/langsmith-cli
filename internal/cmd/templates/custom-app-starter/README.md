# {{.Name}}

{{.Description}}

A LangSmith custom app, scaffolded by `langsmith apps init`.

## What this is

A custom app is a small UI rendered inside LangSmith in a locked-down
sandbox (`sandbox="allow-scripts"`, no `allow-same-origin`). It never gets
direct network access or credentials — everything goes through a
`postMessage` bridge to the host page, which proxies a small allowlist of
API calls using the viewer's own LangSmith session.

Because the sandbox has no bundler or npm access at runtime, `src/index.js`
is bundled locally with esbuild into a single dependency-free file
(`dist/bundle.js`) before it's uploaded. Use real JS/JSX/TS and npm
dependencies freely — they all get inlined at build time.

## Develop

```bash
npm install
npm run watch          # rebuilds dist/bundle.js on change
langsmith apps dev --dir .
```

`apps dev` opens a LangSmith preview pointed at `dist/bundle.js`, live-reloading
as you edit.

## The bridge contract

Your entrypoint must export (or `module.exports`) a `render(data, root)`
function:

```js
export default {
  render(data, root) {
    // data: {} for context_type "none", or {inputs, outputs} for an
    // annotation-queue app. root: an empty <div> to render into.
  },
};
```

`window.langsmith`, injected by the host page, gives you:

- `window.langsmith.call(method, args)` — invoke an allowlisted API method
  (e.g. `'feedback.create'`), proxied through the viewer's session. Returns
  a Promise.
- `window.langsmith.setData(patch)` — merge a patch into this app's bound
  data (e.g. `{ outputs: {...} }`).
- `window.langsmith.feedback.create(args)` / `window.langsmith.data.updateInputs(v)` /
  `window.langsmith.data.updateOutputs(v)` — convenience wrappers around the
  two calls above.

## Deploy

```bash
npm run build
langsmith apps push --dir .
```

The first `push` creates the app and writes `.langsmith/app.json` (the app's
ID) into this directory — commit it so teammates' pushes update the same
app instead of creating a new one. Every push after that updates it in
place.
