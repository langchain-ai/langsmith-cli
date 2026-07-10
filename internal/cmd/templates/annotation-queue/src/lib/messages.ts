export interface NormalizedToolCall {
  id: string;
  name: string;
  args: unknown;
}

interface SerializedMessage {
  lc: number;
  type: 'constructor';
  id: string[];
  kwargs: Record<string, unknown>;
  [key: string]: unknown;
}

function isSerializedMessage(v: Record<string, unknown>): v is SerializedMessage {
  return (
    typeof v.lc === 'number' &&
    v.type === 'constructor' &&
    Array.isArray(v.id) &&
    typeof v.kwargs === 'object' &&
    v.kwargs !== null
  );
}

export function isMessageLike(value: unknown): boolean {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
  const v = value as Record<string, unknown>;
  if (isSerializedMessage(v)) return true;
  return ('role' in v || 'type' in v) && 'content' in v;
}

/** Extracts the flat field bag (content, tool_calls, name, ...) regardless of message shape. */
export function getMessageFields(message: unknown): Record<string, unknown> {
  if (!message || typeof message !== 'object') return {};
  const v = message as Record<string, unknown>;
  if (isSerializedMessage(v)) return v.kwargs;
  return v;
}

/** Lowercased role, e.g. "human" | "ai" | "system" | "tool". */
export function getMessageRole(message: unknown): string {
  if (!message || typeof message !== 'object') return '';
  const v = message as Record<string, unknown>;
  let type: string | undefined;

  if (isSerializedMessage(v)) {
    const last = v.id[v.id.length - 1] ?? '';
    type = last.split('Message')[0]?.toLowerCase();
    const kwargsType = v.kwargs.type;
    if (typeof kwargsType === 'string' && kwargsType) type = kwargsType;
  } else {
    type = (v.type ?? v.role) as string | undefined;
  }

  if (typeof type !== 'string' || !type) return '';
  type = type.toLowerCase();
  if (type.endsWith('messagechunk')) type = type.slice(0, -'messagechunk'.length);
  if (type === 'developer') return 'system';
  if (type === 'user') return 'human';
  if (type === 'assistant') return 'ai';
  return type;
}

/** Uppercase badge label matching LangSmith's annotation queue chat cards, e.g. "HUMAN", "AI", "TOOL". */
export function getDisplayRole(message: unknown): string {
  const role = getMessageRole(message);
  return role ? role.toUpperCase() : 'UNKNOWN';
}

/** Renders only the visible text blocks of a message's content (tool_use/tool_call blocks render as separate cards). */
export function contentToText(content: unknown): string {
  if (typeof content === 'string') return content;
  if (Array.isArray(content)) {
    return content
      .map((part) => {
        if (typeof part === 'string') return part;
        const p = part as Record<string, unknown>;
        if (p.type === 'tool_use' || p.type === 'tool_call' || p.type === 'tool_result') return '';
        if (typeof p.text === 'string') return p.text;
        if (p.type === 'image_url' || p.type === 'image') return '[image]';
        return '';
      })
      .filter(Boolean)
      .join('\n');
  }
  if (content == null) return '';
  return typeof content === 'object' ? JSON.stringify(content, null, 2) : String(content);
}

function parseMaybeJson(value: unknown): unknown {
  if (typeof value !== 'string') return value;
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
}

/** Extracts tool calls from a message's `tool_calls`/`additional_kwargs.tool_calls` field, or from Anthropic-style tool_use blocks embedded directly in content. */
export function getToolCalls(message: unknown): NormalizedToolCall[] {
  const fields = getMessageFields(message);
  const additionalKwargs = fields.additional_kwargs as Record<string, unknown> | undefined;
  const raw = Array.isArray(fields.tool_calls)
    ? fields.tool_calls
    : Array.isArray(additionalKwargs?.tool_calls)
      ? additionalKwargs?.tool_calls
      : undefined;

  if (raw && raw.length > 0) {
    return raw.map((tc) => {
      const t = tc as Record<string, unknown>;
      const fn = t.function as Record<string, unknown> | undefined;
      const custom = t.custom as Record<string, unknown> | undefined;
      const name = (t.name ?? fn?.name ?? custom?.name ?? '') as string;
      const argsRaw = t.args ?? fn?.arguments ?? custom?.input ?? {};
      return { id: (t.id ?? t.call_id ?? '') as string, name, args: parseMaybeJson(argsRaw) };
    });
  }

  const content = fields.content;
  if (Array.isArray(content)) {
    const blocks = content.filter((p) => {
      const type = (p as Record<string, unknown>)?.type;
      return type === 'tool_use' || type === 'tool_call';
    }) as Record<string, unknown>[];
    if (blocks.length > 0) {
      return blocks.map((b) => ({
        id: (b.id ?? '') as string,
        name: (b.name ?? '') as string,
        args: parseMaybeJson(b.input ?? b.args ?? {}),
      }));
    }
  }

  return [];
}

