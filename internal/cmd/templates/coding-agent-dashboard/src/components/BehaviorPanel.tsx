import { useMemo } from 'react';
import type { Run } from '../types';
import { countBy, entries } from '../lib/aggregate';
import { stopReasonOf, subagentTypeOf, toolNameOf } from '../lib/normalize';
import { formatInt } from '../lib/format';
import { BarList, Section } from './primitives';

const TOOLS = 'var(--brand-400)';
const SUBAGENTS = 'var(--purple-400)';
const NORMAL = 'var(--green-500)';
const ERROR = 'var(--bg-error-strong)';

interface Props {
  tool: Run[];
  subagents: Run[];
  llm: Run[];
}

export function BehaviorPanel({ tool, subagents, llm }: Props) {
  const tools = useMemo(() => entries(countBy(tool, toolNameOf)).slice(0, 12), [tool]);
  const subs = useMemo(() => entries(countBy(subagents, subagentTypeOf)), [subagents]);
  const stops = useMemo(
    () => entries(countBy(llm, stopReasonOf)).map((e) => ({ label: e.label, value: e.value, color: /error|fail/i.test(e.label) ? ERROR : NORMAL })),
    [llm]
  );

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
      <Section title="Tool usage" note={`${tool.length} tool calls`}>
        <BarList items={tools.map((e) => ({ label: e.label, value: e.value }))} color={TOOLS} format={formatInt} labelWidth="w-28" />
      </Section>
      <Section title="Subagents" note={`${subagents.length} subagent runs · ls_subagent_type`}>
        <BarList items={subs.map((e) => ({ label: e.label, value: e.value }))} color={SUBAGENTS} format={formatInt} labelWidth="w-28" />
      </Section>
      <Section title="Stop reasons" note="LLM run outcomes">
        <BarList items={stops} format={formatInt} labelWidth="w-28" />
      </Section>
    </div>
  );
}
