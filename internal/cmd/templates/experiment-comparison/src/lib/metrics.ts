import type { ExperimentRun } from '../types';
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

// Scatter/all-pairs stays CVD-safe only for the first four series slots.
export const SCATTER_SERIES_CAP = 4;

const SERIES_VARS = ['var(--series-1)', 'var(--series-2)', 'var(--series-3)', 'var(--series-4)'];

// A, B, C… for the ordered experiments.
export function letterFor(index: number): string {
  return String.fromCharCode(65 + index);
}

// Comparison index (0-based); past the palette, fold to a neutral tone.
export function comparisonColor(index: number): string {
  return SERIES_VARS[index] ?? 'var(--text-tertiary)';
}
