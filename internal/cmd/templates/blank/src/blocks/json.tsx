import { useState } from 'react';
import { CaretDownIcon, CaretRightIcon } from '@langchain/macaw-components/icons';
import { cn } from '@langchain/macaw-components/utils/cn';

const MAX_ITEMS = 100;
const MAX_STRING = 400;

function Primitive({ value }: { value: unknown }) {
  const [full, setFull] = useState(false);
  if (value === null || value === undefined)
    return <span className="italic text-tertiary">null</span>;
  if (typeof value === 'string') {
    const long = value.length > MAX_STRING;
    const shown = long && !full ? `${value.slice(0, MAX_STRING)}…` : value;
    return (
      <span className="whitespace-pre-wrap text-primary [overflow-wrap:anywhere]">
        "{shown}"
        {long && (
          <button
            type="button"
            className="ml-space-1 text-link hover:text-link-hover"
            onClick={() => setFull(!full)}
          >
            {full ? 'less' : `+${value.length - MAX_STRING} chars`}
          </button>
        )}
      </span>
    );
  }
  return <span className="text-brand-primary">{String(value)}</span>;
}

function summary(value: object): string {
  const count = Array.isArray(value) ? value.length : Object.keys(value).length;
  return Array.isArray(value) ? `[${count}]` : `{${count}}`;
}

function Node({
  name,
  value,
  depth,
  collapsedDepth,
}: {
  name?: string;
  value: unknown;
  depth: number;
  collapsedDepth: number;
}) {
  const [open, setOpen] = useState(depth < collapsedDepth);
  const label = name !== undefined ? <span className="text-secondary">{name}:</span> : null;
  if (value === null || typeof value !== 'object') {
    return (
      <div className="flex min-w-0 gap-space-1 py-px pl-[18px]">
        {label}
        <Primitive value={value} />
      </div>
    );
  }
  const entries: [string, unknown][] = Array.isArray(value)
    ? value.map((v, i) => [String(i), v])
    : Object.entries(value);
  const Caret = open ? CaretDownIcon : CaretRightIcon;
  return (
    <div className="min-w-0">
      <button
        type="button"
        className="flex min-w-0 items-center gap-space-1 rounded-xs py-px text-left hover:bg-surface-level-2-hover"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
      >
        <Caret className="size-3.5 shrink-0 text-tertiary" aria-hidden />
        {label}
        <span className="text-tertiary">{summary(value)}</span>
      </button>
      {open && (
        <div className="ml-[7px] border-l border-muted pl-space-2">
          {entries.slice(0, MAX_ITEMS).map(([key, child]) => (
            <Node
              key={key}
              name={key}
              value={child}
              depth={depth + 1}
              collapsedDepth={collapsedDepth}
            />
          ))}
          {entries.length > MAX_ITEMS && (
            <div className="pl-[18px] text-tertiary">… {entries.length - MAX_ITEMS} more</div>
          )}
        </div>
      )}
    </div>
  );
}

export function JsonTree({
  value,
  collapsedDepth = 2,
  className,
}: {
  value: unknown;
  collapsedDepth?: number;
  className?: string;
}) {
  return (
    <div className={cn('min-w-0 overflow-x-auto font-mono text-xs leading-relaxed', className)}>
      <Node value={value} depth={0} collapsedDepth={collapsedDepth} />
    </div>
  );
}
