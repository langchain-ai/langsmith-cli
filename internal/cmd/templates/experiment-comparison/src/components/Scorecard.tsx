import type { ExampleWithRuns, ExperimentView } from '../types';
import type { RunMetric } from '../lib/metrics';
import { verdict } from '../lib/delta';
import { Text } from '@/components/langsmith/design-system/components/Text';
import { StackedBar, type StackedRow } from './primitives';

interface Props {
  examples: ExampleWithRuns[];
  experiments: ExperimentView[]; // baseline first, then comparisons
  metrics: RunMetric[];
}

interface Tally {
  better: number;
  worse: number;
  neutral: number;
}

// Per comparison, how many examples beat / lost to / tied the baseline per
// metric — a stacked win/tie/loss bar so the proportion reads at a glance.
export function Scorecard({ examples, experiments, metrics }: Props) {
  const baseline = experiments[0];
  const comparisons = experiments.slice(1);
  if (!baseline || comparisons.length === 0) return null;

  return (
    <div className="grid gap-space-4 sm:grid-cols-2 lg:grid-cols-3">
      {comparisons.map((exp) => {
        const rows: StackedRow[] = metrics.map((m) => {
          const t = tallyFor(examples, baseline.id, exp.id, m);
          return {
            label: m.label,
            segments: [
              { key: 'better', value: t.better, color: 'var(--bg-success-strong)' },
              { key: 'neutral', value: t.neutral, color: 'var(--border-strong)' },
              { key: 'worse', value: t.worse, color: 'var(--bg-error-strong)' },
            ],
          };
        });
        return (
          <div key={exp.id} className="rounded-lg border border-subtle p-space-4">
            <div className="mb-space-3 flex items-center gap-1.5">
              <span className="size-2.5 shrink-0 rounded-xs" style={{ backgroundColor: exp.color }} />
              <Text as="span" variant="md" weight="medium" className="truncate" title={exp.name}>
                {exp.name}
              </Text>
            </div>
            <StackedBar rows={rows} />
          </div>
        );
      })}
      <div className="flex items-center gap-space-4 text-xs text-tertiary sm:col-span-2 lg:col-span-3">
        <Swatch color="var(--bg-success-strong)" label="beat baseline" />
        <Swatch color="var(--border-strong)" label="tied" />
        <Swatch color="var(--bg-error-strong)" label="lost to baseline" />
      </div>
    </div>
  );
}

function Swatch({ color, label }: { color: string; label: string }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="size-2.5 shrink-0 rounded-xs" style={{ backgroundColor: color }} />
      {label}
    </span>
  );
}

function tallyFor(
  examples: ExampleWithRuns[],
  baselineId: string,
  compId: string,
  metric: RunMetric
): Tally {
  const t: Tally = { better: 0, worse: 0, neutral: 0 };
  for (const ex of examples) {
    const base = metric.value(ex.runs.find((r) => r.session_id === baselineId));
    const val = metric.value(ex.runs.find((r) => r.session_id === compId));
    const v = verdict(base, val, metric.lowerIsBetter);
    if (v === 'better') t.better++;
    else if (v === 'worse') t.worse++;
    else t.neutral++;
  }
  return t;
}
