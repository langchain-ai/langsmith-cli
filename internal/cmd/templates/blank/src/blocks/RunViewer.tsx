import { useMemo, useState } from 'react';
import { Banner } from '@langchain/macaw-components/Banner';
import { GroupedTabs } from '@langchain/macaw-components/GroupedTabs';
import { Text } from '@langchain/macaw-components/Text';
import { maybeGetMessages, normalizeMessages } from './lib/messages.ts';
import { isRecord } from './lib/path.ts';
import { JsonTree } from './json.tsx';
import { MarkdownText } from './markdown.tsx';
import { useQuery } from './query.tsx';
import { Payload, Thread } from './ThreadViewer.tsx';

function Fields({ value }: { value: unknown }) {
  if (!isRecord(value)) return <Payload value={value} />;
  const entries = Object.entries(value);
  if (entries.length === 0) return <span className="text-xs text-tertiary">Empty</span>;
  return (
    <div className="flex min-w-0 flex-col gap-space-4">
      {entries.map(([key, field]) => (
        <div key={key} className="flex min-w-0 flex-col gap-space-1">
          <Text as="div" variant="sm" weight="medium" color="secondary">
            {key}
          </Text>
          {typeof field === 'string' ? (
            <MarkdownText content={field || '""'} />
          ) : field == null || typeof field !== 'object' ? (
            <span className="font-mono text-xs text-primary">{String(field)}</span>
          ) : (
            <JsonTree value={field} collapsedDepth={2} />
          )}
        </div>
      ))}
    </div>
  );
}

/** Every field of `value` beside its message `list`, including the siblings of `output.messages`. */
function fieldsBeside(value: Record<string, unknown>, list: unknown[]): Record<string, unknown> {
  if (list[0] === value) return {};
  const rest: Record<string, unknown> = {};
  for (const [key, field] of Object.entries(value)) {
    if (field === list) continue;
    if (isRecord(field) && field.messages === list) {
      const { messages: _messages, ...siblings } = field;
      if (Object.keys(siblings).length > 0) rest[key] = siblings;
    } else {
      rest[key] = field;
    }
  }
  return rest;
}

function IOSection({ title, value }: { title: string; value: unknown }) {
  const [view, setView] = useState<'pretty' | 'json'>('pretty');
  const { messages, rest } = useMemo(() => {
    const list = maybeGetMessages(value);
    if (!list) return { messages: null, rest: null };
    return {
      messages: normalizeMessages(list),
      rest: isRecord(value) ? fieldsBeside(value, list) : null,
    };
  }, [value]);
  const empty = value == null || (isRecord(value) && Object.keys(value).length === 0);
  return (
    <section className="flex min-w-0 flex-col gap-space-3">
      <div className="flex items-center justify-between gap-space-3">
        <span className="text-xxs font-medium uppercase tracking-wide text-tertiary">{title}</span>
        {!empty && (
          <GroupedTabs<'pretty' | 'json'>
            size="xs"
            value={view}
            onChange={setView}
            options={[
              { value: 'pretty', display: 'Pretty' },
              { value: 'json', display: 'JSON' },
            ]}
          />
        )}
      </div>
      {empty ? (
        <span className="text-sm text-tertiary">—</span>
      ) : view === 'json' ? (
        <JsonTree value={value} collapsedDepth={3} />
      ) : messages ? (
        <>
          <Thread messages={messages} />
          {rest && Object.keys(rest).length > 0 && <Fields value={rest} />}
        </>
      ) : (
        <Fields value={value} />
      )}
    </section>
  );
}

type RunLike = { inputs?: unknown; outputs?: unknown; error?: string | null };

/** A run's inputs and outputs: message payloads as a conversation, anything else as fields; errors on top. */
export function RunViewer({ run, inputs, outputs, error }: { run?: RunLike | null } & RunLike) {
  const failure = error ?? run?.error;
  return (
    <div className="flex min-w-0 flex-col gap-space-6">
      {failure && (
        <Banner intent="error" title="Run failed">
          <span className="whitespace-pre-wrap font-mono text-xs [overflow-wrap:anywhere]">
            {failure}
          </span>
        </Banner>
      )}
      <IOSection title="Input" value={inputs ?? run?.inputs} />
      <IOSection title="Output" value={outputs ?? run?.outputs} />
    </div>
  );
}

export const RUN_SELECTS = [
  'ID',
  'NAME',
  'RUN_TYPE',
  'STATUS',
  'START_TIME',
  'END_TIME',
  'LATENCY_SECONDS',
  'ERROR',
  'INPUTS',
  'OUTPUTS',
  'TOTAL_TOKENS',
  'TOTAL_COST',
  'PROJECT_ID',
  'TRACE_ID',
  'THREAD_ID',
];

/** One run by id. Pass the queue item's or list row's `project_id` and `start_time` verbatim. */
export function useRun({
  runId,
  projectId,
  startTime,
  selects = RUN_SELECTS,
}: {
  runId?: string | null;
  projectId?: string | null;
  startTime?: string | null;
  selects?: string[];
}) {
  return useQuery<Record<string, unknown>>(runId ? `GET /v2/runs/${runId}` : null, {
    params: { project_id: projectId ?? undefined, start_time: startTime ?? undefined, selects },
    enabled: Boolean(runId && projectId),
    paging: false,
  });
}
