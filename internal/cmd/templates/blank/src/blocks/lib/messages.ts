import { stringify } from './format.ts';
import { isRecord } from './path.ts';

export type Role = 'user' | 'assistant' | 'system' | 'tool' | 'other';

export interface ToolResult {
  content: unknown;
  isError: boolean;
}

export interface ToolCall {
  id: string;
  name: string;
  args: unknown;
  result?: ToolResult;
}

export interface NormalizedMessage {
  key: string;
  role: Role;
  label: string;
  name?: string;
  text: string;
  reasoning: string;
  toolCalls: ToolCall[];
  toolCallId?: string;
  content: unknown;
}

const ROLE_LABELS: Record<Role, string> = {
  user: 'User',
  assistant: 'Assistant',
  system: 'System',
  tool: 'Tool',
  other: 'Message',
};

function isSerialized(v: Record<string, unknown>): boolean {
  return (
    typeof v.lc === 'number' &&
    v.type === 'constructor' &&
    Array.isArray(v.id) &&
    isRecord(v.kwargs)
  );
}

export function isMessageLike(value: unknown): boolean {
  if (!isRecord(value)) return false;
  if (isSerialized(value)) return true;
  return ('role' in value || 'type' in value) && ('content' in value || 'tool_calls' in value);
}

function fieldsOf(message: unknown): Record<string, unknown> {
  if (!isRecord(message)) return {};
  return isSerialized(message) ? (message.kwargs as Record<string, unknown>) : message;
}

function rawRole(message: unknown): string {
  if (!isRecord(message)) return '';
  let type: unknown;
  if (isSerialized(message)) {
    const id = message.id as string[];
    type =
      (message.kwargs as Record<string, unknown>).type ||
      (id[id.length - 1] ?? '').replace(/MessageChunk$|Message$/, '');
  } else {
    type = message.role ?? message.type;
  }
  return typeof type === 'string' ? type.toLowerCase().replace(/messagechunk$|message$/, '') : '';
}

export function roleOf(message: unknown): Role {
  const role = rawRole(message);
  if (role === 'human' || role === 'user') return 'user';
  if (role === 'ai' || role === 'assistant' || role === 'model') return 'assistant';
  if (role === 'system' || role === 'developer') return 'system';
  if (role === 'tool' || role === 'function') return 'tool';
  return 'other';
}

