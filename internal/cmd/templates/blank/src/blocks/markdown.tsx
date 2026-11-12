import { Fragment, type ReactNode } from 'react';
import { cn } from '@langchain/macaw-components/utils/cn';
import { CodeBlock } from './code.tsx';

type Block =
  | { kind: 'heading'; level: number; text: string }
  | { kind: 'paragraph'; text: string }
  | { kind: 'code'; lang: string; text: string }
  | { kind: 'quote'; text: string }
  | { kind: 'list'; ordered: boolean; start: number; items: string[] }
  | { kind: 'table'; header: string[]; align: ('left' | 'right' | 'center')[]; rows: string[][] }
  | { kind: 'hr' };

const FENCE = /^\s*(`{3,}|~{3,})\s*([\w+-]*)/;
const HEADING = /^\s{0,3}(#{1,6})\s+(.*?)\s*#*\s*$/;
const HR = /^\s{0,3}([-*_])(\s*\1){2,}\s*$/;
const QUOTE = /^\s{0,3}>\s?/;
const ITEM = /^(\s*)([-*+]|\d{1,9}[.)])\s+(.*)$/;
const TABLE_RULE = /^\s*\|?\s*:?-{1,}:?\s*(\|\s*:?-{1,}:?\s*)*\|?\s*$/;

const splitRow = (line: string) =>
  line
    .trim()
    .replace(/^\||\|$/g, '')
    .split(/(?<!\\)\|/)
    .map((cell) => cell.trim().replace(/\\\|/g, '|'));

function startsBlock(line: string, next?: string): boolean {
  return (
    FENCE.test(line) ||
    HEADING.test(line) ||
    HR.test(line) ||
    QUOTE.test(line) ||
    ITEM.test(line) ||
    (line.includes('|') && next !== undefined && TABLE_RULE.test(next))
  );
}

export function parseBlocks(source: string): Block[] {
  const lines = source.replace(/\r\n?/g, '\n').split('\n');
  const blocks: Block[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (!line.trim()) {
      i++;
      continue;
    }
    const fence = FENCE.exec(line);
    if (fence) {
      const body: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith(fence[1])) body.push(lines[i++]);
      i++;
      blocks.push({ kind: 'code', lang: fence[2], text: body.join('\n') });
      continue;
    }
    const heading = HEADING.exec(line);
    if (heading) {
      blocks.push({ kind: 'heading', level: heading[1].length, text: heading[2] });
      i++;
      continue;
    }
    if (HR.test(line)) {
      blocks.push({ kind: 'hr' });
      i++;
      continue;
    }
    if (QUOTE.test(line)) {
      const body: string[] = [];
      while (i < lines.length && lines[i].trim() && QUOTE.test(lines[i]))
        body.push(lines[i++].replace(QUOTE, ''));
      blocks.push({ kind: 'quote', text: body.join('\n') });
      continue;
    }
    if (line.includes('|') && i + 1 < lines.length && TABLE_RULE.test(lines[i + 1])) {
      const header = splitRow(line);
      const align = splitRow(lines[i + 1]).map((cell) =>
        cell.startsWith(':') && cell.endsWith(':')
          ? 'center'
          : cell.endsWith(':')
            ? 'right'
            : 'left'
      );
      const rows: string[][] = [];
      i += 2;
      while (i < lines.length && lines[i].includes('|') && lines[i].trim())
        rows.push(splitRow(lines[i++]));
      blocks.push({ kind: 'table', header, align, rows });
      continue;
    }
    const item = ITEM.exec(line);
    if (item) {
      const ordered = /\d/.test(item[2]);
      const indent = item[1].length;
      const items: string[] = [];
      while (i < lines.length) {
        const current = ITEM.exec(lines[i]);
        if (current && current[1].length <= indent && /\d/.test(current[2]) === ordered) {
          items.push(current[3]);
          i++;
        } else if (
          lines[i].trim() &&
          (/^\s/.test(lines[i]) || !startsBlock(lines[i], lines[i + 1])) &&
          items.length > 0
        ) {
          items[items.length - 1] +=
            `\n${lines[i].slice(Math.min(lines[i].search(/\S/), indent + 2))}`;
          i++;
        } else if (
          !lines[i].trim() &&
          i + 1 < lines.length &&
          /^\s+\S/.test(lines[i + 1]) &&
          items.length > 0
        ) {
          items[items.length - 1] += '\n';
          i++;
        } else {
          break;
        }
      }
      blocks.push({
        kind: 'list',
        ordered,
        start: ordered ? Number.parseInt(item[2], 10) : 1,
        items,
      });
      continue;
    }
    const body: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() &&
      (body.length === 0 || !startsBlock(lines[i], lines[i + 1]))
    )
      body.push(lines[i++]);
    blocks.push({ kind: 'paragraph', text: body.join('\n') });
  }
  return blocks;
}

const SAFE_URL = /^(https?:\/\/|mailto:|\/)/i;

export function openLink(url: string) {
  if (!SAFE_URL.test(url)) return;
  window.langsmith?.openUrl?.(url, /^https?:\/\//i.test(url) ? { newTab: true } : undefined);
}

function LinkText({ href, children }: { href: string; children: ReactNode }) {
  if (!SAFE_URL.test(href)) return <>{children}</>;
  return (
    <a
      href={href}
      className="text-link underline decoration-1 underline-offset-2 hover:text-link-hover"
      onClick={(event) => {
        event.preventDefault();
        openLink(href);
      }}
    >
      {children}
    </a>
  );
}

const INLINE =
  /(`+)([^`]|[^`][\s\S]*?[^`])\1(?!`)|\[([^\]]+)\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)|(https?:\/\/[^\s<>]*[^\s<>.,;:!?'")\]*_])|\*\*(?=\S)([\s\S]*?\S)\*\*|__(?=\S)([\s\S]*?\S)__|\*(?=[^\s*])([\s\S]*?[^\s*])\*|_(?=[^\s_])([\s\S]*?[^\s_])_|~~(?=\S)([\s\S]*?\S)~~/g;

export function renderInline(text: string, keyPrefix = 'i'): ReactNode[] {
  const out: ReactNode[] = [];
  let last = 0;
  let index = 0;
  const pushText = (value: string) => {
    const parts = value.split('\n');
    parts.forEach((part, i) => {
      if (i > 0) out.push(<br key={`${keyPrefix}-br-${index++}`} />);
      if (part) out.push(part);
    });
  };
  const pattern = new RegExp(INLINE.source, 'g');
  for (let match = pattern.exec(text); match; match = pattern.exec(text)) {
    const before = match.index > 0 ? text[match.index - 1] : '';
    const after = text[match.index + match[0].length] ?? '';
    if (match[9] !== undefined && (/\w/.test(before) || /\w/.test(after))) continue;
    pushText(text.slice(last, match.index));
    const key = `${keyPrefix}-${index++}`;
    if (match[2] !== undefined) {
      out.push(
        <code
          key={key}
          className="rounded-xs bg-surface-level-3 px-1 py-px font-mono text-[0.85em]"
        >
          {match[2].replace(/^ (.*) $/, '$1')}
        </code>
      );
    } else if (match[3] !== undefined) {
      out.push(
        <LinkText key={key} href={match[4]}>
          {renderInline(match[3], key)}
        </LinkText>
      );
    } else if (match[5] !== undefined) {
      out.push(
        <LinkText key={key} href={match[5]}>
          {match[5]}
        </LinkText>
      );
    } else if (match[6] !== undefined || match[7] !== undefined) {
      out.push(
        <strong key={key} className="font-semibold">
          {renderInline(match[6] ?? match[7], key)}
        </strong>
      );
    } else if (match[8] !== undefined || match[9] !== undefined) {
      out.push(<em key={key}>{renderInline(match[8] ?? match[9], key)}</em>);
    } else if (match[10] !== undefined) {
      out.push(<del key={key}>{renderInline(match[10], key)}</del>);
    }
    last = match.index + match[0].length;
  }
  pushText(text.slice(last));
  return out;
}

const HEADING_CLASSES = [
  'text-lg font-semibold',
  'text-base font-semibold',
  'text-sm font-semibold',
  'text-sm font-semibold',
  'text-sm font-medium',
  'text-sm font-medium',
];

function renderBlocks(blocks: Block[], prefix: string): ReactNode[] {
  return blocks.map((block, index) => {
    const key = `${prefix}${index}`;
    switch (block.kind) {
      case 'heading':
        return (
          <div
            key={key}
            role="heading"
            aria-level={block.level}
            className={cn(
              'mt-space-2 leading-snug text-primary first:mt-0',
              HEADING_CLASSES[block.level - 1]
            )}
          >
            {renderInline(block.text, key)}
          </div>
        );
      case 'paragraph':
        return (
          <p key={key} className="m-0">
            {renderInline(block.text, key)}
          </p>
        );
      case 'code':
        return <CodeBlock key={key} code={block.text} language={block.lang} />;
      case 'quote':
        return (
          <blockquote
            key={key}
            className="m-0 flex flex-col gap-space-2 border-l-2 border-default pl-space-3 text-secondary"
          >
            {renderBlocks(parseBlocks(block.text), `${key}-`)}
          </blockquote>
        );
      case 'hr':
        return <hr key={key} className="border-0 border-t border-subtle" />;
      case 'table':
        return (
          <div key={key} className="max-w-full overflow-x-auto rounded-md border border-subtle">
            <table className="w-full border-collapse text-xs">
              <thead className="bg-surface-level-2">
                <tr>
                  {block.header.map((cell, c) => (
                    <th
                      key={c}
                      className="border-b border-subtle px-space-2 py-space-1 font-medium text-secondary"
                      style={{ textAlign: block.align[c] ?? 'left' }}
                    >
                      {renderInline(cell, `${key}-h${c}`)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {block.rows.map((row, r) => (
                  <tr key={r} className="border-t border-muted first:border-t-0">
                    {block.header.map((_, c) => (
                      <td
                        key={c}
                        className="px-space-2 py-space-1 align-top"
                        style={{ textAlign: block.align[c] ?? 'left' }}
                      >
                        {renderInline(row[c] ?? '', `${key}-${r}-${c}`)}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        );
      case 'list': {
        const ListTag = block.ordered ? 'ol' : 'ul';
        return (
          <ListTag
            key={key}
            start={block.ordered && block.start !== 1 ? block.start : undefined}
            className={cn(
              'm-0 flex flex-col gap-space-1 pl-space-5',
              block.ordered ? 'list-decimal' : 'list-disc'
            )}
          >
            {block.items.map((item, itemIndex) => {
              const inner = parseBlocks(item);
              const simple = inner.length === 1 && inner[0].kind === 'paragraph';
              return (
                <li key={itemIndex} className="pl-space-1 marker:text-tertiary">
                  {simple ? (
                    renderInline((inner[0] as { text: string }).text, `${key}-${itemIndex}`)
                  ) : (
                    <div className="flex flex-col gap-space-2">
                      {renderBlocks(inner, `${key}-${itemIndex}-`)}
                    </div>
                  )}
                </li>
              );
            })}
          </ListTag>
        );
      }
      default:
        return <Fragment key={key} />;
    }
  });
}

export function MarkdownText({
  content,
  className,
}: {
  content: string | null | undefined;
  className?: string;
}) {
  if (!content) return null;
  return (
    <div
      className={cn(
        'flex min-w-0 flex-col gap-space-3 text-sm leading-relaxed text-primary [overflow-wrap:anywhere]',
        className
      )}
    >
      {renderBlocks(parseBlocks(content), 'b')}
    </div>
  );
}
