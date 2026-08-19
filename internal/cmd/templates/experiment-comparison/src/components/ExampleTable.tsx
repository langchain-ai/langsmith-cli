import { useMemo, useState } from 'react';
import { Select } from '@/components/langsmith/design-system/components/Select';
import { Text } from '@/components/langsmith/design-system/components/Text';
import type { ExampleWithRuns, ExperimentRun, ExperimentView } from '../types';
import { improvementDelta, verdict, verdictClass } from '../lib/delta';
import type { RunMetric } from '../lib/metrics';

interface Props {
  examples: ExampleWithRuns[];
  experiments: ExperimentView[]; // baseline first, then comparisons
  metric: RunMetric | undefined; // drives the per-cell value and coloring
}

type Sort = 'regression' | 'improvement' | 'default';

function truncate(value: unknown, max = 80): string {
  if (value == null) return '—';
  const s = typeof value === 'string' ? value : JSON.stringify(value);
  return s.length > max ? s.slice(0, max) + '…' : s;
}

function runFor(example: ExampleWithRuns, sessionId: string): ExperimentRun | undefined {
  return example.runs.find((r) => r.session_id === sessionId);
}

// One row per example; per experiment its output and the selected metric,
// colored vs the baseline run and sortable by worst regression.
export function ExampleTable({ examples, experiments, metric }: Props) {
  const baseline = experiments[0];
  const comparisons = experiments.slice(1);
  const [sort, setSort] = useState<Sort>('default');

  const rows = useMemo(() => {
    if (sort === 'default' || !metric || !baseline || comparisons.length === 0) return examples;
    const scored = examples.map((ex) => ({ ex, score: worstDelta(ex, baseline.id, comparisons, metric) }));
    scored.sort((a, b) => rank(a.score, b.score, sort));
    return scored.map((s) => s.ex);
  }, [examples, sort, metric, baseline, comparisons]);

  return (
    <div className="flex flex-col gap-space-2">
      {comparisons.length > 0 && metric && (
        <div className="flex items-center gap-space-2">
          <Text variant="sm" color="tertiary">
            Sort
          </Text>
          <Select
            size="sm"
            value={sort}
            onChange={(value) => value && setSort(value as Sort)}
            triggerClassName="w-[260px]"
            options={[
              { value: 'default', label: 'Default order' },
              { value: 'regression', label: `Worst ${metric.label} regressions` },
              { value: 'improvement', label: `Best ${metric.label} improvements` },
            ]}
          />
        </div>
      )}

      <div className="overflow-x-auto rounded-lg border border-default">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-default bg-surface-level-2">
              <th className="px-space-3 py-space-2 text-left font-medium text-tertiary">Input</th>
              {experiments.map((x) => (
                <th key={x.id} colSpan={2} className="px-space-3 py-space-2 text-left font-medium text-primary">
                  <span className="inline-flex items-center gap-1.5">
                    <span className="size-2.5 shrink-0 rounded-xs" style={{ backgroundColor: x.color }} />
                    <span className="truncate" title={x.name}>{x.name}</span>
                    {x.isBaseline && <span className="text-xs text-tertiary">(baseline)</span>}
                  </span>
                </th>
              ))}
            </tr>
            <tr className="border-b border-default bg-surface-level-2 text-xs text-tertiary">
              <th className="px-space-3 py-1.5 text-left font-normal" />
              {experiments.map((x) => (
                <th key={x.id} colSpan={2} className="px-space-3 py-1.5 text-left font-normal">
                  output · {metric?.label ?? 'value'}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((ex) => {
              const baseVal = metric ? metric.value(runFor(ex, baseline?.id ?? '')) : null;
              return (
                <tr key={ex.id} className="border-b border-default align-top last:border-0">
                  <td
                    className="max-w-[240px] truncate px-space-3 py-space-2 text-secondary"
                    title={truncate(ex.inputs, 500)}
                  >
                    {truncate(ex.inputs)}
                  </td>
                  {experiments.map((x) => {
                    const run = runFor(ex, x.id);
                    const value = metric ? metric.value(run) : null;
                    const cls = x.isBaseline
                      ? 'text-tertiary'
                      : verdictClass(verdict(baseVal, value, metric?.lowerIsBetter ?? false));
                    return (
                      <FragmentCells
                        key={x.id}
                        output={truncate(run?.outputs ?? run?.outputs_preview)}
                        value={metric ? metric.format(value) : '—'}
                        valueClass={cls}
                      />
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function FragmentCells(props: { output: string; value: string; valueClass: string }) {
  return (
    <>
      <td className="max-w-[240px] truncate px-space-3 py-space-2 text-primary" title={props.output}>
        {props.output}
      </td>
      <td className={`whitespace-nowrap px-space-3 py-space-2 tabular-nums ${props.valueClass}`}>{props.value}</td>
    </>
  );
}

// Most negative directional delta across comparisons; null when nothing pairs up.
function worstDelta(
  ex: ExampleWithRuns,
  baselineId: string,
  comparisons: ExperimentView[],
  metric: RunMetric
): number | null {
  const base = metric.value(runFor(ex, baselineId));
  let worst: number | null = null;
  for (const c of comparisons) {
    const d = improvementDelta(base, metric.value(runFor(ex, c.id)), metric.lowerIsBetter);
    if (d == null) continue;
    worst = worst == null ? d : Math.min(worst, d);
  }
  return worst;
}

// Nulls sink; regression = most negative first, improvement = most positive first.
function rank(a: number | null, b: number | null, sort: Sort): number {
  if (a == null && b == null) return 0;
  if (a == null) return 1;
  if (b == null) return -1;
  return sort === 'regression' ? a - b : b - a;
}
