// {{.Name}} — a LangSmith custom app.
//
// This file is bundled by esbuild (see package.json's "build" script) into
// dist/bundle.js, which is what actually gets pushed. Add npm dependencies
// freely — anything you `require()` gets bundled in, since the sandbox this
// runs in has no access to node_modules or the network at runtime.
//
// render(data, root) is called once when data arrives, and again whenever
// it changes. data is {} for a standalone app, or { queueId } for an
// annotation-queue app — see AGENTS.md for what's available either way.
//
// window.langsmith, injected by the host page, is the only way to reach the
// LangSmith API (window.langsmith.call(operation, args)) — see AGENTS.md
// and README.md for the full bridge contract.
module.exports = {
  render: function (data, root) {
    var pre = document.createElement('pre');
    pre.style.cssText = 'white-space:pre-wrap;word-break:break-word;';
    pre.textContent = JSON.stringify(data, null, 2);
    root.appendChild(pre);
  },
};
