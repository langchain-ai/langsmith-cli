import type { Run } from '../types';
import { durationMs, repoOf, threadOf, UNKNOWN } from '../lib/normalize';
import { formatCost, formatDuration, formatRelativeTime, formatTokens } from '../lib/format';
import { Empty } from './primitives';

// A flat, bounded sample of the most recent turns — for browsing, not for
// computing any of the stats above it (see api.ts's RECENT_RUNS_LIMIT).
export function RunsTable({ runs }: { runs: Run[] }) {
  if (runs.length === 0) return <Empty label="No recent turns in this window" />;

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-xs text-tertiary">
            <th className="pb-1.5 font-medium">Turn</th>
            <th className="pb-1.5 font-medium">Repo</th>
            <th className="pb-1.5 font-medium">Thread</th>
            <th className="pb-1.5 text-right font-medium">Duration</th>
            <th className="pb-1.5 text-right font-medium">Tokens</th>
            <th className="pb-1.5 text-right font-medium">Cost</th>
            <th className="pb-1.5 text-right font-medium">When</th>
          </tr>
        </thead>
        <tbody>
          {runs.map((r) => {
            const ms = durationMs(r);
            const thread = threadOf(r);
            const repo = repoOf(r);
            return (
              <tr key={r.id} className="border-t border-default">
                <td className="max-w-[220px] truncate py-1.5 pr-2 text-primary" title={r.name ?? ''}>
                  <span className="inline-flex items-center gap-1.5">
                    {r.error && (
                      <span
                        className="size-1.5 shrink-0 rounded-full bg-error-strong"
                        title={r.error}
                        aria-label="This turn errored"
                      />
                    )}
                    {r.name ?? 'run'}
                  </span>
                </td>
                <td className="max-w-[160px] truncate py-1.5 pr-2 text-secondary" title={repo}>
                  {repo === UNKNOWN ? '—' : repo}
                </td>
                <td className="py-1.5 pr-2 font-mono text-xs text-tertiary" title={thread}>
                  {thread ? thread.slice(0, 8) : '—'}
                </td>
                <td className="py-1.5 text-right tabular-nums text-secondary">{ms != null ? formatDuration(ms) : '—'}</td>
                <td className="py-1.5 text-right tabular-nums text-secondary">{formatTokens(r.total_tokens ?? 0)}</td>
                <td className="py-1.5 text-right tabular-nums text-secondary">{formatCost(r.total_cost ?? 0)}</td>
                <td className="py-1.5 text-right tabular-nums text-tertiary">{r.start_time ? formatRelativeTime(r.start_time) : '—'}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
