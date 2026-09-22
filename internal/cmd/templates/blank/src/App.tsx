import { Card } from '@langchain/macaw-components/Card';
import { Text } from '@langchain/macaw-components/Text';
import { SparkleFillIcon } from '@langchain/macaw-components/icons';

export function App(_props: { data: unknown; metadata?: RenderMetadata }) {
  return (
    <main className="flex min-h-screen items-center justify-center p-space-6">
      <Card
        intent="plain"
        className="flex w-full max-w-md flex-col items-center gap-space-4 border-2 border-default text-center shadow-lg"
      >
        <SparkleFillIcon className="size-8 text-icon-brand" aria-hidden />
        <Text variant="h2">Your custom app starts here</Text>
        <Text variant="sm" color="tertiary">
          Edit <code>src/App.tsx</code> to build your app with Macaw components.
        </Text>
        <Card intent="neutral" className="w-full text-left">
          <Text variant="xs" weight="medium">
            Calling the API
          </Text>
          <pre className="mt-space-2 overflow-x-auto font-mono text-xs text-secondary">
            {`await window.langsmith.call(
  'GET /api/v1/sessions',
  { params: { limit: '10' } }
);`}
          </pre>
        </Card>
        <Text variant="xs" color="tertiary">
          See <code>AGENTS.md</code> for the API and design system guidance.
        </Text>
      </Card>
    </main>
  );
}
