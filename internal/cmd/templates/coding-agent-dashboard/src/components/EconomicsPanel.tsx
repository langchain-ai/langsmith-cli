import { useMemo } from 'react';
import type { Run } from '../types';
import { entries, sumBy } from '../lib/aggregate';
import { cacheCreation, cacheRead, integrationOf, turnOf } from '../lib/normalize';
import { formatCost, formatTokens } from '../lib/format';
import { BarList, Legend, Section, StackedBar, type StackedRow } from './primitives';

const PROMPT = 'var(--brand-400)';
const COMPLETION = 'var(--purple-400)';
const READ = 'var(--green-500)';
const WRITE = 'var(--orange-500)';
const FRESH = 'var(--gray-400)';

interface Props {
  roots: Run[];
  colorOf: (integration: string) => string;
}

export function EconomicsPanel({ roots, colorOf }: Props) {
  const integrations = useMemo(() => entries(sumBy(roots, integrationOf, () => 1)).map((e) => e.label), [roots]);

  const tokenRows = useMemo<StackedRow[]>(
    () =>
      integrations.map((integ) => {
        const rs = roots.filter((r) => integrationOf(r) === integ);
        return {
          label: integ,
          segments: [
            { key: 'prompt', value: rs.reduce((a, r) => a + (r.prompt_tokens ?? 0), 0), color: PROMPT },
            { key: 'completion', value: rs.reduce((a, r) => a + (r.completion_tokens ?? 0), 0), color: COMPLETION },
          ],
        };
      }),
    [roots, integrations]
  );

  const cacheRows = useMemo<StackedRow[]>(
    () =>
      integrations.map((integ) => {
        const rs = roots.filter((r) => integrationOf(r) === integ);
        const read = rs.reduce((a, r) => a + cacheRead(r), 0);
        const write = rs.reduce((a, r) => a + cacheCreation(r), 0);
        const fresh = Math.max(rs.reduce((a, r) => a + (r.prompt_tokens ?? 0), 0) - read - write, 0);
        return {
          label: integ,
          segments: [
            { key: 'cache read', value: read, color: READ },
            { key: 'cache write', value: write, color: WRITE },
            { key: 'fresh input', value: fresh, color: FRESH },
          ],
        };
      }),
    [roots, integrations]
  );

  const costItems = useMemo(
    () => entries(sumBy(roots, integrationOf, (r) => r.total_cost ?? 0)).map((e) => ({ label: e.label, value: e.value, color: colorOf(e.label) })),
    [roots, colorOf]
  );

  const topTurns = useMemo(
    () => [...roots].filter((r) => (r.total_cost ?? 0) > 0).sort((a, b) => (b.total_cost ?? 0) - (a.total_cost ?? 0)).slice(0, 6),
    [roots]
  );

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Section title="Token usage" note="prompt vs completion per integration">
        <Legend items={[{ label: 'prompt', color: PROMPT }, { label: 'completion', color: COMPLETION }]} />
        <StackedBar rows={tokenRows} format={formatTokens} />
      </Section>

      <Section title="Cache efficiency" note="reused vs written vs fresh prompt tokens">
        <Legend items={[{ label: 'cache read', color: READ }, { label: 'cache write', color: WRITE }, { label: 'fresh input', color: FRESH }]} />
        <StackedBar rows={cacheRows} format={formatTokens} />
      </Section>

      <Section title="Cost by integration" note="rolled up onto root turns">
        <BarList items={costItems} format={formatCost} />
      </Section>

      <Section title="Most expensive turns" note="top root turns by cost">
        {topTurns.length === 0 ? (
          <span className="py-4 text-center text-xs text-tertiary">No cost data</span>
        ) : (
          <ul className="flex flex-col divide-y divide-secondary">
            {topTurns.map((r) => (
              <li key={r.id} className="flex items-center gap-3 py-2 text-sm">
                <span className="size-2.5 shrink-0 rounded-sm" style={{ backgroundColor: colorOf(integrationOf(r)) }} />
                <span className="min-w-0 flex-1 truncate text-primary" title={r.name ?? ''}>
                  {r.name ?? 'run'}
                  {turnOf(r) !== undefined && <span className="text-tertiary"> · turn {turnOf(r)}</span>}
                </span>
                <span className="shrink-0 tabular-nums text-tertiary">{formatTokens(r.total_tokens ?? 0)}</span>
                <span className="w-16 shrink-0 text-right tabular-nums text-secondary">{formatCost(r.total_cost ?? 0)}</span>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </div>
  );
}
