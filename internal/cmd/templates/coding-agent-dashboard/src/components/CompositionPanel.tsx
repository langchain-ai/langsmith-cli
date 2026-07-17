import { useMemo } from 'react';
import type { IntegrationStat, Run } from '../types';
import { countBy, entries } from '../lib/aggregate';
import { integrationOf, modelOf } from '../lib/normalize';
import { colorAt } from '../lib/palette';
import { formatInt } from '../lib/format';
import { PieChart, type Slice } from './PieChart';
import { IntegrationBreakdown } from './IntegrationBreakdown';
import { BarList, Section } from './primitives';

interface Props {
  roots: Run[];
  llm: Run[];
  colorOf: (integration: string) => string;
}

export function CompositionPanel({ roots, llm, colorOf }: Props) {
  const slices: Slice[] = useMemo(
    () => entries(countBy(roots, integrationOf)).map((e) => ({ label: e.label, value: e.value, color: colorOf(e.label) })),
    [roots, colorOf]
  );

  const models = useMemo(() => entries(countBy(llm, modelOf)), [llm]);
  const modelColor = useMemo(() => {
    const m = new Map<string, string>();
    models.forEach((e, i) => m.set(e.label, colorAt(i)));
    return m;
  }, [models]);

  const stats = useMemo<IntegrationStat[]>(() => {
    const map = new Map<string, { count: number; errors: number; models: Map<string, number> }>();
    for (const r of llm) {
      const integ = integrationOf(r);
      const entry = map.get(integ) ?? { count: 0, errors: 0, models: new Map() };
      entry.count++;
      if (r.error) entry.errors++;
      const mo = modelOf(r);
      entry.models.set(mo, (entry.models.get(mo) ?? 0) + 1);
      map.set(integ, entry);
    }
    return [...map.entries()]
      .map(([integration, e]) => ({
        integration,
        count: e.count,
        errors: e.errors,
        models: [...e.models.entries()].map(([model, count]) => ({ model, count })).sort((a, b) => b.count - a.count),
      }))
      .sort((a, b) => b.count - a.count);
  }, [llm]);

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Section title="Integration share" note={`${roots.length} root turns`}>
        <PieChart slices={slices} />
      </Section>
      <Section title="Models" note={`${llm.length} LLM calls`}>
        <BarList
          items={models.map((e) => ({ label: e.label, value: e.value, color: modelColor.get(e.label) }))}
          format={formatInt}
          labelWidth="w-40"
        />
      </Section>
      <div className="lg:col-span-2">
        <Section title="Models by integration" note="ls_model_name grouped under ls_integration (from LLM child runs)">
          <IntegrationBreakdown stats={stats} colorOf={colorOf} />
        </Section>
      </div>
    </div>
  );
}
