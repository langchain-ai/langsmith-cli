import type { ExampleWithRuns, ExperimentRun, Experiment } from '../types';
import { latencyMs, scoreFor, verdict, verdictClass } from '../lib/delta';

interface Props {
  examples: ExampleWithRuns[];
  experiments: Experiment[]; // baseline first, then comparisons
  primaryKey: string | undefined; // feedback key shown per cell
  lowerIsBetter: boolean; // direction for primaryKey
}

function truncate(value: unknown, max = 80): string {
  if (value == null) return '—';
  const s = typeof value === 'string' ? value : JSON.stringify(value);
  return s.length > max ? s.slice(0, max) + '…' : s;
}

function runFor(example: ExampleWithRuns, sessionId: string): ExperimentRun | undefined {
  return example.runs.find((r) => r.session_id === sessionId);
}

// One row per example; per experiment the output, latency, and score —
// colored vs the baseline run for that example.
export function ExampleTable({ examples, experiments, primaryKey, lowerIsBetter }: Props) {
  const baselineId = experiments[0]?.id;

  return (
    <div className="overflow-x-auto rounded-lg border border-secondary">
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="border-b border-secondary bg-surface-level-2">
            <th className="px-3 py-2 text-left font-medium text-tertiary">Input</th>
            {experiments.map((x, i) => (
              <th key={x.id} colSpan={2} className="px-3 py-2 text-left font-medium text-primary">
                {x.name}
                {i === 0 && <span className="ml-1 text-xs text-tertiary">(baseline)</span>}
              </th>
            ))}
          </tr>
          <tr className="border-b border-secondary bg-surface-level-2 text-xs text-tertiary">
            <th className="px-3 py-1.5 text-left font-normal" />
            {experiments.map((x) => (
              <th key={x.id} colSpan={2} className="px-3 py-1.5 text-left font-normal">
                output · latency / {primaryKey ?? 'score'}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {examples.map((ex) => {
            const baseRun = baselineId ? runFor(ex, baselineId) : undefined;
            const baseLatency = latencyMs(baseRun);
            const baseScore = primaryKey ? scoreFor(baseRun, primaryKey) : null;
            return (
              <tr key={ex.id} className="border-b border-secondary align-top last:border-0">
                <td className="max-w-[240px] truncate px-3 py-2 text-secondary" title={truncate(ex.inputs, 500)}>
                  {truncate(ex.inputs)}
                </td>
                {experiments.map((x, i) => {
                  const run = runFor(ex, x.id);
                  const latency = latencyMs(run);
                  const score = primaryKey ? scoreFor(run, primaryKey) : null;
                  const isBase = i === 0;
                  const latCls = isBase
                    ? 'text-tertiary'
                    : verdictClass(verdict(baseLatency, latency, true));
                  const scoreCls = isBase
                    ? 'text-tertiary'
                    : verdictClass(verdict(baseScore, score, lowerIsBetter));
                  return (
                    <FragmentCells
                      key={x.id}
                      output={truncate(run?.outputs ?? run?.outputs_preview)}
                      latency={latency}
                      latCls={latCls}
                      score={score}
                      scoreCls={scoreCls}
                    />
                  );
                })}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function FragmentCells(props: {
  output: string;
  latency: number | null;
  latCls: string;
  score: number | null;
  scoreCls: string;
}) {
  return (
    <>
      <td className="max-w-[240px] truncate px-3 py-2 text-primary" title={props.output}>
        {props.output}
      </td>
      <td className="whitespace-nowrap px-3 py-2">
        <span className={props.latCls}>{props.latency == null ? '—' : `${Math.round(props.latency)} ms`}</span>
        <span className="text-tertiary"> / </span>
        <span className={props.scoreCls}>{props.score == null ? '—' : props.score.toFixed(3)}</span>
      </td>
    </>
  );
}
