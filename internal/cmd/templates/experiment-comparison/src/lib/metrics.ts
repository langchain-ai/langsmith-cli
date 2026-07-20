import type { Aggregate, ExperimentRun } from '../types';
import { costOf, latencyMs, scoreFor } from './delta';

// A single comparable measure read straight off one run.
export interface RunMetric {
  id: string;
  label: string;
  lowerIsBetter: boolean;
  value: (run: ExperimentRun | undefined) => number | null;
  format: (v: number | null) => string;
}

export const fmt = {
  ms: (v: number | null) => (v == null ? '—' : `${Math.round(v)} ms`),
  cost: (v: number | null) => (v == null ? '—' : `$${v.toFixed(4)}`),
  tokens: (v: number | null) => (v == null ? '—' : Math.round(v).toLocaleString()),
  score: (v: number | null) => (v == null ? '—' : v.toFixed(3)),
};

// Feedback keys first, then the always-lower-is-better system metrics.
export function buildMetrics(
  feedbackKeys: string[],
  lowerIsBetter: Record<string, boolean>
): RunMetric[] {
  const scores: RunMetric[] = feedbackKeys.map((key) => ({
    id: `score:${key}`,
    label: key,
    lowerIsBetter: lowerIsBetter[key] ?? false,
    value: (run) => scoreFor(run, key),
    format: fmt.score,
  }));
  const system: RunMetric[] = [
    { id: 'latency', label: 'Latency', lowerIsBetter: true, value: latencyMs, format: fmt.ms },
    { id: 'cost', label: 'Cost', lowerIsBetter: true, value: costOf, format: fmt.cost },
    {
      id: 'tokens',
      label: 'Tokens',
      lowerIsBetter: true,
      value: (run) => (typeof run?.total_tokens === 'number' ? run.total_tokens : null),
      format: fmt.tokens,
    },
  ];
  return [...scores, ...system];
}

// Reads the aggregate matching a RunMetric id, so components that show
// per-experiment aggregates (Scoreboard, SummaryPanel) can share the same
// metric list ExampleTable/ScatterPlot use instead of re-deriving their own.
export function aggregateValue(agg: Aggregate | undefined, metric: RunMetric): number | null {
  if (!agg) return null;
  if (metric.id === 'latency') return agg.avgLatencyMs;
  if (metric.id === 'cost') return agg.totalCost;
  if (metric.id === 'tokens') return agg.avgTokens;
  if (metric.id.startsWith('score:')) {
    const key = metric.id.slice('score:'.length);
    return key in agg.avgScores ? agg.avgScores[key] : null;
  }
  return null;
}

export interface HistogramBin {
  lo: number;
  hi: number;
  count: number;
}

// Fixed-width bins spanning the observed range; empty bins are kept so the
// bars read as one continuous distribution instead of a sparse strip.
export function histogram(values: number[], binCount = 9): HistogramBin[] {
  if (values.length === 0) return [];
  let lo = Math.min(...values);
  let hi = Math.max(...values);
  if (lo === hi) {
    lo -= 1;
    hi += 1;
  }
  const width = (hi - lo) / binCount;
  const bins: HistogramBin[] = Array.from({ length: binCount }, (_, i) => ({
    lo: lo + i * width,
    hi: lo + (i + 1) * width,
    count: 0,
  }));
  for (const v of values) {
    const idx = Math.min(binCount - 1, Math.floor((v - lo) / width));
    bins[idx].count++;
  }
  return bins;
}

// Series/small-multiples stay CVD-safe (and legible) only for the first four slots.
export const SERIES_CAP = 4;

const SERIES_VARS = ['var(--series-1)', 'var(--series-2)', 'var(--series-3)', 'var(--series-4)'];

// A, B, C… for the ordered experiments.
export function letterFor(index: number): string {
  return String.fromCharCode(65 + index);
}

// Comparison index (0-based); past the palette, fold to a neutral tone.
export function comparisonColor(index: number): string {
  return SERIES_VARS[index] ?? 'var(--text-tertiary)';
}
