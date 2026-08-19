import type { CodingShare, ProjectStats } from '../types';
import { formatCost, formatDuration, formatInt, formatPct, formatTokens } from '../lib/format';
import { StatTile, type StatTone } from './primitives';

const DASH = '—';

function errorTone(rate: number | null | undefined): StatTone | undefined {
  if (rate == null) return undefined;
  if (rate >= 10) return 'bad';
  if (rate >= 2) return 'warning';
  return 'good';
}

// The headline numbers, straight from the two stats endpoints — exact,
// whole-window aggregates, not derived from the sampled recent-runs table
// below. A tile shows "—" only when its field is genuinely absent from the
// response, never a silently-wrong zero.
export function OverviewPanel({ stats, codingShare }: { stats: ProjectStats; codingShare: CodingShare | null }) {
  const t = stats.turns;
  const th = stats.threads;

  // "50%" over the last N root runs, with "10 of last 20" spelling out the
  // sample. Null (query failed / no runs yet) shows "—", never a fake 0%.
  const hasShare = codingShare != null && codingShare.total > 0;
  const sharePct = hasShare ? (codingShare!.coding / codingShare!.total) * 100 : null;

  const tokenHint =
    t?.prompt_tokens != null && t?.completion_tokens != null
      ? `${formatTokens(t.prompt_tokens)} prompt · ${formatTokens(t.completion_tokens)} completion`
      : undefined;
  const costHint =
    t?.prompt_cost != null && t?.completion_cost != null
      ? `${formatCost(t.prompt_cost)} prompt · ${formatCost(t.completion_cost)} completion`
      : undefined;

  return (
    <div className="grid grid-cols-2 gap-space-4 sm:grid-cols-3 lg:grid-cols-4">
      <StatTile label="Turns" value={t?.run_count != null ? formatInt(t.run_count) : DASH} />
      <StatTile label="Threads" value={th?.group_count != null ? formatInt(th.group_count) : DASH} />
      <StatTile label="Cost" value={t?.total_cost != null ? formatCost(t.total_cost) : DASH} hint={costHint} />
      <StatTile label="Tokens" value={t?.total_tokens != null ? formatTokens(t.total_tokens) : DASH} hint={tokenHint} />
      <StatTile
        label="Error rate"
        value={t?.error_rate != null ? formatPct(t.error_rate) : DASH}
        tone={errorTone(t?.error_rate)}
      />
      <StatTile
        label="Coding share"
        value={sharePct != null ? formatPct(sharePct) : DASH}
        hint={hasShare ? `${formatInt(codingShare!.coding)} of last ${formatInt(codingShare!.total)}` : undefined}
      />
      <StatTile label="p50 latency" value={t?.latency_p50 != null ? formatDuration(t.latency_p50) : DASH} />
      <StatTile label="p99 latency" value={t?.latency_p99 != null ? formatDuration(t.latency_p99) : DASH} />
    </div>
  );
}
