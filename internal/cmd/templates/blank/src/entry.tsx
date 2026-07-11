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

export function render(data: unknown, container: HTMLElement) {
  ensureStyles();
  if (!root) {
    root = createRoot(container);
  }
  root.render(createElement(App, { data }));
}

// Some sandbox hosts call a bare default export; support both shapes.
export default { render };
