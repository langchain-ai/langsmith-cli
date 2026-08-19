import type { ReactNode } from 'react';
import { Text } from '@/components/langsmith/design-system/components/Text';

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

export type StatTone = 'good' | 'warning' | 'bad';

const TONE_CLASS: Record<StatTone, string> = {
  good: 'text-success-primary',
  warning: 'text-warning-primary',
  bad: 'text-error-primary',
};

// tone colors the value itself (a status, not a series) — e.g. an error rate
// tile going amber past a threshold. Omit it for a plain neutral stat.
export function StatTile({ label, value, hint, tone }: { label: string; value: string; hint?: string; tone?: StatTone }) {
  return (
    <div className="flex flex-col gap-space-1 rounded-lg border border-default p-space-4">
      <Text variant="xs" weight="medium" color="tertiary" className="uppercase tracking-wide">
        {label}
      </Text>
      <span className={`text-2xl font-semibold ${tone ? TONE_CLASS[tone] : 'text-primary'}`}>{value}</span>
      {hint && (
        <Text variant="sm" color="tertiary">
          {hint}
        </Text>
      )}
    </div>
  );
}