export function maybeGetMessages(data: unknown): unknown[] | null {
  if (!data) return null;

  if (isMessageLike(data)) return [data];

  if (Array.isArray(data)) {
    if (data.length > 0 && data.every(isMessageLike)) return data;
    return null;
  }

  if (typeof data !== 'object') return null;
  const record = data as Record<string, unknown>;

  const direct = record['messages'];
  if (Array.isArray(direct) && direct.length > 0 && direct.every(isMessageLike)) {
    return direct;
  }

  const outputVal = record['output'];
  if (outputVal && typeof outputVal === 'object' && !Array.isArray(outputVal)) {
    const nested = (outputVal as Record<string, unknown>)['messages'];
    if (Array.isArray(nested) && nested.length > 0 && nested.every(isMessageLike)) {
      return nested;
    }
  }

  for (const key of Object.keys(record)) {
    const val = record[key];
    if (Array.isArray(val) && val.length > 0 && val.every(isMessageLike)) {
      return val;
    }
  }

  return null;
}

export function getCollapsedPreview(data: unknown): string {
  if (!data) return '';

  const messages = maybeGetMessages(data);
  if (messages && messages.length > 0) {
    const last = messages[messages.length - 1];
    return contentToText(getMessageFields(last).content).replace(/\n+/g, ' ').trim().slice(0, 120);
  }

  if (typeof data === 'string') return data;

  if (typeof data === 'object' && !Array.isArray(data)) {
    const record = data as Record<string, unknown>;
    const firstVal = Object.values(record)[0];
    if (typeof firstVal === 'string') return firstVal.slice(0, 120);
    return JSON.stringify(firstVal ?? data).slice(0, 120);
  }

  return JSON.stringify(data).slice(0, 120);
}

// ── YAML-ish pretty-printer, matching the block-style rendering LangSmith uses
// for tool call args / tool results (e.g. multiline strings as `|-` blocks). ──

function scalarToYaml(value: unknown): string {
  if (value === null || value === undefined) return 'null';
  return String(value);
}

function entryLines(key: string, value: unknown, indent: number): string[] {
  const pad = '  '.repeat(indent);
  if (value !== null && typeof value === 'object') {
    if (Array.isArray(value) && value.length === 0) return [`${pad}${key}: []`];
    if (!Array.isArray(value) && Object.keys(value as object).length === 0) {
      return [`${pad}${key}: {}`];
    }
    return [`${pad}${key}:`, ...valueLines(value, indent + 1)];
  }
  if (typeof value === 'string' && value.includes('\n')) {
    const body = value.split('\n').map((line) => `${'  '.repeat(indent + 1)}${line}`);
    return [`${pad}${key}: |-`, ...body];
  }
  return [`${pad}${key}: ${scalarToYaml(value)}`];
}

function valueLines(value: unknown, indent: number): string[] {
  const pad = '  '.repeat(indent);
  if (Array.isArray(value)) {
    if (value.length === 0) return [`${pad}[]`];
    const lines: string[] = [];
    for (const item of value) {
      if (item !== null && typeof item === 'object' && !Array.isArray(item)) {
        const entries = Object.entries(item as Record<string, unknown>);
        if (entries.length === 0) {
          lines.push(`${pad}- {}`);
          continue;
        }
        entries.forEach(([k, v], idx) => {
          const childLines = entryLines(k, v, indent + 1);
          if (idx === 0) {
            lines.push(`${pad}- ${childLines[0].slice((indent + 1) * 2)}`);
            lines.push(...childLines.slice(1));
          } else {
            lines.push(...childLines);
          }
        });
      } else if (Array.isArray(item)) {
        lines.push(`${pad}-`);
        lines.push(...valueLines(item, indent + 1));
      } else {
        lines.push(`${pad}- ${scalarToYaml(item)}`);
      }
    }
    return lines;
  }
  if (value !== null && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>);
    if (entries.length === 0) return [`${pad}{}`];
    return entries.flatMap(([k, v]) => entryLines(k, v, indent));
  }
  return [`${pad}${scalarToYaml(value)}`];
}

export function toYamlish(value: unknown): string {
  if (value === null || value === undefined) return 'null';
  if (typeof value !== 'object') return scalarToYaml(value);
  return valueLines(value, 0).join('\n');
}
