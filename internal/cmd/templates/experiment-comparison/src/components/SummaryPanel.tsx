import { Card } from '@langchain/macaw-components/Card';
import { Text } from '@langchain/macaw-components/Text';
import type { Aggregate, ExperimentView } from '../types';
import { aggregateValue, type RunMetric } from '../lib/metrics';
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
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {metrics.map((m) => (
          <StatTile
            key={m.id}
            label={m.label}
            value={m.format(aggregateValue(aggregates[exp?.id ?? ''], m))}
          />
        ))}
      </div>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2">
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
          <Card intent="plain" key={m.id} className="flex min-w-0 flex-col gap-space-2">
            <Text variant="xs" color="tertiary" weight="medium">
              {m.label}
              {m.lowerIsBetter && (
                <span className="normal-case text-quaternary"> · lower is better</span>
              )}
            </Text>
            <BarList items={items} label={m.label} format={m.format} />
          </Card>
        );
      })}
    </div>
  );
}
