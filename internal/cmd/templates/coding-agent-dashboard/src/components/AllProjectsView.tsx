import { useEffect, useState } from 'react';
import { fetchProjectsSummary } from '../api';
import type { ProjectSummary } from '../types';
import { formatInt, formatPct } from '../lib/format';
import { Section, StatTile } from './primitives';

const CODING = 'var(--brand-400)';
const OTHER = 'var(--gray-400)';

// Feasibility probe: how many projects carry coding-agent traces. Each project
// is sampled from its 100 most-recent root runs, so counts can undercount.
export function AllProjectsView() {
  const [rows, setRows] = useState<ProjectSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setLoading(true);
    setFailed(false);
    fetchProjectsSummary()
      .then((r) => setRows([...r].sort((a, b) => b.coding - a.coding)))
      .catch((e) => {
        console.error('Failed to scan projects', e);
        setFailed(true);
      })
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <Centered>Scanning projects…</Centered>;
  if (failed) return <Centered>Failed to scan projects — check the console and your access.</Centered>;
  if (rows.length === 0) return <Centered>No tracing projects in this workspace.</Centered>;

  const withCoding = rows.filter((r) => r.coding > 0).length;
  const totalCoding = rows.reduce((a, r) => a + r.coding, 0);
  const anyCapped = rows.some((r) => r.capped);

  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
        <StatTile label="Projects" value={formatInt(rows.length)} />
        <StatTile label="With coding traces" value={formatInt(withCoding)} />
        <StatTile label="Coding traces" value={formatInt(totalCoding)} hint="sampled, last 30d" />
      </div>

      <Section
        title="Coding-agent traces by project"
        note={`Sampled from each project's ${anyCapped ? '100 most-recent (capped)' : 'recent'} root traces over the last 30 days — a feasibility probe, not exact totals.`}
      >
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-xs text-tertiary">
              <th className="pb-1 font-medium">Project</th>
              <th className="pb-1 text-right font-medium">Coding</th>
              <th className="pb-1 text-right font-medium">Other</th>
              <th className="pb-1 text-right font-medium">Sampled</th>
              <th className="w-40 pb-1 font-medium">Coding share</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => {
              const pct = r.sampled ? (r.coding / r.sampled) * 100 : 0;
              return (
                <tr key={r.project.id} className="border-t border-secondary">
                  <td className="min-w-0 py-1.5 pr-3 text-primary">
                    <span className="block truncate" title={r.project.name}>{r.project.name}</span>
                  </td>
                  <td className="py-1.5 text-right tabular-nums text-secondary">{r.coding}</td>
                  <td className="py-1.5 text-right tabular-nums text-tertiary">{r.other}</td>
                  <td className="py-1.5 text-right tabular-nums text-tertiary">
                    {r.sampled}
                    {r.capped ? '+' : ''}
                  </td>
                  <td className="py-1.5">
                    <span className="flex items-center gap-2">
                      <span className="flex h-2.5 min-w-0 flex-1 overflow-hidden rounded-full bg-secondary">
                        <span className="block h-full" style={{ width: `${pct}%`, backgroundColor: CODING }} />
                        <span className="block h-full" style={{ width: `${100 - pct}%`, backgroundColor: OTHER }} />
                      </span>
                      <span className="w-9 shrink-0 text-right tabular-nums text-tertiary">{formatPct(pct)}</span>
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </Section>
    </div>
  );
}

function Centered({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-full items-center justify-center">
      <span className="text-sm text-tertiary">{children}</span>
    </div>
  );
}
