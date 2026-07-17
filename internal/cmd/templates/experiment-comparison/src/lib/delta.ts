import type { ExperimentRun } from '../types';

export type Verdict = 'better' | 'worse' | 'neutral';

// Coerce feedback_stats[key].avg to a number, guarding the untyped nested map
// so a missing/non-numeric score returns null instead of NaN.
export function scoreFor(run: ExperimentRun | undefined, key: string): number | null {
  const raw = run?.feedback_stats?.[key]?.avg;
  const n = typeof raw === 'number' ? raw : Number(raw);
  return Number.isFinite(n) ? n : null;
}

export function latencyMs(run: ExperimentRun | undefined): number | null {
  if (!run?.end_time) return null;
  const ms = new Date(run.end_time).getTime() - new Date(run.start_time).getTime();
  return Number.isFinite(ms) ? ms : null;
}

export function costOf(run: ExperimentRun | undefined): number | null {
  const n = Number(run?.total_cost);
  return Number.isFinite(n) ? n : null;
}

// Compare value against baseline; 'neutral' when either is missing or they
// tie, so missing data never miscolors.
export function verdict(
  baseline: number | null,
  value: number | null,
  lowerIsBetter: boolean
): Verdict {
  if (baseline == null || value == null || value === baseline) return 'neutral';
  const valueIsBetter = lowerIsBetter ? value < baseline : value > baseline;
  return valueIsBetter ? 'better' : 'worse';
}

// Token-based text colors that flip under html.dark, so deltas read in both themes.
export function verdictClass(v: Verdict): string {
  if (v === 'better') return 'text-success-primary';
  if (v === 'worse') return 'text-error-primary';
  return 'text-primary';
}
