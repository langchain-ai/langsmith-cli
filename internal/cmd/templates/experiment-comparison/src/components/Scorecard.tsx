import type { ExampleWithRuns, ExperimentView } from '../types';
import type { RunMetric } from '../lib/metrics';
import { verdict } from '../lib/delta';

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

// Per comparison, how many examples beat / lost to / tied the baseline per metric.
export function Scorecard({ examples, experiments, metrics }: Props) {
  const baseline = experiments[0];
  const comparisons = experiments.slice(1);
  if (!baseline || comparisons.length === 0) return null;

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {comparisons.map((exp) => (
        <div key={exp.id} className="rounded-lg border border-secondary p-4">
          <div className="mb-3 flex items-center gap-1.5 text-sm font-medium text-primary">
            <span className="h-2.5 w-2.5 shrink-0 rounded-[2px]" style={{ backgroundColor: exp.color }} />
            <span className="truncate" title={exp.name}>{exp.name}</span>
          </div>
          <table className="w-full text-sm">
            <tbody>
              {metrics.map((m) => {
                const t = tallyFor(examples, baseline.id, exp.id, m);
                return (
                  <tr key={m.id} className="border-t border-secondary first:border-0">
                    <td className="py-1.5 pr-2 text-secondary">{m.label}</td>
                    <td className="py-1.5 text-right tabular-nums">
                      <span className="text-success-primary">{t.better}↑</span>{' '}
                      <span className="text-error-primary">{t.worse}↓</span>{' '}
                      <span className="text-tertiary">{t.neutral}=</span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      ))}
    </div>
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
