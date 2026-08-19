import type { Aggregate, ExperimentView } from '../types';
import { aggregateValue, type RunMetric } from '../lib/metrics';
import { verdict } from '../lib/delta';
import { StatTile, type StatDelta } from './primitives';

interface Props {
  experiments: ExperimentView[]; // baseline first, then comparisons
  aggregates: Record<string, Aggregate>;
  metric: RunMetric;
}

function signed(raw: number, format: (v: number | null) => string): string {
  if (raw === 0) return `±${format(0)}`;
  return `${raw > 0 ? '+' : '−'}${format(Math.abs(raw))}`;
}

const TONE: Record<ReturnType<typeof verdict>, StatDelta['tone']> = {
  better: 'good',
  worse: 'bad',
  neutral: 'neutral',
};

// The headline moment: baseline's value, then each comparison's value with a
// delta pill (colored by whether it actually beat the baseline, not by sign).
export function Scoreboard({ experiments, aggregates, metric }: Props) {
  const baseline = experiments[0];
  const comparisons = experiments.slice(1);
  if (!baseline) return null;
  const baseValue = aggregateValue(aggregates[baseline.id], metric);

  return (
    <div className="grid grid-cols-2 gap-space-3 sm:grid-cols-3 lg:grid-cols-4">
      <StatTile
        label={`${baseline.letter} · ${baseline.name}`}
        value={metric.format(baseValue)}
        hint="baseline"
        accent={baseline.color}
      />
      {comparisons.map((exp) => {
        const value = aggregateValue(aggregates[exp.id], metric);
        const raw = baseValue != null && value != null ? value - baseValue : null;
        const v = verdict(baseValue, value, metric.lowerIsBetter);
        return (
          <StatTile
            key={exp.id}
            label={`${exp.letter} · ${exp.name}`}
            value={metric.format(value)}
            accent={exp.color}
            delta={raw == null ? undefined : { text: signed(raw, metric.format), tone: TONE[v] }}
          />
        );
      })}
    </div>
  );
}
