import { useMemo, useState } from 'react';
import type { ExampleWithRuns, ExperimentView } from '../types';
import { SCATTER_SERIES_CAP, type RunMetric } from '../lib/metrics';

interface Props {
  examples: ExampleWithRuns[];
  experiments: ExperimentView[]; // baseline first, then comparisons
  metric: RunMetric;
}

interface Point {
  cx: number;
  cy: number;
  xv: number;
  yv: number;
  name: string;
  color: string;
  letter: string;
}

const W = 520;
const H = 360;
const M = { top: 16, right: 16, bottom: 44, left: 64 };
const PW = W - M.left - M.right;
const PH = H - M.top - M.bottom;

// Baseline (x) vs each comparison (y) for one metric; diagonal is parity.
export function ScatterPlot({ examples, experiments, metric }: Props) {
  const [hover, setHover] = useState<Point | null>(null);
  const baseline = experiments[0];
  const shown = experiments.slice(1, 1 + SCATTER_SERIES_CAP);
  const hidden = experiments.length - 1 - shown.length;

  const { points, lo, hi } = useMemo(() => {
    const raw: Omit<Point, 'cx' | 'cy'>[] = [];
    const vals: number[] = [];
    for (const exp of shown) {
      for (const ex of examples) {
        const xv = metric.value(ex.runs.find((r) => r.session_id === baseline?.id));
        const yv = metric.value(ex.runs.find((r) => r.session_id === exp.id));
        if (xv == null || yv == null) continue;
        raw.push({ xv, yv, name: ex.name || ex.id, color: exp.color, letter: exp.letter });
        vals.push(xv, yv);
      }
    }
    let lo = vals.length ? Math.min(...vals) : 0;
    let hi = vals.length ? Math.max(...vals) : 1;
    if (hi === lo) hi = lo + 1;
    const pad = (hi - lo) * 0.08;
    lo -= pad;
    hi += pad;
    const points = raw.map((r) => ({ ...r, cx: scaleX(r.xv, lo, hi), cy: scaleY(r.yv, lo, hi) }));
    return { points, lo, hi };
  }, [examples, shown, baseline, metric]);

  if (!baseline) return null;
  if (points.length === 0) {
    return (
      <div className="rounded-lg border border-secondary p-6 text-sm text-tertiary">
        No paired values for {metric.label} in this selection.
      </div>
    );
  }

  const ticks = [lo, (lo + hi) / 2, hi];
  const betterHint = metric.lowerIsBetter
    ? 'points below the line beat the baseline'
    : 'points above the line beat the baseline';

  return (
    <div className="rounded-lg border border-secondary p-4">
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label={`Baseline vs comparison ${metric.label}`}>
        {ticks.map((t) => (
          <g key={`gx-${t}`}>
            <line
              x1={scaleX(t, lo, hi)}
              x2={scaleX(t, lo, hi)}
              y1={M.top}
              y2={M.top + PH}
              stroke="var(--border-subtle)"
            />
            <line
              x1={M.left}
              x2={M.left + PW}
              y1={scaleY(t, lo, hi)}
              y2={scaleY(t, lo, hi)}
              stroke="var(--border-subtle)"
            />
            <text
              x={scaleX(t, lo, hi)}
              y={M.top + PH + 16}
              textAnchor="middle"
              className="fill-current text-tertiary"
              style={{ fontSize: 10 }}
            >
              {metric.format(t)}
            </text>
            <text
              x={M.left - 8}
              y={scaleY(t, lo, hi) + 3}
              textAnchor="end"
              className="fill-current text-tertiary"
              style={{ fontSize: 10 }}
            >
              {metric.format(t)}
            </text>
          </g>
        ))}

        <line
          x1={M.left}
          y1={M.top + PH}
          x2={M.left + PW}
          y2={M.top}
          stroke="var(--text-tertiary)"
          strokeDasharray="4 4"
          opacity={0.7}
        />

        {points.map((p, i) => (
          <circle
            key={i}
            cx={p.cx}
            cy={p.cy}
            r={hover === p ? 6 : 4}
            fill={p.color}
            stroke="var(--bg-surface-level-1)"
            strokeWidth={1.5}
            onMouseEnter={() => setHover(p)}
            onMouseLeave={() => setHover(null)}
          >
            <title>{`${p.letter} · ${p.name}\nbaseline ${metric.format(p.xv)} → ${metric.format(p.yv)}`}</title>
          </circle>
        ))}

        <text
          x={M.left + PW / 2}
          y={H - 4}
          textAnchor="middle"
          className="fill-current text-tertiary"
          style={{ fontSize: 11 }}
        >
          baseline {metric.label}
        </text>
        <text
          transform={`translate(14 ${M.top + PH / 2}) rotate(-90)`}
          textAnchor="middle"
          className="fill-current text-tertiary"
          style={{ fontSize: 11 }}
        >
          comparison {metric.label}
        </text>

        {hover && <Tooltip point={hover} metric={metric} />}
      </svg>

      <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-tertiary">
        {shown.map((exp) => (
          <span key={exp.id} className="inline-flex items-center gap-1.5 text-secondary">
            <span className="h-2.5 w-2.5 rounded-[2px]" style={{ backgroundColor: exp.color }} />
            {exp.letter} · <span className="truncate max-w-[180px]" title={exp.name}>{exp.name}</span>
          </span>
        ))}
        <span>dashed line = parity; {betterHint}.</span>
        {hidden > 0 && <span>+{hidden} more comparison{hidden > 1 ? 's' : ''} not plotted.</span>}
      </div>
    </div>
  );
}

function Tooltip({ point, metric }: { point: Point; metric: RunMetric }) {
  const boxW = 168;
  const x = Math.min(Math.max(point.cx + 10, M.left), W - M.right - boxW);
  const y = Math.min(Math.max(point.cy - 34, M.top), M.top + PH - 34);
  return (
    <g pointerEvents="none">
      <rect x={x} y={y} width={boxW} height={30} rx={4} fill="var(--bg-elevated)" stroke="var(--border-default)" />
      <text x={x + 8} y={y + 13} className="fill-current text-primary" style={{ fontSize: 10 }}>
        {point.letter} · {truncate(point.name)}
      </text>
      <text x={x + 8} y={y + 25} className="fill-current text-tertiary" style={{ fontSize: 10 }}>
        {metric.format(point.xv)} → {metric.format(point.yv)}
      </text>
    </g>
  );
}

function scaleX(v: number, lo: number, hi: number): number {
  return M.left + ((v - lo) / (hi - lo)) * PW;
}

function scaleY(v: number, lo: number, hi: number): number {
  return M.top + PH - ((v - lo) / (hi - lo)) * PH;
}

function truncate(s: string, max = 22): string {
  return s.length > max ? s.slice(0, max) + '…' : s;
}
