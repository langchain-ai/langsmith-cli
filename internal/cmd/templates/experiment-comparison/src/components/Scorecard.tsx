import { Card } from '@langchain/macaw-components/Card';
import { ChartLegend } from '@langchain/macaw-components/ChartLegend';
import {
  CHART_STATUS_FILL_COLORS,
  CHART_OTHER_COLOR,
} from '@langchain/macaw-components/utils/chartColors';
import type { ExampleWithRuns, ExperimentView } from '../types';
import type { RunMetric } from '../lib/metrics';
import { verdict } from '../lib/delta';
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
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {comparisons.map((exp) => {
        const rows: StackedRow[] = metrics.map((m) => {
          const t = tallyFor(examples, baseline.id, exp.id, m);
          return {
            label: m.label,
            segments: [
              { key: 'better', value: t.better, color: CHART_STATUS_FILL_COLORS.positive },
              { key: 'neutral', value: t.neutral, color: CHART_OTHER_COLOR },
              { key: 'worse', value: t.worse, color: CHART_STATUS_FILL_COLORS.negative },
            ],
          };
        });
        return (
          <Card key={exp.id} className="min-w-0">
            <div className="mb-3 flex items-center gap-1.5 text-sm font-medium text-primary">
              <span
                className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                style={{ backgroundColor: exp.color }}
              />
              <span className="truncate" title={exp.name}>
                {exp.name}
              </span>
            </div>
            <StackedBar rows={rows} />
          </Card>
        );
      })}
      <ChartLegend
        className="sm:col-span-2 lg:col-span-3"
        items={[
          { id: 'better', label: 'Beat baseline', markerColor: CHART_STATUS_FILL_COLORS.positive },
          { id: 'neutral', label: 'Tied', markerColor: CHART_OTHER_COLOR },
          {
            id: 'worse',
            label: 'Lost to baseline',
            markerColor: CHART_STATUS_FILL_COLORS.negative,
          },
        ]}
      />
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
