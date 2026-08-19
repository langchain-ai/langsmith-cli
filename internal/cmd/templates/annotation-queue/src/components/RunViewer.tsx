import { useState } from 'react';
import { CornerDownRightIcon, Maximize01Icon, Minimize01Icon } from '@langchain/untitled-ui-icons';
import { Badge } from '@/components/langsmith/design-system/components/Badge';
import { Banner } from '@/components/langsmith/design-system/components/Banner';
import { Button } from '@/components/langsmith/design-system/components/Button';
import { IconButton } from '@/components/langsmith/design-system/components/IconButton';
import { Spinner } from '@/components/langsmith/design-system/components/Spinner';
import { Text } from '@/components/langsmith/design-system/components/Text';
import type { IOMode } from '../types';
import {
  maybeGetMessages,
  contentToText,
  getDisplayRole,
  getMessageFields,
  getMessageRole,
  getToolCalls,
  getCollapsedPreview,
  toYamlish,
  type NormalizedToolCall,
} from '../lib/messages';

interface Props {
  inputs: Record<string, unknown> | null;
  outputs: Record<string, unknown> | null;
  error?: string | null;
  loading?: boolean;
}

// ── Tool call cards, nested beneath an AI message ───────────────────────────

function ToolCallItem({ toolCall }: { toolCall: NormalizedToolCall }) {
  return (
    <div className="ml-space-4 mt-space-2 flex items-start gap-space-2">
      <CornerDownRightIcon className="mt-3 size-4 shrink-0 text-icon-tertiary" />
      <div className="flex-1 rounded-lg border border-default bg-surface-level-1">
        <div className="flex items-center gap-space-2 px-space-4 py-space-2">
          <Badge size="xxs" rounded="xs" className="uppercase">
            Tool call
          </Badge>
          {toolCall.name && (
            <Text as="span" variant="sm" weight="medium" className="font-mono">
              {toolCall.name}
            </Text>
          )}
          {toolCall.id && (
            <Text as="span" variant="sm" color="quaternary" className="font-mono">
              {toolCall.id}
            </Text>
          )}
        </div>
        <div className="px-space-4 pb-space-4">
          <pre className="whitespace-pre-wrap break-words font-mono text-xs text-secondary">
            {toYamlish(toolCall.args)}
          </pre>
        </div>
      </div>
    </div>
  );
}

function ToolCallsSection({ toolCalls }: { toolCalls: NormalizedToolCall[] }) {
  if (toolCalls.length === 0) return null;
  return (
    <div className="flex flex-col">
      {toolCalls.map((tc, i) => (
        <ToolCallItem key={i} toolCall={tc} />
      ))}
    </div>
  );
}

// ── Tool result message ──────────────────────────────────────────────────────

function ToolMessageCard({ message }: { message: unknown }) {
  const fields = getMessageFields(message);
  const toolCallId = (fields.tool_call_id ?? (fields.additional_kwargs as Record<string, unknown> | undefined)?.tool_call_id) as
    | string
    | undefined;
  const toolName = typeof fields.name === 'string' ? fields.name : '';
  const content = fields.content;
  const parsedContent = (() => {
    try {
      return typeof content === 'string' ? JSON.parse(content) : content;
    } catch {
      return content;
    }
  })();

  return (
    <div className="rounded-lg border border-default bg-surface-level-1">
      <div className="flex items-center gap-space-2 px-space-4 py-space-2">
        <Badge size="xxs" rounded="xs" className="uppercase">
          Tool
        </Badge>
        {toolName && (
          <Text as="span" variant="sm" weight="medium" className="font-mono">
            {toolName}
          </Text>
        )}
        {toolCallId && (
          <Text as="span" variant="sm" color="quaternary" className="font-mono">
            {toolCallId}
          </Text>
        )}
      </div>
      <div className="px-space-4 pb-space-4">
        <pre className="whitespace-pre-wrap break-words font-mono text-xs text-secondary">
          {typeof parsedContent === 'string' ? parsedContent : toYamlish(parsedContent)}
        </pre>
      </div>
    </div>
  );
}

// ── Single message card (human / ai / system) ────────────────────────────────

function MessageCard({ message }: { message: unknown }) {
  const role = getMessageRole(message);

  if (role === 'tool') {
    return <ToolMessageCard message={message} />;
  }

  const fields = getMessageFields(message);
  const text = contentToText(fields.content);
  const toolCalls = role === 'ai' ? getToolCalls(message) : [];

  return (
    <div className="flex flex-col">
      <div className="rounded-lg border border-default bg-surface-level-1">
        <div className="flex items-center gap-space-2 px-space-4 py-space-2">
          <Badge size="xxs" rounded="xs" className="uppercase">
            {getDisplayRole(message)}
          </Badge>
        </div>
        <div className="px-space-4 pb-space-4">
          {text ? (
            <Text variant="body" className="whitespace-pre-wrap break-words leading-relaxed">
              {text}
            </Text>
          ) : (
            <Text variant="sm" color="quaternary" className="italic">
              Empty
            </Text>
          )}
        </div>
      </div>
      <ToolCallsSection toolCalls={toolCalls} />
    </div>
  );
}

