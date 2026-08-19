import { createElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import css from './index.css?inline';
import { App } from './App';
import { DesignSystemProvider } from './components/DesignSystemProvider';

let root: Root | null = null;
let styleInjected = false;

function ensureStyles() {
  if (styleInjected) return;
  const style = document.createElement('style');
  style.textContent = css;
  document.head.appendChild(style);
  styleInjected = true;
}

export function render(data: unknown, container: HTMLElement, metadata: RenderMetadata) {
  ensureStyles();
  if (!root) {
    root = createRoot(container);
  }
  // metadata.mode is "dark"|"light". The sandbox sets html.dark from it, so
  // this token UI themes without branching; passed to App.
  root.render(
    createElement(DesignSystemProvider, null, createElement(App, { data, metadata }))
  );
}

// Some sandbox hosts call a bare default export; support both shapes.
export default { render };
