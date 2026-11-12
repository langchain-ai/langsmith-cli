import { useEffect, useMemo, useRef, type ReactNode } from 'react';
import { Accordion, AccordionItem } from '@langchain/macaw-components/Accordion';
import { ChatMessage } from '@langchain/macaw-components/ChatMessage';
import { EmptyState } from '@langchain/macaw-components/EmptyState';
import { StatusBadge } from '@langchain/macaw-components/StatusBadge';
import {
  CpuRegularIcon,
  GearRegularIcon,
  WrenchRegularIcon,
} from '@langchain/macaw-components/icons';
import { stringify, truncateText } from './lib/format.ts';
import {
  contentText,
  normalizeMessages,
  type NormalizedMessage,
  type ToolCall,
} from './lib/messages.ts';
import { isRecord } from './lib/path.ts';
import { JsonTree } from './json.tsx';
import { MarkdownText } from './markdown.tsx';
import { useQuery } from './query.tsx';

function parsed(value: unknown): unknown {
  if (typeof value !== 'string') return value;
  const text = value.trim();
  if (!/^[[{]/.test(text)) return value;
  try {
    return JSON.parse(text);
  } catch {
    return value;
  }
}

/** A tool argument, result or other payload: text as text, JSON as a collapsible tree. */
export function Payload({ value }: { value: unknown }) {
  const data = parsed(value);
  if (data == null || data === '') return <span className="text-xs text-tertiary">Empty</span>;
  if (typeof data === 'string')
    return (
      <pre className="m-0 max-h-80 overflow-auto whitespace-pre-wrap font-mono text-xs leading-relaxed text-primary [overflow-wrap:anywhere]">
        {data}
      </pre>
    );
  if (
    Array.isArray(data) &&
    data.every((part) => isRecord(part) && typeof part.text === 'string')
  ) {
    return <Payload value={contentText(data)} />;
  }
  return <JsonTree value={data} collapsedDepth={2} />;
}

function argsPreview(args: unknown): string {
  if (isRecord(args)) {
    const entries = Object.entries(args);
    if (entries.length === 0) return '';
    return truncateText(
      entries.map(([k, v]) => `${k}: ${typeof v === 'string' ? v : stringify(v)}`).join(', '),
      120
    );
  }
  return truncateText(stringify(args ?? ''), 120);
}

function ToolCalls({ calls }: { calls: ToolCall[] }) {
  return (
    <Accordion className="w-full">
      {calls.map((call, index) => (
        <AccordionItem
          key={call.id || index}
          icon={<WrenchRegularIcon aria-hidden />}
          title={<span className="font-mono text-xs">{call.name || 'tool'}</span>}
          badge={
            call.result ? (
              call.result.isError ? (
                <StatusBadge variant="error">Error</StatusBadge>
              ) : null
            ) : (
              <StatusBadge variant="neutral" icon={null}>
                No result
              </StatusBadge>
            )
          }
          description={argsPreview(call.args) || undefined}
          contents={
            <div className="flex flex-col gap-space-3 py-space-1">
              <div className="flex flex-col gap-space-1">
                <span className="text-xxs font-medium uppercase tracking-wide text-tertiary">
                  Arguments
                </span>
                <Payload value={call.args} />
              </div>
              {call.result && (
                <div className="flex flex-col gap-space-1">
                  <span className="text-xxs font-medium uppercase tracking-wide text-tertiary">
                    Result
                  </span>
                  <Payload value={call.result.content} />
                </div>
              )}
            </div>
          }
        />
      ))}
    </Accordion>
  );
}

function Collapsible({
  icon,
  title,
  preview,
  children,
}: {
  icon: ReactNode;
  title: string;
  preview: string;
  children: ReactNode;
}) {
  return (
    <Accordion className="w-full">
      <AccordionItem
        icon={icon}
        title={title}
        description={preview || undefined}
        contents={children}
      />
    </Accordion>
  );
}

const CHAT_ROLE = {
  user: 'user',
  assistant: 'assistant',
  system: 'system',
  tool: 'tool',
  other: 'assistant',
} as const;

type Block = { id: string; type: 'custom'; content: ReactNode };

function messageBlocks(message: NormalizedMessage): Block[] {
  const blocks: Block[] = [];
  if (message.reasoning.trim()) {
    blocks.push({
      id: `${message.key}-reasoning`,
      type: 'custom',
      content: (
        <Collapsible
          icon={<CpuRegularIcon aria-hidden />}
          title="Reasoning"
          preview={truncateText(message.reasoning.replace(/\s+/g, ' '), 140)}
        >
          <MarkdownText content={message.reasoning} className="py-space-1 text-secondary" />
        </Collapsible>
      ),
    });
  }
  if (message.text.trim()) {
    blocks.push({
      id: `${message.key}-text`,
      type: 'custom',
      content:
        message.role === 'user' ? (
          <div className="w-fit max-w-full rounded-xl bg-surface-level-2 px-space-3 py-space-2">
            <MarkdownText content={message.text} />
          </div>
        ) : (
          <MarkdownText content={message.text} />
        ),
    });
  }
  if (message.toolCalls.length > 0)
    blocks.push({
      id: `${message.key}-tools`,
      type: 'custom',
      content: <ToolCalls calls={message.toolCalls} />,
    });
  return blocks;
}

function Standalone({ message }: { message: NormalizedMessage }) {
  if (message.role === 'system') {
    return (
      <Collapsible
        icon={<GearRegularIcon aria-hidden />}
        title={message.name ?? 'System prompt'}
        preview={truncateText(message.text.replace(/\s+/g, ' '), 140)}
      >
        <MarkdownText content={message.text} className="py-space-1" />
      </Collapsible>
    );
  }
  return (
    <Collapsible
      icon={<WrenchRegularIcon aria-hidden />}
      title={`Tool result${message.name ? ` · ${message.name}` : ''}`}
      preview={truncateText(message.text.replace(/\s+/g, ' '), 140)}
    >
      <Payload value={message.content} />
    </Collapsible>
  );
}

/** Consecutive assistant messages (tool-calling steps, then the answer) render as one turn. */
function groupTurns(messages: NormalizedMessage[]): NormalizedMessage[][] {
  const groups: NormalizedMessage[][] = [];
  for (const message of messages) {
    const last = groups[groups.length - 1];
    if (
      last &&
      message.role === 'assistant' &&
      last[0].role === 'assistant' &&
      (message.name ?? '') === (last[0].name ?? '')
    )
      last.push(message);
    else groups.push([message]);
  }
  return groups;
}

function Turn({ group }: { group: NormalizedMessage[] }) {
  const first = group[0];
  if (first.role === 'system' || first.role === 'tool') return <Standalone message={first} />;
  const blocks = group.flatMap(messageBlocks);
  if (blocks.length === 0)
    blocks.push({
      id: `${first.key}-empty`,
      type: 'custom',
      content: <span className="text-xs italic text-tertiary">Empty message</span>,
    });
  const name = first.role === 'assistant' && first.name ? first.name : first.label;
  return (
    <ChatMessage
      message={{
        id: first.key,
        role: CHAT_ROLE[first.role],
        status: 'completed',
        readOnly: true,
        content: blocks,
      }}
      name={name}
      align={first.role === 'user' ? 'end' : 'start'}
    />
  );
}

export function Thread({
  messages,
  emptyText,
}: {
  messages: NormalizedMessage[];
  emptyText?: string | null;
}) {
  if (messages.length === 0) return <EmptyState size="sm" title={emptyText || 'No messages'} />;
  return (
    <div className="flex min-w-0 flex-col gap-space-5">
      {groupTurns(messages).map((group) => (
        <Turn key={group[0].key} group={group} />
      ))}
    </div>
  );
}

/** A conversation: LangChain, OpenAI or Anthropic message shapes, tool calls grouped under their message. */
export function ThreadViewer({ messages, emptyText }: { messages: unknown; emptyText?: string }) {
  const normalized = useMemo(() => normalizeMessages(messages), [messages]);
  return <Thread messages={normalized} emptyText={emptyText} />;
}

// Trajectory pages hold 10 trace scopes by default, oldest first.
const MAX_THREAD_PAGES = 10;

/**
 * A thread's messages through POST /v1/trajectory, oldest first (GET /v2/threads/{id}/messages is SSE-only).
 * Follows `next_cursor` for up to ten pages; `truncated` is true when the thread has more.
 */
export function useThreadMessages({
  projectId,
  threadId,
}: {
  projectId?: string | null;
  threadId?: string | null;
}) {
  const result = useQuery<Record<string, unknown>, unknown>(
    threadId ? 'POST /v1/trajectory' : null,
    {
      body: { project_id: projectId, thread_id: threadId, format: 'messages' },
      enabled: Boolean(threadId && projectId),
      itemsPath: 'messages',
    }
  );
  const { loading, loadingMore, hasMore, error, items, loadMore } = result;
  const pages = useRef(1);
  useEffect(() => {
    if (loading) {
      pages.current = 1;
      return;
    }
    if (hasMore && !loadingMore && !error && pages.current < MAX_THREAD_PAGES) {
      pages.current += 1;
      loadMore();
    }
  }, [loading, loadingMore, hasMore, error, items, loadMore]);
  const messages = useMemo(() => items ?? [], [items]);
  const truncated = !loading && !loadingMore && hasMore && pages.current >= MAX_THREAD_PAGES;
  return { ...result, messages, truncated };
}
