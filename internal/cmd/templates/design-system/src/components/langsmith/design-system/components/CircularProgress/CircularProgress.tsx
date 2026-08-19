import { cn } from '../../utils/cn';

export type ProgressSegmentVariant = 'brand' | 'success' | 'error' | 'warning';

export interface ProgressSegment {
  /** Semantic color applied to the segment. */
  variant: ProgressSegmentVariant;
  percentage: number;
}

const segmentVariantClasses: Record<ProgressSegmentVariant, string> = {
  brand: 'text-icon-brand',
  success: 'text-success-secondary',
  error: 'text-error-secondary',
  warning: 'text-warning-secondary',
};

function normalizePercentage(percentage: number, max: number) {
  if (!Number.isFinite(percentage)) return 0;
  return Math.min(Math.max(percentage, 0), max);
}

/**
 * CircularProgress renders a circular progress ring.
 *
 * Two modes:
 * - **Determinate value** — pass `value` (0–1) for a single-color ring (e.g. a
 *   sampling rate). Renders with `role="progressbar"` for accessibility.
 * - **Segments** — pass `segments` for a multi-color ring, each with its own
 *   color and fraction of the circle.
 *
 * @example
 * ```tsx
 * // Determinate value (single ring)
 * <CircularProgress value={0.73} size={16} strokeWidth={2} aria-label="Sampling rate" />
 *
 * // Multi-segment
 * <CircularProgress
 *   segments={[
 *     { variant: 'success', percentage: 0.3 },
 *     { variant: 'error', percentage: 0.1 },
 *   ]}
 *   size={120}
 *   strokeWidth={8}
 * />
 * ```
 */
interface CircularProgressBaseProps {
  size?: number;
  strokeWidth?: number;
  className?: string;
  testId?: string;
}

interface CircularProgressValueProps extends CircularProgressBaseProps {
  /** Determinate progress, 0–1. */
  value: number;
  segments?: never;
  /** Accessible name describing the determinate value. */
  'aria-label': string;
}

interface CircularProgressSegmentsProps extends CircularProgressBaseProps {
  /** Multi-segment progress. */
  segments: ProgressSegment[];
  value?: never;
  /** Accessible summary. Omit only when the ring is decorative. */
  'aria-label'?: string;
}

export type CircularProgressProps =
  | CircularProgressValueProps
  | CircularProgressSegmentsProps;

export function CircularProgress({
  value,
  segments,
  size = 100,
  strokeWidth = 10,
  className,
  testId,
  'aria-label': ariaLabel,
}: CircularProgressProps) {
  // Determinate single-value mode collapses to a one-segment ring.
  const isValueMode = value != null;
  const normalizedValue = normalizePercentage(value ?? 0, 1);
  const resolvedSegments: ProgressSegment[] =
    segments ??
    (value != null ? [{ variant: 'brand', percentage: normalizedValue }] : []);

  // Calculate dimensions
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const center = size / 2;

  // Calculate segment paths
  let currentAngle = 0;

  return (
    <div
      className={cn('relative inline-block', className)}
      data-testid={testId}
      style={{ width: size, height: size }}
      {...(isValueMode
        ? {
            role: 'progressbar',
            'aria-label': ariaLabel,
            'aria-valuenow': Math.round(normalizedValue * 100),
            'aria-valuemin': 0,
            'aria-valuemax': 100,
          }
        : ariaLabel
          ? { role: 'img', 'aria-label': ariaLabel }
          : {})}
    >
      {/* Background circle */}
      <svg
        aria-hidden="true"
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
      >
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          className="text-quaternary opacity-20"
        />

        {/* Segments */}
        {resolvedSegments.map((segment, index) => {
          const strokeDasharray = circumference;
          const remainingPercentage = Math.max(1 - currentAngle, 0);
          const normalizedPercentage = normalizePercentage(
            segment.percentage,
            remainingPercentage
          );
          const strokeDashoffset = circumference * (1 - normalizedPercentage);

          // Calculate rotation for this segment
          const rotation = currentAngle * 360 - 90; // -90 to start at top
          currentAngle += normalizedPercentage;

          return (
            <circle
              key={index}
              cx={center}
              cy={center}
              r={radius}
              fill="none"
              stroke="currentColor"
              strokeWidth={strokeWidth}
              strokeLinecap={isValueMode ? 'round' : 'butt'}
              strokeDasharray={strokeDasharray}
              strokeDashoffset={strokeDashoffset}
              className={cn(
                segmentVariantClasses[segment.variant],
                'transition-[stroke-dashoffset,transform] duration-slower ease-in-out motion-reduce:transition-none'
              )}
              style={{
                transform: `rotate(${rotation}deg)`,
                transformOrigin: `${center}px ${center}px`,
              }}
            />
          );
        })}
      </svg>
    </div>
  );
}
