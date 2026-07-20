import type { ReactNode } from 'react';

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
    <div className="flex flex-col gap-1 rounded-lg border border-secondary p-4">
      <span className="text-xs font-medium uppercase tracking-wide text-tertiary">{label}</span>
      <span className={`text-2xl font-semibold ${tone ? TONE_CLASS[tone] : 'text-primary'}`}>{value}</span>
      {hint && <span className="text-xs text-tertiary">{hint}</span>}
    </div>
  );
}
