import { useMemo, useState } from 'react';
import type { ExampleWithRuns, ExperimentView } from '../types';
import { histogram, SERIES_CAP, type HistogramBin, type RunMetric } from '../lib/metrics';
import { improvementDelta } from '../lib/delta';
import { Empty } from './primitives';

interface Props {
  examples: ExampleWithRuns[];
  experiments: ExperimentView[]; // baseline first, then comparisons
  metric: RunMetric;
}

const W = 260;
const H = 150;
const M = { top: 12, right: 8, bottom: 22, left: 8 };
const PW = W - M.left - M.right;
const PH = H - M.top - M.bottom;
const GAP = 2;

// Per comparison, a distribution of per-example improvement over the
// baseline for the selected metric — positive (green) bars are wins,
// negative (red) bars are regressions, so the shape of the win/loss spread
// is visible at a glance, not just its average.
export function DeltaHistogram({ examples, experiments, metric }: Props) {
  const baseline = experiments[0];
  const shown = experiments.slice(1, 1 + SERIES_CAP);
  const hidden = experiments.length - 1 - shown.length;
  if (!baseline) return null;
  if (shown.length === 0) return <Empty label="Pick a comparison experiment to see its delta distribution." />;

  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {shown.map((exp) => (
          <ComparisonHistogram key={exp.id} examples={examples} baselineId={baseline.id} comparison={exp} metric={metric} />
        ))}
      </div>
      {hidden > 0 && (
        <p className="text-xs text-tertiary">+{hidden} more comparison{hidden > 1 ? 's' : ''} not plotted.</p>
      )}
    </div>
  );
}

function ComparisonHistogram({
  examples,
  baselineId,
  comparison,
  metric,
}: {
  examples: ExampleWithRuns[];
  baselineId: string;
  comparison: ExperimentView;
  metric: RunMetric;
}) {
  const [hover, setHover] = useState<number | null>(null);

  const bins = useMemo(() => {
    const deltas: number[] = [];
    for (const ex of examples) {
      const base = metric.value(ex.runs.find((r) => r.session_id === baselineId));
      const val = metric.value(ex.runs.find((r) => r.session_id === comparison.id));
      const d = improvementDelta(base, val, metric.lowerIsBetter);
      if (d != null) deltas.push(d);
    }
    return histogram(deltas, 9);
  }, [examples, baselineId, comparison.id, metric]);

  const maxCount = bins.reduce((m, b) => Math.max(m, b.count), 0);

  return (
    <div className="rounded-lg border border-subtle p-3">
      <div className="mb-1 flex items-center gap-1.5 text-sm font-medium text-primary">
        <span className="h-2.5 w-2.5 shrink-0 rounded-[2px]" style={{ backgroundColor: comparison.color }} />
        <span className="truncate" title={comparison.name}>{comparison.name}</span>
      </div>
      {maxCount === 0 ? (
        <Empty label="No paired values." />
      ) : (
        <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label={`Delta distribution for ${comparison.name}`}>
          <line x1={M.left} x2={M.left + PW} y1={M.top + PH} y2={M.top + PH} stroke="var(--border-subtle)" strokeWidth={1} />
          {bins.map((b, i) => (
            <HistogramBar key={i} bin={b} index={i} maxCount={maxCount} hovered={hover === i} onHover={setHover} format={metric.format} />
          ))}
          <ZeroLine bins={bins} />
        </svg>
      )}
    </div>
  );
}

function HistogramBar({
  bin,
  index,
  maxCount,
  hovered,
  onHover,
  format,
}: {
  bin: HistogramBin;
  index: number;
  maxCount: number;
  hovered: boolean;
  onHover: (i: number | null) => void;
  format: (v: number | null) => string;
}) {
  const slot = PW / 9;
  const x = M.left + index * slot;
  const barW = Math.max(slot - GAP, 1);
  const barH = (bin.count / maxCount) * PH;
  const y = M.top + PH - barH;
  const mid = (bin.lo + bin.hi) / 2;
  const color = mid > 0 ? 'var(--bg-success-strong)' : mid < 0 ? 'var(--bg-error-strong)' : 'var(--border-strong)';

  return (
    <g onMouseEnter={() => onHover(index)} onMouseLeave={() => onHover(null)} className="cursor-pointer">
      {/* transparent full-height hit area so hovering a short bar is as easy as a tall one */}
      <rect x={x} y={M.top} width={barW} height={PH} fill="transparent" />
      <rect x={x} y={y} width={barW} height={Math.max(barH, bin.count > 0 ? 2 : 0)} rx={2} fill={color} opacity={hovered ? 1 : 0.85}>
        <title>{`${format(bin.lo)} to ${format(bin.hi)}: ${bin.count} example${bin.count === 1 ? '' : 's'}`}</title>
      </rect>
      {bin.count > 0 && (
        <text x={x + barW / 2} y={y - 3} textAnchor="middle" className="fill-current text-tertiary" style={{ fontSize: 9 }}>
          {bin.count}
        </text>
      )}
    </g>
  );
}

// Dashed reference line at delta = 0, when the observed range straddles it —
// a threshold, not a gridline, so dashing is the right signal here.
function ZeroLine({ bins }: { bins: HistogramBin[] }) {
  if (bins.length === 0) return null;
  const lo = bins[0].lo;
  const hi = bins[bins.length - 1].hi;
  if (lo > 0 || hi < 0) return null;
  const x = M.left + ((0 - lo) / (hi - lo)) * PW;
  return (
    <line x1={x} x2={x} y1={M.top} y2={M.top + PH} stroke="var(--text-tertiary)" strokeDasharray="3 3" opacity={0.6} />
  );
}
