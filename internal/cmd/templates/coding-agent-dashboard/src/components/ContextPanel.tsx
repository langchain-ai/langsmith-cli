import { useMemo } from 'react';
import type { Run } from '../types';
import { countBy, entries } from '../lib/aggregate';
import { branchOf, integrationOf, integrationVersionOf, repoOf, sdkVersionOf, threadOf, userOf } from '../lib/normalize';
import { formatCost, formatInt, formatTokens } from '../lib/format';
import { BarList, Section } from './primitives';

interface Props {
  roots: Run[];
  colorOf: (integration: string) => string;
}

export function ContextPanel({ roots, colorOf }: Props) {
  const threads = useMemo(() => {
    const map = new Map<string, { turns: number; tokens: number; cost: number; integ: string }>();
    for (const r of roots) {
      const t = threadOf(r);
      if (!t) continue;
      const e = map.get(t) ?? { turns: 0, tokens: 0, cost: 0, integ: integrationOf(r) };
      e.turns++;
      e.tokens += r.total_tokens ?? 0;
      e.cost += r.total_cost ?? 0;
      map.set(t, e);
    }
    return [...map.entries()].map(([id, e]) => ({ id, ...e })).sort((a, b) => b.turns - a.turns).slice(0, 8);
  }, [roots]);

  const repos = useMemo(() => entries(countBy(roots, repoOf)), [roots]);
  const branches = useMemo(() => entries(countBy(roots, branchOf)), [roots]);
  const versions = useMemo(() => entries(countBy(roots, integrationVersionOf)), [roots]);
  const sdks = useMemo(() => entries(countBy(roots, sdkVersionOf)), [roots]);
  const users = useMemo(() => entries(countBy(roots, userOf)), [roots]);

  return (
    <div className="flex flex-col gap-4">
      <Section title="Threads" note="conversations grouped by thread_id">
        {threads.length === 0 ? (
          <span className="py-4 text-center text-xs text-tertiary">No thread data</span>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs text-tertiary">
                <th className="pb-1 font-medium">Thread</th>
                <th className="pb-1 text-right font-medium">Turns</th>
                <th className="pb-1 text-right font-medium">Tokens</th>
                <th className="pb-1 text-right font-medium">Cost</th>
              </tr>
            </thead>
            <tbody>
              {threads.map((t) => (
                <tr key={t.id} className="border-t border-secondary">
                  <td className="flex items-center gap-2 py-1.5 text-primary">
                    <span className="size-2.5 shrink-0 rounded-sm" style={{ backgroundColor: colorOf(t.integ) }} />
                    <span className="font-mono text-xs" title={t.id}>{t.id.slice(0, 8)}</span>
                  </td>
                  <td className="py-1.5 text-right tabular-nums text-secondary">{t.turns}</td>
                  <td className="py-1.5 text-right tabular-nums text-secondary">{formatTokens(t.tokens)}</td>
                  <td className="py-1.5 text-right tabular-nums text-secondary">{formatCost(t.cost)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Section>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Section title="Repositories" note="repository_name">
          <BarList items={repos.map((e) => ({ label: e.label, value: e.value }))} format={formatInt} />
        </Section>
        <Section title="Branches" note="git_branch">
          <BarList items={branches.map((e) => ({ label: e.label, value: e.value }))} color="var(--purple-400)" format={formatInt} />
        </Section>
        <Section title="Versions" note="integration + SDK versions in use">
          <BarList items={[...versions, ...sdks].map((e) => ({ label: e.label, value: e.value }))} color="var(--green-500)" format={formatInt} labelWidth="w-40" />
        </Section>
        <Section title="Contributors" note="user / local_username">
          <BarList items={users.map((e) => ({ label: e.label, value: e.value }))} color="var(--orange-500)" format={formatInt} />
        </Section>
      </div>
    </div>
  );
}
