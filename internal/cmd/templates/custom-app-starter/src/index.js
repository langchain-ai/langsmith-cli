// {{.Name}} — a LangSmith custom app.
//
// This file is bundled by esbuild (see package.json's "build" script) into
// dist/bundle.js, which is what actually gets pushed. Add npm dependencies
// freely — anything you `import` gets bundled in, since the sandbox this
// runs in has no access to node_modules or the network at runtime.
//
// The sandbox calls `render(data, root)` every time the data it's bound to
// changes (e.g. a different run selected). `data` is `{}` for context_type
// "none", or `{inputs, outputs}` for an annotation-queue app.
//
// `window.langsmith` is injected by the host page — use it to call back
// out through the allowlisted API proxy (e.g. `feedback.create`) or to
// push data mutations. See README.md for the full bridge contract.
export default {
  render(data, root) {
    const pre = document.createElement('pre');
    pre.style.cssText = 'white-space:pre-wrap;word-break:break-word;';
    pre.textContent = JSON.stringify(data, null, 2);
    root.appendChild(pre);
  },
};
