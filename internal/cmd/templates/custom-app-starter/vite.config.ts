import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Library-mode build: the sandbox this app runs in evaluates a single
// dependency-free CJS file exporting { render(data, root) } — not a
// self-mounting SPA. See src/entry.tsx and AGENTS.md.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    lib: {
      entry: 'src/entry.tsx',
      formats: ['cjs'],
      fileName: () => 'bundle.js',
    },
  },
});