// ── Data renderer ────────────────────────────────────────────────────────────

function DataView({ data }: { data: Record<string, unknown> }) {
  const messages = maybeGetMessages(data);

  if (messages && messages.length > 0) {
    return (
      <div className="flex flex-col gap-space-3">
        {messages.map((msg, i) => (
          <MessageCard key={i} message={msg} />
        ))}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-space-4">
      {Object.entries(data).map(([key, value]) => (
        <div key={key} className="flex flex-col gap-space-1">
          <Text variant="sm" weight="medium" color="secondary">
            {key}
          </Text>
          {typeof value === 'string' ? (
            <Text variant="body" className="whitespace-pre-wrap break-words leading-relaxed">
              {value}
            </Text>
          ) : (
            <pre className="whitespace-pre-wrap break-words rounded-lg border border-default bg-surface-level-2 p-space-3 text-xs text-primary">
              {JSON.stringify(value, null, 2)}
            </pre>
          )}
        </div>
      ))}
    </div>
  );
}

// ── CollapsibleSection ───────────────────────────────────────────────────────

function CollapsibleSection({
  title,
  data,
  mode,
  onModeChange,
  error,
  loading,
  showRawButton = true,
}: {
  title: string;
  data: Record<string, unknown> | null;
  mode: IOMode;
  onModeChange: (m: IOMode) => void;
  error?: string | null;
  loading?: boolean;
  showRawButton?: boolean;
}) {
  const hasData = data && Object.keys(data).length > 0;

  const preview = mode === 'collapsed' && hasData ? getCollapsedPreview(data) : '';

  return (
    <div className="group flex flex-col">
      {/* Header */}
      <div
        className="flex cursor-pointer items-center justify-between border-b border-default bg-surface-level-2 px-space-5 py-space-2"
        onClick={() => onModeChange(mode === 'collapsed' ? 'expanded' : 'collapsed')}
      >
        <div className="flex min-w-0 flex-1 items-center gap-space-4">
          <Text
            as="span"
            variant="xs"
            weight="medium"
            color="tertiary"
            className="shrink-0 uppercase tracking-wide"
          >
            {title}
          </Text>
          {loading ? (
            <Spinner size="xs" className="text-icon-tertiary" />
          ) : (
            mode === 'collapsed' &&
            preview && (
              <Text as="span" variant="md" color="quaternary" className="line-clamp-1 min-w-0 flex-1">
                {preview}
              </Text>
            )
          )}
        </div>
        {!loading && (
          <div
            className="flex shrink-0 items-center gap-space-2 opacity-0 transition-opacity duration-fast group-hover:opacity-100"
            onClick={(e) => e.stopPropagation()}
          >
            {showRawButton && mode !== 'collapsed' && (
              <Button
                size="xs"
                color="secondary"
                variant={mode === 'raw' ? 'outlined' : 'plain'}
                onClick={() => onModeChange(mode === 'raw' ? 'expanded' : 'raw')}
              >
                RAW
              </Button>
            )}
            <IconButton
              size="xs"
              color="secondary"
              variant="plain"
              icon={mode === 'collapsed' ? Maximize01Icon : Minimize01Icon}
              label={mode === 'collapsed' ? `Expand ${title.toLowerCase()}` : `Collapse ${title.toLowerCase()}`}
              onClick={() => onModeChange(mode === 'collapsed' ? 'expanded' : 'collapsed')}
            />
          </div>
        )}
      </div>

      {/* Content */}
      {!loading && mode !== 'collapsed' && (
        <div className="flex flex-col gap-space-3 border-b border-default p-space-4">
          {error && (
            <Banner intent="error" title="Error">
              {error}
            </Banner>
          )}
          {!hasData ? (
            <Text variant="md" color="tertiary">
              —
            </Text>
          ) : mode === 'raw' ? (
            <pre className="whitespace-pre-wrap break-words rounded-lg border border-default bg-surface-level-2 p-space-3 text-xs text-primary">
              {JSON.stringify(data, null, 2)}
            </pre>
          ) : (
            <DataView data={data} />
          )}
        </div>
      )}
    </div>
  );
}

// ── RunViewer ────────────────────────────────────────────────────────────────

export function RunViewer({ inputs, outputs, error, loading }: Props) {
  const [inputMode, setInputMode] = useState<IOMode>('expanded');
  const [outputMode, setOutputMode] = useState<IOMode>('expanded');

  return (
    <div className="flex h-full flex-col">
      <CollapsibleSection
        title="Inputs"
        data={inputs}
        mode={inputMode}
        onModeChange={setInputMode}
        loading={loading}
      />
      <CollapsibleSection
        title="Outputs"
        data={outputs}
        mode={outputMode}
        onModeChange={setOutputMode}
        error={error}
        loading={loading}
      />
    </div>
  );
}
