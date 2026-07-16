// A static starting point — no API calls, nothing to break. Edit this
// file (or delete it) and build whatever you want; see AGENTS.md for the
// full LangSmith API surface and README.md for the render(data, root, metadata)
// bridge contract.
export function App(_props: { data: unknown; metadata?: RenderMetadata }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-surface-level-1 p-6">
      <div className="w-full max-w-md rounded-xl border border-secondary bg-elevated p-8 text-center shadow-md">
        <div className="mx-auto flex size-10 items-center justify-center rounded-full bg-brand-subtle">
          <svg
            viewBox="0 0 24 24"
            fill="none"
            className="size-5 text-brand-primary"
            stroke="currentColor"
            strokeWidth={1.75}
          >
            <path
              d="M13 2 4 14h6l-1 8 9-12h-6l1-8Z"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </div>

        <h1 className="mt-4 text-lg font-semibold text-primary">Your custom app starts here</h1>
        <p className="mt-2 text-sm leading-relaxed text-tertiary">
          A blank canvas — edit <code className="text-secondary">src/App.tsx</code> to build
          whatever you want.
        </p>

        <div className="mt-5 rounded-lg border border-secondary bg-secondary p-3 text-left">
          <p className="text-xs font-medium uppercase tracking-wide text-quaternary">
            Calling the API
          </p>
          <pre className="mt-1.5 overflow-x-auto text-xs leading-relaxed text-secondary">
            {`await window.langsmith.call(
  'GET /api/v1/sessions',
  { params: { limit: '10' } }
);`}
          </pre>
        </div>

        <p className="mt-4 text-xs text-quaternary">
          See <code className="text-tertiary">AGENTS.md</code> for the full API surface.
        </p>
      </div>
    </div>
  );
}
