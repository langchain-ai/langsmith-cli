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

export type StatTone = 'good' | 'warning' | 'bad';

const TONE_CLASS: Record<StatTone, string> = {
  good: 'text-success-primary',
  warning: 'text-warning-primary',
  bad: 'text-error-primary',
};

// tone colors the value itself (a status, not a series) — e.g. an error rate
// tile going amber past a threshold. Omit it for a plain neutral stat.
export function StatTile({
  label,
  value,
  hint,
  tone,
}: {
  label: string;
  value: string;
  hint?: string;
  tone?: StatTone;
}) {
  return (
    <Card className="flex flex-col gap-space-1">
      <span className="text-xs font-medium uppercase tracking-wide text-tertiary">{label}</span>
      <span className={`text-2xl font-semibold ${tone ? TONE_CLASS[tone] : 'text-primary'}`}>
        {value}
      </span>
      {hint && <span className="text-xs text-tertiary">{hint}</span>}
    </Card>
  );
}
