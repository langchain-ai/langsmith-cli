import { Badge } from '@/components/langsmith/design-system/components/Badge';
import { Spinner } from '@/components/langsmith/design-system/components/Spinner';
import { Text } from '@/components/langsmith/design-system/components/Text';
import type { StandardMessage } from '../types';

interface Props {
  messages: StandardMessage[] | undefined;
  threadId: string | undefined;
  loading?: boolean;
}

function contentToText(content: StandardMessage['content']): string {
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) {
    return content == null ? '' : JSON.stringify(content, null, 2);
  }
  const parts: string[] = [];
  for (const block of content) {
    if (typeof block === 'string') {
      parts.push(block);
      continue;
    }
    if (!block || typeof block !== 'object') continue;
    const b = block as Record<string, unknown>;
    if (typeof b.text === 'string') {
      parts.push(b.text);
    } else if (typeof b.reasoning === 'string') {
      parts.push(b.reasoning);
    } else if (typeof b.thinking === 'string') {
      parts.push(b.thinking);
    } else if (b.type === 'tool_call' && typeof b.name === 'string') {
      parts.push(`[tool_call ${b.name}] ${JSON.stringify(b.args ?? {})}`);
    } else {
      parts.push(JSON.stringify(b));
    }
  }
  return parts.filter(Boolean).join('\n');
}

function roleLabel(role: string): string {
  switch (role) {
    case 'human':
      return 'Human';
    case 'ai':
      return 'AI';
    case 'system':
      return 'System';
    case 'tool':
      return 'Tool';
    default:
      return role || 'Unknown';
  }
}

function bubbleClass(role: string): string {
  switch (role) {
    case 'human':
      return 'ml-8 border-brand bg-brand-muted';
    case 'ai':
      return 'mr-8 border-default bg-surface-level-2';
    case 'system':
      return 'border-dashed border-default bg-transparent';
    case 'tool':
      return 'mr-4 border-default bg-surface-level-1';
    default:
      return 'border-default bg-surface-level-1';
  }
}

/** Chronological chat view for THREAD queue items (from POST /v1/trajectory). */
export function ThreadViewer({ messages, threadId, loading }: Props) {
  if (loading && (!messages || messages.length === 0)) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <Spinner size="sm" className="text-icon-tertiary" />
      </div>
    );
  }

  if (!messages || messages.length === 0) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-space-1 p-space-5">
        <Text variant="md" color="tertiary">
          No messages in this thread
        </Text>
        {threadId && (
          <Text variant="sm" color="quaternary" className="font-mono">
            {threadId}
          </Text>
        )}
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col gap-space-3 overflow-auto p-space-4">
      <Text variant="xs" weight="medium" color="tertiary" className="uppercase tracking-wide">
        {`Thread · ${messages.length} message${messages.length === 1 ? '' : 's'}`}
      </Text>
      {messages.map((msg, index) => {
        const text = contentToText(msg.content);
        if (!text.trim()) return null;
        return (
          <div
            key={msg.id ?? `${msg.role}-${index}`}
            className={`flex flex-col items-start gap-space-1 rounded-lg border p-space-3 ${bubbleClass(msg.role)}`}
          >
            <Badge size="xxs" rounded="xs" className="uppercase">
              {roleLabel(msg.role) + (msg.name ? ` · ${msg.name}` : '')}
            </Badge>
            <pre className="w-full whitespace-pre-wrap break-words font-sans text-sm text-primary">
              {text}
            </pre>
          </div>
        );
      })}
    </div>
  );
}
