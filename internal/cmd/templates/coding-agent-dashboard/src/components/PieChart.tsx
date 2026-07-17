export interface Slice {
  label: string;
  value: number;
  color: string;
}

interface Props {
  slices: Slice[];
}

const CX = 100;
const CY = 100;
const R = 90;

function polar(angleDeg: number): [number, number] {
  const a = ((angleDeg - 90) * Math.PI) / 180;
  return [CX + R * Math.cos(a), CY + R * Math.sin(a)];
}

// SVG wedge from startDeg to endDeg, clockwise from 12 o'clock.
function wedgePath(startDeg: number, endDeg: number): string {
  const [sx, sy] = polar(startDeg);
  const [ex, ey] = polar(endDeg);
  const large = endDeg - startDeg > 180 ? 1 : 0;
  return `M ${CX} ${CY} L ${sx} ${sy} A ${R} ${R} 0 ${large} 1 ${ex} ${ey} Z`;
}

// Hand-drawn pie: one wedge per slice, plus a legend with counts and shares.
export function PieChart({ slices }: Props) {
  const total = slices.reduce((sum, s) => sum + s.value, 0);
  if (total === 0) return null;

  let cursor = 0;
  const wedges = slices.map((s) => {
    const start = cursor;
    const sweep = (s.value / total) * 360;
    cursor += sweep;
    return { ...s, start, end: cursor };
  });

  return (
    <div className="flex flex-wrap items-center gap-8">
      <svg viewBox="0 0 200 200" className="h-52 w-52 shrink-0" role="img" aria-label="Integration share">
        {wedges.length === 1 ? (
          <circle cx={CX} cy={CY} r={R} fill={wedges[0].color} />
        ) : (
          wedges.map((w) => (
            <path
              key={w.label}
              d={wedgePath(w.start, w.end)}
              fill={w.color}
              stroke="var(--bg-surface-level-1)"
              strokeWidth={1.5}
            />
          ))
        )}
      </svg>

      <ul className="flex min-w-[180px] flex-col gap-2">
        {wedges.map((w) => (
          <li key={w.label} className="flex items-center gap-2 text-sm">
            <span className="size-3 shrink-0 rounded-sm" style={{ backgroundColor: w.color }} />
            <span className="min-w-0 flex-1 truncate text-primary">{w.label}</span>
            <span className="shrink-0 text-tertiary">
              {w.value} ({Math.round((w.value / total) * 100)}%)
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
