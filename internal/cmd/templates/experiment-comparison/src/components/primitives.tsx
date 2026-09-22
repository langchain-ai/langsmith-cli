import { BarChart } from '@langchain/macaw-components/BarChart';
import { Badge } from '@langchain/macaw-components/Badge';
import { Card } from '@langchain/macaw-components/Card';
import { Text } from '@langchain/macaw-components/Text';
import { EmptyState } from '@langchain/macaw-components/EmptyState';
import type { ReactNode } from 'react';

export function Section({
  title,
  note,
  children,
}: {
  title: string;
  note?: string;
  children: ReactNode;
}) {
  return (
    <Card className="flex flex-col gap-space-3">
      <div className="flex flex-col gap-0.5">
        <Text variant="h3">{title}</Text>
        {note && <span className="text-xs text-tertiary">{note}</span>}
      </div>
      {children}
    </Card>
  );
}

export function Empty({ label = 'No data' }: { label?: string }) {
  return <EmptyState title={label} size="sm" />;
}

export interface StatDelta {
  text: string;
  tone: 'good' | 'bad' | 'neutral';
}

const DELTA_COLOR = { good: 'success', bad: 'error', neutral: 'secondary' } as const;

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
    <Card className="flex flex-col gap-space-2">
      <span className="inline-flex items-center gap-1.5 text-xs font-medium uppercase tracking-wide text-tertiary">
        {accent && (
          <span className="size-2 shrink-0 rounded-[2px]" style={{ backgroundColor: accent }} />
        )}
        <span className="truncate" title={label}>
          {label}
        </span>
      </span>
      <span className="text-2xl font-semibold text-primary">{value}</span>
      {(delta || hint) && (
        <div className="flex flex-wrap items-center gap-1.5">
          {delta && <Badge color={DELTA_COLOR[delta.tone]}>{delta.text}</Badge>}
          {hint && <span className="text-xs text-tertiary">{hint}</span>}
        </div>
      )}
    </Card>
  );
}

export interface BarItem {
  key: string;
  label: string;
  value: number | null; // null → missing: empty track, em-dash value
  display: string;
  color: string;
}

export function BarList({
  items,
  label,
  format,
}: {
  items: BarItem[];
  label: string;
  format: (value: number) => string;
}) {
  if (items.length === 0) return <Empty />;
  return (
    <BarChart
      aria-label={label}
      orientation="horizontal"
      className="w-full"
      style={{ height: Math.max(140, items.length * 36 + 40) }}
      showLegend={false}
      plotPadding={{ right: 80 }}
      valueAxes={[{ id: 'value', formatValue: format }]}
      slots={{
        barLabel: (bar) => (
          <text
            x={bar.x + bar.width + 8}
            y={bar.y + bar.height / 2}
            dominantBaseline="middle"
            className="fill-current text-xs text-secondary"
          >
            {items.find((item) => item.key === bar.category)?.display}
          </text>
        ),
      }}
      categoryAxis={{
        formatValue: (key) => items.find((item) => item.key === key)?.label ?? String(key),
        thickness: 140,
      }}
      series={[
        {
          id: 'value',
          label: 'Value',
          valueAxisId: 'value',
          data: items.map((item) => ({ category: item.key, value: item.value, color: item.color })),
        },
      ]}
      getBarAriaLabel={(bar) => {
        const item = items.find((item) => item.key === bar.category);
        return `${item?.label}: ${item?.display}`;
      }}
    />
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

export function StackedBar({
  rows,
  format = String,
}: {
  rows: StackedRow[];
  format?: (n: number) => string;
}) {
  if (rows.length === 0) return <Empty />;
  const keys = [...new Set(rows.flatMap((row) => row.segments.map((segment) => segment.key)))];
  return (
    <BarChart
      aria-label="Results against baseline"
      orientation="horizontal"
      mode="stacked"
      showLegend={false}
      style={{ height: Math.max(140, rows.length * 36 + 40) }}
      valueAxes={[{ id: 'value', formatValue: format }]}
      series={keys.map((key) => ({
        id: key,
        label: key,
        valueAxisId: 'value',
        data: rows.map((row) => {
          const segment = row.segments.find((segment) => segment.key === key);
          return { category: row.label, value: segment?.value ?? 0, color: segment?.color };
        }),
      }))}
      getBarAriaLabel={(bar) => `${bar.category}: ${bar.seriesId} ${format(bar.value)}`}
    />
  );
}
