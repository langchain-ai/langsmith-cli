import type { Run } from '../types';

// Custom metadata read helpers for the recent-runs table. Field names vary
// across integrations, so the entity accessors fall back through the known
// aliases (see AGENTS.md).
function str(run: Run, key: string): string | undefined {
  const v = run.extra?.metadata?.[key];
  return typeof v === 'string' && v ? v : undefined;
}

export const UNKNOWN = 'unknown';

export function repoOf(run: Run): string {
  return str(run, 'repository_name') ?? UNKNOWN;
}

export function threadOf(run: Run): string | undefined {
  return str(run, 'thread_id');
}

export function durationMs(run: Run): number | null {
  if (!run.start_time || !run.end_time) return null;
  const s = Date.parse(run.start_time);
  const e = Date.parse(run.end_time);
  return Number.isFinite(s) && Number.isFinite(e) && e >= s ? e - s : null;
}
