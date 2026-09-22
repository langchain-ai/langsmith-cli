import { Card } from '@langchain/macaw-components/Card';
import { Text } from '@langchain/macaw-components/Text';
import { EmptyState } from '@langchain/macaw-components/EmptyState';
import type { StandardMessage } from '../types';
import { Spinner } from '@langchain/macaw-components/Spinner';

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

/** Chronological chat view for THREAD queue items (from POST /v1/trajectory). */
export function ThreadViewer({ messages, threadId, loading }: Props) {
  if (loading && (!messages || messages.length === 0)) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <Spinner size="md" />
      </div>
    );
  }

  if (!messages || messages.length === 0) {
    return <EmptyState title="No messages in this thread" description={threadId} size="sm" />;
  }

  return (
    <div className="flex flex-1 flex-col gap-3 overflow-auto p-4">
      <div className="text-xs font-medium uppercase tracking-wide text-tertiary">
        Thread · {messages.length} message{messages.length === 1 ? '' : 's'}
      </div>
      {messages.map((msg, index) => {
        const text = contentToText(msg.content);
        if (!text.trim()) return null;
        return (
          <Card
            key={msg.id ?? `${msg.role}-${index}`}
            intent={msg.role === 'human' || msg.role === 'user' ? 'info' : 'neutral'}
            className="flex flex-col gap-space-2"
          >
            <Text variant="xs" color="tertiary" weight="medium">
              {roleLabel(msg.role)}
              {msg.name ? ` · ${msg.name}` : ''}
            </Text>
            <Text variant="sm" className="whitespace-pre-wrap break-words">
              {text}
            </Text>
          </Card>
        );
      })}
    </div>
  );
}
