import { useMemo } from 'react';
import type { Run } from '../types';
import { countBy, dayKey, entries, percentile } from '../lib/aggregate';
import { durationMs, integrationOf } from '../lib/normalize';
import { formatDuration, formatInt, formatPct } from '../lib/format';
import { BarList, ColumnChart, Legend, Section, type ColumnDay } from './primitives';

interface Props {
  roots: Run[];
  colorOf: (integration: string) => string;
}

export function ActivityPanel({ roots, colorOf }: Props) {
  const integrations = useMemo(() => entries(countBy(roots, integrationOf)).map((e) => e.label), [roots]);

  const days = useMemo<ColumnDay[]>(() => {
    const byDay = new Map<string, Run[]>();
    for (const r of roots) {
      const k = dayKey(r);
      if (!k) continue;
      (byDay.get(k) ?? byDay.set(k, []).get(k)!).push(r);
    }
    return [...byDay.keys()].sort().map((k) => {
      const rs = byDay.get(k)!;
      return {
        label: k.slice(5),
        segments: integrations.map((integ) => ({
          key: integ,
          value: rs.filter((r) => integrationOf(r) === integ).length,
          color: colorOf(integ),
        })),
      };
    });
  }, [roots, integrations, colorOf]);

  const latency = useMemo(
    () =>
      integrations
        .map((integ) => {
          const ds = roots.filter((r) => integrationOf(r) === integ).map(durationMs).filter((d): d is number => d != null);
          const avg = ds.length ? ds.reduce((a, b) => a + b, 0) / ds.length : 0;
          return { integ, n: ds.length, p50: percentile(ds, 50), p95: percentile(ds, 95), avg };
        })
        .filter((x) => x.n > 0),
    [roots, integrations]
  );

  const errorItems = useMemo(
    () =>
      integrations
        .map((integ) => {
          const rs = roots.filter((r) => integrationOf(r) === integ);
          const errs = rs.filter((r) => r.error).length;
          return { label: integ, value: errs, color: 'var(--bg-error-strong)', hint: `${integ}: ${errs}/${rs.length} (${formatPct(rs.length ? (errs / rs.length) * 100 : 0)})` };
        })
        .sort((a, b) => b.value - a.value),
    [roots, integrations]
  );

  return (
    <div className="flex flex-col gap-4">
      <Section title="Activity over time" note="root turns per day, stacked by integration">
        <Legend items={integrations.map((i) => ({ label: i, color: colorOf(i) }))} />
        <ColumnChart days={days} format={formatInt} />
      </Section>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Section title="Latency" note="root turn duration">
          {latency.length === 0 ? (
            <span className="py-4 text-center text-xs text-tertiary">No timing data</span>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs text-tertiary">
                  <th className="pb-1 font-medium">Integration</th>
                  <th className="pb-1 text-right font-medium">p50</th>
                  <th className="pb-1 text-right font-medium">p95</th>
                  <th className="pb-1 text-right font-medium">avg</th>
                </tr>
              </thead>
              <tbody>
                {latency.map((l) => (
                  <tr key={l.integ} className="border-t border-secondary">
                    <td className="flex items-center gap-2 py-1.5 text-primary">
                      <span className="size-2.5 shrink-0 rounded-sm" style={{ backgroundColor: colorOf(l.integ) }} />
                      {l.integ}
                    </td>
                    <td className="py-1.5 text-right tabular-nums text-secondary">{formatDuration(l.p50)}</td>
                    <td className="py-1.5 text-right tabular-nums text-secondary">{formatDuration(l.p95)}</td>
                    <td className="py-1.5 text-right tabular-nums text-secondary">{formatDuration(l.avg)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Section>

        <Section title="Errors by integration" note="failed root turns">
          <BarList items={errorItems} format={formatInt} />
        </Section>
      </div>
    </div>
  );
}
