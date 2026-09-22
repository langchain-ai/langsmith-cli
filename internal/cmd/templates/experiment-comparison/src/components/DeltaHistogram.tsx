import { useMemo } from 'react';
import { BarChart } from '@langchain/macaw-components/BarChart';
import { Card } from '@langchain/macaw-components/Card';
import { ChartLegend } from '@langchain/macaw-components/ChartLegend';
import {
  CHART_STATUS_FILL_COLORS,
  CHART_OTHER_COLOR,
} from '@langchain/macaw-components/utils/chartColors';
import type { ExampleWithRuns, ExperimentView } from '../types';
import { histogram, SERIES_CAP, type RunMetric } from '../lib/metrics';
import { improvementDelta } from '../lib/delta';
import { Empty } from './primitives';

interface Props {
  examples: ExampleWithRuns[];
  experiments: ExperimentView[]; // baseline first, then comparisons
  metric: RunMetric;
}

// Per comparison, a distribution of per-example improvement over the
// baseline for the selected metric — positive (green) bars are wins,
// negative (red) bars are regressions, so the shape of the win/loss spread
// is visible at a glance, not just its average.
export function DeltaHistogram({ examples, experiments, metric }: Props) {
  const baseline = experiments[0];
  const shown = experiments.slice(1, 1 + SERIES_CAP);
  const hidden = experiments.length - 1 - shown.length;
  if (!baseline) return null;
  if (shown.length === 0)
    return <Empty label="Pick a comparison experiment to see its delta distribution." />;

  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {shown.map((exp) => (
          <ComparisonHistogram
            key={exp.id}
            examples={examples}
            baselineId={baseline.id}
            comparison={exp}
            metric={metric}
          />
        ))}
      </div>
      {hidden > 0 && (
        <p className="text-xs text-tertiary">
          +{hidden} more comparison{hidden > 1 ? 's' : ''} not plotted.
        </p>
      )}
    </div>
  );
}

function ComparisonHistogram({
  examples,
  baselineId,
  comparison,
  metric,
}: {
  examples: ExampleWithRuns[];
  baselineId: string;
  comparison: ExperimentView;
  metric: RunMetric;
}) {
  const bins = useMemo(() => {
    const deltas: number[] = [];
    for (const ex of examples) {
      const base = metric.value(ex.runs.find((r) => r.session_id === baselineId));
      const val = metric.value(ex.runs.find((r) => r.session_id === comparison.id));
      const d = improvementDelta(base, val, metric.lowerIsBetter);
      if (d != null) deltas.push(d);
    }
    return histogram(deltas, 9);
  }, [examples, baselineId, comparison.id, metric]);

  return (
    <Card className="min-w-0">
      <ChartLegend
        items={[{ id: comparison.id, label: comparison.name, markerColor: comparison.color }]}
      />
      {bins.length === 0 ? (
        <Empty label="No paired values" />
      ) : (
        <BarChart
          aria-label={`${comparison.name}: improvement over baseline`}
          className="h-48 w-full"
          showLegend={false}
          categoryPadding={{ inner: 0.08 }}
          categoryAxis={{
            label: 'Improvement',
            tickCount: 3,
            formatValue: (index) =>
              metric.format((bins[Number(index)].lo + bins[Number(index)].hi) / 2),
          }}
          valueAxes={[{ id: 'count', label: 'Examples', tickCount: 3 }]}
          series={[
            {
              id: 'examples',
              label: 'Examples',
              valueAxisId: 'count',
              data: bins.map((bin, index) => ({
                category: index,
                value: bin.count,
                color:
                  (bin.lo + bin.hi) / 2 > 0
                    ? CHART_STATUS_FILL_COLORS.positive
                    : (bin.lo + bin.hi) / 2 < 0
                      ? CHART_STATUS_FILL_COLORS.negative
                      : CHART_OTHER_COLOR,
              })),
            },
          ]}
          getBarAriaLabel={(bar) => {
            const bin = bins[Number(bar.category)];
            return `${metric.format(bin.lo)} to ${metric.format(bin.hi)}: ${bin.count} examples`;
          }}
        />
      )}
    </Card>
  );
}
