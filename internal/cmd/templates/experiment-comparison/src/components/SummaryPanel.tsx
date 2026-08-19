import type { Aggregate, ExperimentView } from '../types';
import { aggregateValue, type RunMetric } from '../lib/metrics';
import { Text } from '@/components/langsmith/design-system/components/Text';
import { BarList, StatTile, type BarItem } from './primitives';

interface Props {
  experiments: ExperimentView[]; // baseline first, then comparisons
  aggregates: Record<string, Aggregate>;
  metrics: RunMetric[];
}

// Every metric ExampleTable/ScatterPlot can plot, rendered as one small
// grouped-bar comparison per metric. With a lone baseline (no comparisons
// picked yet) a one-bar chart is a stat tile in bar's clothing, so that case
// renders a plain KPI row instead.
export function SummaryPanel({ experiments, aggregates, metrics }: Props) {
  if (experiments.length <= 1) {
    const exp = experiments[0];
    return (
      <div className="grid grid-cols-2 gap-space-3 sm:grid-cols-3 lg:grid-cols-4">
        {metrics.map((m) => (
          <StatTile key={m.id} label={m.label} value={m.format(aggregateValue(aggregates[exp?.id ?? ''], m))} />
        ))}
      </div>
    );
  }

  return (
    <div className="grid gap-space-4 sm:grid-cols-2">
      {metrics.map((m) => {
        const items: BarItem[] = experiments.map((x) => {
          const v = aggregateValue(aggregates[x.id], m);
          return {
            key: x.id,
            label: `${x.letter} · ${x.name}`,
            value: v,
            display: m.format(v),
            color: x.color,
          };
        });
        return (
          <div key={m.id} className="flex flex-col gap-space-2 rounded-lg border border-subtle p-space-4">
            <Text as="span" variant="xs" weight="medium" color="tertiary" className="uppercase tracking-wide">
              {m.label}
              {m.lowerIsBetter && <span className="normal-case text-quaternary"> · lower is better</span>}
            </Text>
            <BarList items={items} />
          </div>
        );
      })}
    </div>
  );
}
