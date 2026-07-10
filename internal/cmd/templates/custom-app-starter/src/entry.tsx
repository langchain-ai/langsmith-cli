import { createRoot, type Root } from 'react-dom/client';
import { StrictMode } from 'react';
import { App } from './App';
// `?inline` gets the fully-processed (Tailwind + autoprefixer) CSS as a plain
// string instead of Vite emitting a separate stylesheet asset — there's no
// way to <link> a second file into this sandbox; everything has to be in the
// one bundled entrypoint. See README.md.
import css from './index.css?inline';

let styleInjected = false;
let root: Root | null = null;

function ensureStyleInjected() {
  if (styleInjected) return;
  const style = document.createElement('style');
  style.textContent = css;
  document.head.appendChild(style);
  styleInjected = true;
}

interface RenderData {
  queueId?: string;
}

export default {
  render(data: RenderData, rootEl: HTMLElement) {
    ensureStyleInjected();
    if (!root) root = createRoot(rootEl);
    root.render(
      <StrictMode>
        <App queueId={data?.queueId ?? ''} />
      </StrictMode>
    );
  },
};
