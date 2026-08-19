// A static starting point — no API calls, nothing to break. Edit this
// file (or delete it) and build whatever you want; see AGENTS.md for the
// full LangSmith API surface and README.md for the render(data, root, metadata)
// bridge contract.
//
// The imports below are the LangSmith design system, vendored into
// src/components/langsmith/design-system/. Reach for it before hand-rolling
// UI: `npx shadcn add @langsmith/button` drops a real <Button> (and its npm
// dependencies) in beside these.
import { Badge } from '@/components/langsmith/design-system/components/Badge';
import { Text } from '@/components/langsmith/design-system/components/Text';

export function App(_props: { data: unknown; metadata?: RenderMetadata }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-surface-level-1 p-space-5">
      <div className="flex w-full max-w-md flex-col gap-space-4 rounded-lg border border-default bg-surface-level-2 p-space-6 shadow-sm">
        <div className="flex flex-col gap-space-2">
          <div className="flex items-center gap-space-2">
            <Text variant="h3">Your custom app starts here</Text>
            <Badge color="special" size="xs">
              blank
            </Badge>
          </div>
          <Text variant="sm" color="tertiary">
            A blank canvas — edit <Code>src/App.tsx</Code> to build whatever you want. This card is
            plain design-system tokens and components; nothing here is special.
          </Text>
        </div>

        <div className="flex flex-col gap-space-1 rounded-md border border-subtle bg-surface-level-3 p-space-3">
          <Text variant="xs" color="quaternary" weight="medium" className="uppercase tracking-wide">
            Calling the API
          </Text>
          <pre className="overflow-x-auto text-xs leading-relaxed text-secondary">
            {`await window.langsmith.call(
  'GET /api/v1/sessions',
  { params: { limit: '10' } }
);`}
          </pre>
        </div>

        <div className="flex flex-wrap items-baseline gap-space-2">
          <Text variant="xs" color="quaternary">
            Add a component
          </Text>
          <Code>npx shadcn add @langsmith/button</Code>
        </div>

        <Text variant="xs" color="quaternary">
          See <Code>AGENTS.md</Code> for the design-system rules and the full API surface.
        </Text>
      </div>
    </div>
  );
}

function Code({ children }: { children: string }) {
  return <code className="rounded-xs bg-surface-level-3 px-1 py-0.5 text-secondary">{children}</code>;
}
