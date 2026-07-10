import { useState } from 'react';
import { CornerDownRightIcon, Maximize01Icon, Minimize01Icon } from '@langchain/untitled-ui-icons';
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
import { cn } from '../lib/utils';
import { ErrorBanner } from './ErrorBanner';
import { Spinner } from './Spinner';

interface Props {
  inputs: Record<string, unknown> | null;
  outputs: Record<string, unknown> | null;
  error?: string | null;
  loading?: boolean;
}

// ── Shared card chrome ───────────────────────────────────────────────────────

function RoleBadge({ role }: { role: string }) {
  return (
    <span className="shrink-0 rounded-sm bg-secondary px-1.5 py-0.5 text-xs font-semibold uppercase tracking-wide text-secondary">
      {role}
    </span>
  );
}

// ── Tool call cards, nested beneath an AI message ───────────────────────────

function ToolCallItem({ toolCall }: { toolCall: NormalizedToolCall }) {
  return (
    <div className="ml-4 mt-2 flex items-start gap-2">
      <CornerDownRightIcon className="mt-3 size-4 shrink-0 text-tertiary" />
      <div className="flex-1 rounded-lg border border-secondary bg-primary">
        <div className="flex items-center gap-2 px-4 py-2">
          <RoleBadge role="Tool call" />
          {toolCall.name && (
            <span className="font-mono text-xs font-medium text-primary">{toolCall.name}</span>
          )}
          {toolCall.id && <span className="font-mono text-xs text-quaternary">{toolCall.id}</span>}
        </div>
        <div className="px-4 pb-4">
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
    <div className="rounded-lg border border-secondary bg-primary">
      <div className="flex items-center gap-2 px-4 py-2">
        <RoleBadge role="Tool" />
        {toolName && <span className="font-mono text-xs font-medium text-primary">{toolName}</span>}
        {toolCallId && <span className="font-mono text-xs text-quaternary">{toolCallId}</span>}
      </div>
      <div className="px-4 pb-4">
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
      <div className="rounded-lg border border-secondary bg-primary">
        <div className="flex items-center gap-2 px-4 py-2">
          <RoleBadge role={getDisplayRole(message)} />
        </div>
        <div className="px-4 pb-4">
          {text ? (
            <p className="whitespace-pre-wrap break-words text-sm leading-relaxed text-primary">{text}</p>
          ) : (
            <p className="text-xs italic text-quaternary">Empty</p>
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
      <div className="flex flex-col gap-3">
        {messages.map((msg, i) => (
          <MessageCard key={i} message={msg} />
        ))}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {Object.entries(data).map(([key, value]) => (
        <div key={key} className="flex flex-col gap-1">
          <span className="text-xs font-medium text-secondary">{key}</span>
          {typeof value === 'string' ? (
            <p className="whitespace-pre-wrap break-words text-sm leading-relaxed text-primary">
              {value}
            </p>
          ) : (
            <pre className="whitespace-pre-wrap break-words rounded-lg border border-secondary bg-secondary p-3 text-xs text-primary">
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
        className="flex cursor-pointer items-center justify-between border-b border-secondary bg-secondary px-6 py-2"
        onClick={() => onModeChange(mode === 'collapsed' ? 'expanded' : 'collapsed')}
      >
        <div className="flex min-w-0 flex-1 items-center gap-4">
          <span className="shrink-0 text-xs font-medium uppercase tracking-wide text-tertiary">
            {title}
          </span>
          {loading ? (
            <Spinner size="sm" />
          ) : (
            mode === 'collapsed' &&
            preview && (
              <span className="line-clamp-1 min-w-0 flex-1 text-sm text-quaternary">
                {preview}
              </span>
            )
          )}
        </div>
        {!loading && (
          <div
            className="flex shrink-0 items-center gap-2 opacity-0 transition-opacity group-hover:opacity-100"
            onClick={(e) => e.stopPropagation()}
          >
            {showRawButton && mode !== 'collapsed' && (
              <button
                type="button"
                className={cn(
                  'rounded px-2 py-1 text-xs text-quaternary',
                  mode === 'raw' ? 'bg-tertiary' : 'bg-transparent hover:bg-secondary'
                )}
                onClick={() => onModeChange(mode === 'raw' ? 'expanded' : 'raw')}
              >
                RAW
              </button>
            )}
            <button
              type="button"
              className="rounded p-1 text-quaternary hover:bg-tertiary"
              onClick={() => onModeChange(mode === 'collapsed' ? 'expanded' : 'collapsed')}
            >
              {mode === 'collapsed' ? (
                <Maximize01Icon className="h-3.5 w-3.5" />
              ) : (
                <Minimize01Icon className="h-3.5 w-3.5" />
              )}
            </button>
          </div>
        )}
      </div>

      {/* Content */}
      {!loading && mode !== 'collapsed' && (
        <div className="border-b border-secondary p-4">
          {error && <ErrorBanner error={error} />}
          {!hasData ? (
            <p className="text-sm text-tertiary">—</p>
          ) : mode === 'raw' ? (
            <pre className="whitespace-pre-wrap break-words rounded-lg border border-secondary bg-secondary p-3 text-xs text-primary">
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
