import { useMemo } from 'react';
import type { Run } from '../types';
import { distinct } from '../lib/aggregate';
import { repoOf, threadOf } from '../lib/normalize';
import { formatCost, formatInt, formatPct, formatTokens } from '../lib/format';
import { StatTile } from './primitives';

export function OverviewPanel({ roots }: { roots: Run[] }) {
  const s = useMemo(() => {
    const tokens = roots.reduce((a, r) => a + (r.total_tokens ?? 0), 0);
    const cost = roots.reduce((a, r) => a + (r.total_cost ?? 0), 0);
    const errors = roots.filter((r) => r.error).length;
    return {
      turns: roots.length,
      threads: distinct(roots, threadOf),
      repos: distinct(roots, repoOf),
      tokens,
      cost,
      errors,
      errorRate: roots.length ? (errors / roots.length) * 100 : 0,
    };
  }, [roots]);

  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
      <StatTile label="Turns" value={formatInt(s.turns)} />
      <StatTile label="Threads" value={formatInt(s.threads)} />
      <StatTile label="Repos" value={formatInt(s.repos)} />
      <StatTile label="Tokens" value={formatTokens(s.tokens)} />
      <StatTile label="Cost" value={formatCost(s.cost)} />
      <StatTile label="Error rate" value={formatPct(s.errorRate)} hint={`${s.errors} failed`} />
    </div>
  );
}
