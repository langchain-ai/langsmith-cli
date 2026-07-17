import type { ReactNode } from 'react';

const SURFACE = 'var(--bg-surface-level-1)';

export function Section({ title, note, children }: { title: string; note?: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3 rounded-lg border border-secondary p-5">
      <div className="flex flex-col gap-0.5">
        <h2 className="text-sm font-semibold text-primary">{title}</h2>
        {note && <span className="text-xs text-tertiary">{note}</span>}
      </div>
      {children}
    </section>
  );
}

export function Empty({ label = 'No data' }: { label?: string }) {
  return <span className="py-4 text-center text-xs text-tertiary">{label}</span>;
}

export function StatTile({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="flex flex-col gap-1 rounded-lg border border-secondary p-4">
      <span className="text-xs font-medium uppercase tracking-wide text-tertiary">{label}</span>
      <span className="text-2xl font-semibold tabular-nums text-primary">{value}</span>
      {hint && <span className="text-xs text-tertiary">{hint}</span>}
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
          {it.label}
        </li>
      ))}
    </ul>
  );
}

export interface BarItem {
  label: string;
  value: number;
  color?: string;
  hint?: string;
}

export function BarList({
  items,
  color = 'var(--brand-400)',
  format = (n: number) => String(n),
  labelWidth = 'w-32',
}: {
  items: BarItem[];
  color?: string;
  format?: (n: number) => string;
  labelWidth?: string;
}) {
  const max = items.reduce((m, x) => Math.max(m, x.value), 0);
  if (items.length === 0) return <Empty />;
  return (
    <ul className="flex flex-col gap-2">
      {items.map((it) => (
        <li key={it.label} className="flex items-center gap-3 text-sm" title={it.hint ?? `${it.label}: ${format(it.value)}`}>
          <span className={`${labelWidth} shrink-0 truncate text-secondary`} title={it.label}>
            {it.label}
          </span>
          <span className="h-2.5 min-w-0 flex-1 overflow-hidden rounded-full bg-secondary">
            <span
              className="block h-full rounded-full"
              style={{
                width: `${max > 0 && it.value > 0 ? Math.max((it.value / max) * 100, 2) : 0}%`,
                backgroundColor: it.color ?? color,
              }}
            />
          </span>
          <span className="w-16 shrink-0 text-right tabular-nums text-tertiary">{format(it.value)}</span>
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

// Horizontal stacked bars sharing one scale; a 2px surface gap separates fills.
export function StackedBar({ rows, format = (n: number) => String(n) }: { rows: StackedRow[]; format?: (n: number) => string }) {
  const max = rows.reduce((m, r) => Math.max(m, r.segments.reduce((s, x) => s + x.value, 0)), 0);
  if (rows.length === 0) return <Empty />;
  return (
    <ul className="flex flex-col gap-2">
      {rows.map((r) => {
        const total = r.segments.reduce((s, x) => s + x.value, 0);
        return (
          <li key={r.label} className="flex items-center gap-3 text-sm">
            <span className="w-28 shrink-0 truncate text-secondary" title={r.label}>
              {r.label}
            </span>
            <span className="flex h-2.5 min-w-0 flex-1 overflow-hidden rounded-full bg-secondary">
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

export interface ColumnDay {
  label: string;
  segments: Segment[];
}

const PLOT_H = 140;

// Stacked activity columns over days; segments stack from the baseline up.
export function ColumnChart({ days, format = (n: number) => String(n) }: { days: ColumnDay[]; format?: (n: number) => string }) {
  const max = days.reduce((m, d) => Math.max(m, d.segments.reduce((s, x) => s + x.value, 0)), 0);
  if (max === 0) return <Empty />;
  return (
    <div className="flex items-end gap-1.5 overflow-x-auto pb-1">
      {days.map((d) => {
        const total = d.segments.reduce((s, x) => s + x.value, 0);
        return (
          <div key={d.label} className="flex min-w-[16px] flex-1 flex-col items-center gap-1">
            <div className="flex w-full flex-col-reverse overflow-hidden rounded-sm" style={{ height: PLOT_H }} title={`${d.label}: ${format(total)}`}>
              {d.segments.map((s) =>
                s.value > 0 ? (
                  <div key={s.key} className="w-full" style={{ height: (s.value / max) * PLOT_H, backgroundColor: s.color }} />
                ) : null
              )}
            </div>
            <span className="text-[10px] text-tertiary">{d.label}</span>
          </div>
        );
      })}
    </div>
  );
}
