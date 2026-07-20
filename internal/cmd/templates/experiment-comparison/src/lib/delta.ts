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

// Signed improvement of `value` over `baseline` (positive = better), honoring
// direction; null if either side is missing so callers never mistake a gap
// for a real zero delta.
export function improvementDelta(
  baseline: number | null,
  value: number | null,
  lowerIsBetter: boolean
): number | null {
  if (baseline == null || value == null) return null;
  return lowerIsBetter ? baseline - value : value - baseline;
}

// Compare value against baseline; 'neutral' when either is missing or they
// tie, so missing data never miscolors.
export function verdict(
  baseline: number | null,
  value: number | null,
  lowerIsBetter: boolean
): Verdict {
  const d = improvementDelta(baseline, value, lowerIsBetter);
  if (d == null || d === 0) return 'neutral';
  return d > 0 ? 'better' : 'worse';
}

// Token-based text colors that flip under html.dark, so deltas read in both themes.
export function verdictClass(v: Verdict): string {
  if (v === 'better') return 'text-success-primary';
  if (v === 'worse') return 'text-error-primary';
  return 'text-primary';
}
