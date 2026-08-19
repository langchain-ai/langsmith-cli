import type { ReactNode } from 'react';
import { Badge } from '@/components/langsmith/design-system/components/Badge';
import { Text } from '@/components/langsmith/design-system/components/Text';

const SURFACE = 'var(--bg-surface-level-1)';

export function Section({ title, note, children }: { title: string; note?: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-space-3 rounded-lg border border-default p-space-5">
      <div className="flex flex-col gap-space-1">
        <Text variant="md" weight="semibold" as="h2">
          {title}
        </Text>
        {note && (
          <Text variant="sm" color="tertiary">
            {note}
          </Text>
        )}
      </div>
      {children}
    </section>
  );
}

export function Empty({ label = 'No data' }: { label?: string }) {
  return (
    <Text variant="sm" color="tertiary" className="py-space-4 text-center">
      {label}
    </Text>
  );
}

export interface StatDelta {
  text: string;
  tone: 'good' | 'bad' | 'neutral';
}

// The delta pill's color is a verdict (did it beat the baseline?), which maps
// straight onto the Badge intent colors.
const DELTA_TONE_COLOR: Record<StatDelta['tone'], 'success' | 'error' | 'secondary'> = {
  good: 'success',
  bad: 'error',
  neutral: 'secondary',
};

// Value uses proportional figures (not tabular-nums) — this is a standalone
// hero/stat-tile number, not a column that needs to align with others.
export function StatTile({
  label,
  value,
  hint,
  delta,
  accent,
}: {
  label: string;
  value: string;
  hint?: string;
  delta?: StatDelta;
  accent?: string;
}) {
  return (
    <div className="flex flex-col gap-1.5 rounded-lg border border-default p-space-4">
      <span className="inline-flex items-center gap-1.5">
        {accent && <span className="size-2 shrink-0 rounded-xs" style={{ backgroundColor: accent }} />}
        <Text
          as="span"
          variant="xs"
          weight="medium"
          color="tertiary"
          className="truncate uppercase tracking-wide"
          title={label}
        >
          {label}
        </Text>
      </span>
      <span className="text-2xl font-semibold text-primary">{value}</span>
      {(delta || hint) && (
        <div className="flex flex-wrap items-center gap-1.5">
          {delta && (
            <Badge size="xs" color={DELTA_TONE_COLOR[delta.tone]}>
              {delta.text}
            </Badge>
          )}
          {hint && (
            <Text as="span" variant="sm" color="tertiary">
              {hint}
            </Text>
          )}
        </div>
      )}
    </div>
  );
}

export interface LegendItem {
  label: string;
  color: string;
}

export function Legend({ items }: { items: LegendItem[] }) {
  return (
    <ul className="flex flex-wrap items-center gap-x-4 gap-y-1">
      {items.map((it) => (
        <li key={it.label} className="flex items-center gap-1.5 text-xs text-secondary">
          <span className="size-2.5 shrink-0 rounded-sm" style={{ backgroundColor: it.color }} />
          <span className="truncate max-w-[180px]" title={it.label}>{it.label}</span>
        </li>
      ))}
    </ul>
  );
}

export interface BarItem {
  key: string;
  label: string;
  value: number | null; // null → missing: empty track, em-dash value
  display: string;
  color: string;
  hint?: string;
}

// One bar per item sharing a scale. A null value renders no fill (never a
// fake sliver) so "missing" never reads as "zero".
export function BarList({ items, labelWidth = 'w-32' }: { items: BarItem[]; labelWidth?: string }) {
  if (items.length === 0) return <Empty />;
  const max = items.reduce((m, x) => Math.max(m, x.value ?? 0), 0);
  return (
    <ul className="flex flex-col gap-space-2">
      {items.map((it) => (
        <li key={it.key} className="flex items-center gap-3 text-sm" title={it.hint ?? `${it.label}: ${it.display}`}>
          <span className={`${labelWidth} shrink-0 truncate text-secondary`} title={it.label}>
            {it.label}
          </span>
          <span className="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-surface-level-3">
            {it.value != null && (
              <span
                className="block h-full rounded-full motion-safe:transition-[width] motion-safe:duration-slow"
                style={{
                  width: `${max > 0 ? Math.max((it.value / max) * 100, it.value > 0 ? 3 : 0) : 0}%`,
                  backgroundColor: it.color,
                }}
              />
            )}
          </span>
          <span className="w-20 shrink-0 text-right tabular-nums text-tertiary">{it.display}</span>
        </li>
      ))}
    </ul>
  );
}

export interface Segment {
  key: string;
  value: number;
  color: string;
}

export interface StackedRow {
  label: string;
  segments: Segment[];
}

// Horizontal stacked bars sharing one scale; a 2px surface-color gap (border-r)
// separates each segment instead of an outline.
export function StackedBar({ rows, format = (n: number) => String(n) }: { rows: StackedRow[]; format?: (n: number) => string }) {
  if (rows.length === 0) return <Empty />;
  const max = rows.reduce((m, r) => Math.max(m, r.segments.reduce((s, x) => s + x.value, 0)), 0);
  return (
    <ul className="flex flex-col gap-space-2">
      {rows.map((r) => {
        const total = r.segments.reduce((s, x) => s + x.value, 0);
        return (
          <li key={r.label} className="flex items-center gap-3 text-sm">
            <span className="w-28 shrink-0 truncate text-secondary" title={r.label}>
              {r.label}
            </span>
            <span className="flex h-2.5 min-w-0 flex-1 overflow-hidden rounded-full bg-surface-level-3">
              {r.segments.map((s) =>
                s.value > 0 ? (
                  <span
                    key={s.key}
                    title={`${s.key}: ${format(s.value)}`}
                    className="block h-full border-r-2 first:rounded-l-full last:rounded-r-full last:border-r-0"
                    style={{ width: `${max > 0 ? (s.value / max) * 100 : 0}%`, backgroundColor: s.color, borderColor: SURFACE }}
                  />
                ) : null
              )}
            </span>
            <span className="w-16 shrink-0 text-right tabular-nums text-tertiary">{format(total)}</span>
          </li>
        );
      })}
    </ul>
  );
}
