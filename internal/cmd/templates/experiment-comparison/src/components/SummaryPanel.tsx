import type { Aggregate, ExperimentView } from '../types';
import { verdict, verdictClass } from '../lib/delta';
import { fmt } from '../lib/metrics';

interface Props {
  experiments: ExperimentView[]; // baseline first, then comparisons
  aggregates: Record<string, Aggregate>;
  feedbackKeys: string[];
  lowerIsBetter: Record<string, boolean>;
}

interface Metric {
  label: string;
  lowerIsBetter: boolean;
  value: (a: Aggregate | undefined) => number | null;
  format: (v: number | null) => string;
}

export function SummaryPanel({ experiments, aggregates, feedbackKeys, lowerIsBetter }: Props) {
  const baselineId = experiments[0]?.id;
  const baseAgg = aggregates[baselineId];

  const metrics: Metric[] = [
    { label: 'Avg latency', lowerIsBetter: true, value: (a) => a?.avgLatencyMs ?? null, format: fmt.ms },
    { label: 'Total cost', lowerIsBetter: true, value: (a) => a?.totalCost ?? null, format: fmt.cost },
    { label: 'Avg tokens', lowerIsBetter: true, value: (a) => a?.avgTokens ?? null, format: fmt.tokens },
    ...feedbackKeys.map(
      (key): Metric => ({
        label: `Avg ${key}`,
        lowerIsBetter: lowerIsBetter[key] ?? false,
        value: (a) => (a && key in a.avgScores ? a.avgScores[key] : null),
        format: fmt.score,
      })
    ),
  ];

  return (
    <div className="overflow-x-auto rounded-lg border border-secondary">
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="border-b border-secondary bg-surface-level-2">
            <th className="px-3 py-2 text-left font-medium text-tertiary">Metric</th>
            {experiments.map((x) => (
              <th key={x.id} className="px-3 py-2 text-left font-medium text-primary">
                <span className="inline-flex items-center gap-1.5">
                  <span className="h-2.5 w-2.5 shrink-0 rounded-[2px]" style={{ backgroundColor: x.color }} />
                  <span className="truncate" title={x.name}>{x.name}</span>
                  {x.isBaseline && <span className="text-xs text-tertiary">(baseline)</span>}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {metrics.map((m) => {
            const base = m.value(baseAgg);
            return (
              <tr key={m.label} className="border-b border-secondary last:border-0">
                <td className="px-3 py-2 text-secondary">{m.label}</td>
                {experiments.map((x) => {
                  const v = m.value(aggregates[x.id]);
                  const cls = x.isBaseline
                    ? 'text-primary'
                    : verdictClass(verdict(base, v, m.lowerIsBetter));
                  return (
                    <td key={x.id} className={`px-3 py-2 tabular-nums ${cls}`}>
                      {m.format(v)}
                    </td>
                  );
                })}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