function parseMaybeJson(value: unknown): unknown {
  if (typeof value !== 'string') return value;
  const trimmed = value.trim();
  if (!/^[[{]/.test(trimmed)) return value;
  try {
    return JSON.parse(trimmed);
  } catch {
    return value;
  }
}

const HIDDEN_BLOCKS = new Set([
  'tool_use',
  'tool_call',
  'tool_result',
  'thinking',
  'reasoning',
  'redacted_thinking',
]);

/** Visible text of a message's content; tool and reasoning blocks are rendered separately. */
export function contentText(content: unknown): string {
  if (typeof content === 'string') return content;
  if (content == null) return '';
  if (Array.isArray(content)) {
    return content
      .map((part) => {
        if (typeof part === 'string') return part;
        if (!isRecord(part)) return '';
        if (HIDDEN_BLOCKS.has(String(part.type))) return '';
        if (typeof part.text === 'string') return part.text;
        if (part.type === 'image_url' || part.type === 'image') return '[image]';
        return '';
      })
      .filter(Boolean)
      .join('\n\n');
  }
  if (isRecord(content) && typeof content.text === 'string') return content.text;
  return stringify(content, 2);
}

function reasoningText(fields: Record<string, unknown>): string {
  const parts: string[] = [];
  if (Array.isArray(fields.content)) {
    for (const block of fields.content) {
      if (!isRecord(block)) continue;
      if (typeof block.thinking === 'string') parts.push(block.thinking);
      else if (typeof block.reasoning === 'string') parts.push(block.reasoning);
      else if (block.type === 'reasoning' && Array.isArray(block.summary)) {
        for (const s of block.summary)
          if (isRecord(s) && typeof s.text === 'string') parts.push(s.text);
      }
    }
  }
  const extra = fields.additional_kwargs;
  if (isRecord(extra) && typeof extra.reasoning_content === 'string')
    parts.push(extra.reasoning_content);
  return parts.join('\n\n');
}

export function toolCallsOf(message: unknown): ToolCall[] {
  const fields = fieldsOf(message);
  const extra = isRecord(fields.additional_kwargs) ? fields.additional_kwargs : undefined;
  const raw =
    Array.isArray(fields.tool_calls) && fields.tool_calls.length > 0
      ? fields.tool_calls
      : Array.isArray(extra?.tool_calls)
        ? extra?.tool_calls
        : undefined;
  if (raw && raw.length > 0) {
    return raw.filter(isRecord).map((t) => {
      const fn = isRecord(t.function) ? t.function : undefined;
      return {
        id: String(t.id ?? t.call_id ?? ''),
        name: String(t.name ?? fn?.name ?? ''),
        args: parseMaybeJson(t.args ?? fn?.arguments ?? t.input ?? {}),
      };
    });
  }
  if (Array.isArray(fields.content)) {
    return fields.content
      .filter(
        (b): b is Record<string, unknown> =>
          isRecord(b) && (b.type === 'tool_use' || b.type === 'tool_call')
      )
      .map((b) => ({
        id: String(b.id ?? ''),
        name: String(b.name ?? ''),
        args: parseMaybeJson(b.input ?? b.args ?? {}),
      }));
  }
  return [];
}

function toolResultBlocks(
  fields: Record<string, unknown>
): { id: string; content: unknown; isError: boolean }[] {
  if (!Array.isArray(fields.content)) return [];
  return fields.content
    .filter((b): b is Record<string, unknown> => isRecord(b) && b.type === 'tool_result')
    .map((b) => ({
      id: String(b.tool_use_id ?? b.id ?? ''),
      content: b.content,
      isError: b.is_error === true,
    }));
}

/** Finds a message list in run inputs/outputs: the value itself, `messages`, `output.messages`, LLM `generations`, or any array of messages. */
export function maybeGetMessages(data: unknown): unknown[] | null {
  if (data == null) return null;
  if (Array.isArray(data)) {
    if (data.length > 0 && data.every(isMessageLike)) return data;
    if (data.length > 0 && data.every((d) => Array.isArray(d) && d.every(isMessageLike)))
      return data.flat();
    return null;
  }
  if (isMessageLike(data)) return [data];
  if (!isRecord(data)) return null;
  const candidates: unknown[] = [
    data.messages,
    isRecord(data.output) ? data.output.messages : undefined,
    data.output,
    isRecord(data.data) ? data.data.messages : undefined,
  ];
  for (const candidate of candidates) {
    if (Array.isArray(candidate) && candidate.length > 0 && candidate.every(isMessageLike))
      return candidate;
  }
  if (Array.isArray(data.generations)) {
    const messages = data.generations
      .flat()
      .map((g) =>
        isRecord(g)
          ? (g.message ??
            (typeof g.text === 'string' ? { role: 'assistant', content: g.text } : undefined))
          : undefined
      )
      .filter((m) => m !== undefined);
    if (messages.length > 0 && messages.every(isMessageLike)) return messages;
  }
  for (const value of Object.values(data)) {
    if (Array.isArray(value) && value.length > 0 && value.every(isMessageLike)) return value;
  }
  return null;
}

/** Normalizes LangChain (serialized or dict), OpenAI and Anthropic messages, nesting tool results under their calls. */
export function normalizeMessages(input: unknown): NormalizedMessage[] {
  const list = maybeGetMessages(input) ?? [];
  const results = new Map<string, ToolResult>();

  const normalized = list.map(
    (message, index): NormalizedMessage & { resultIds: string[] | null } => {
      const fields = fieldsOf(message);
      const role = roleOf(message);
      const blocks = toolResultBlocks(fields);
      for (const block of blocks)
        if (block.id) results.set(block.id, { content: block.content, isError: block.isError });
      const text = contentText(fields.content);
      const toolCallId = typeof fields.tool_call_id === 'string' ? fields.tool_call_id : undefined;
      if (role === 'tool' && toolCallId) {
        results.set(toolCallId, {
          content: parseMaybeJson(fields.content),
          isError: fields.status === 'error',
        });
      }
      return {
        key: `${index}:${String(fields.id ?? '')}`,
        role,
        label: role === 'other' ? rawRole(message) || ROLE_LABELS.other : ROLE_LABELS[role],
        name: typeof fields.name === 'string' && fields.name ? fields.name : undefined,
        text,
        reasoning: reasoningText(fields),
        toolCalls: role === 'assistant' ? toolCallsOf(message) : [],
        toolCallId,
        content: fields.content,
        resultIds: blocks.length > 0 && !text.trim() ? blocks.map((b) => b.id) : null,
      };
    }
  );

  const attached = new Set<string>();
  for (const message of normalized) {
    for (const call of message.toolCalls) {
      const result = call.id ? results.get(call.id) : undefined;
      if (result) {
        call.result = result;
        attached.add(call.id);
      }
    }
  }
  return normalized
    .filter((m) => {
      if (m.role === 'tool' && m.toolCallId && attached.has(m.toolCallId)) return false;
      return !(m.resultIds && m.resultIds.every((id) => attached.has(id)));
    })
    .map(({ resultIds: _resultIds, ...m }) => m);
}
