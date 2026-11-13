import { createElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import css from './index.css?inline';
import { App } from './App';

let root: Root | null = null;
let styleInjected = false;

function ensureStyles() {
  if (styleInjected) return;
  const style = document.createElement('style');
  style.textContent = css;
  document.head.appendChild(style);
  styleInjected = true;
}

// A "thread" context app: `data` is { threadId, projectId } (empty in
// `langsmith apps dev` unless you pass --thread-id/--project-id). The host
// re-calls render() whenever data or metadata changes.
export function render(data: ThreadData, container: HTMLElement, metadata: RenderMetadata) {
  ensureStyles();
  if (!root) {
    root = createRoot(container);
  }
  root.render(createElement(App, { data, metadata }));
}

// Some sandbox hosts call a bare default export; support both shapes.
export default { render };
