import type { Run } from '../types';

export interface Entry {
  label: string;
  value: number;
}

export function countBy<T>(items: T[], key: (t: T) => string): Map<string, number> {
  const m = new Map<string, number>();
  for (const it of items) {
    const k = key(it);
    m.set(k, (m.get(k) ?? 0) + 1);
  }
  return m;
}

export function sumBy<T>(items: T[], key: (t: T) => string, val: (t: T) => number): Map<string, number> {
  const m = new Map<string, number>();
  for (const it of items) {
    const k = key(it);
    m.set(k, (m.get(k) ?? 0) + val(it));
  }
  return m;
}

// Map → entries, sorted by value descending.
export function entries(m: Map<string, number>): Entry[] {
  return [...m.entries()].map(([label, value]) => ({ label, value })).sort((a, b) => b.value - a.value);
}

export function distinct<T>(items: T[], key: (t: T) => string | undefined): number {
  const s = new Set<string>();
  for (const it of items) {
    const k = key(it);
    if (k) s.add(k);
  }
  return s.size;
}

export function percentile(values: number[], p: number): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const idx = Math.min(sorted.length - 1, Math.max(0, Math.ceil((p / 100) * sorted.length) - 1));
  return sorted[idx];
}

// Local YYYY-MM-DD bucket for a run's start_time, or null if unparseable.
export function dayKey(run: Run): string | null {
  if (!run.start_time) return null;
  const t = Date.parse(run.start_time);
  if (!Number.isFinite(t)) return null;
  const d = new Date(t);
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${mm}-${dd}`;
}
