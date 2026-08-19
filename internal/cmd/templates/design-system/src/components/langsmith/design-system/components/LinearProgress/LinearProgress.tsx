import { forwardRef, type CSSProperties, type HTMLAttributes } from 'react';

import { cn } from '../../utils/cn';

interface LinearProgressStyle extends CSSProperties {
  '--linear-progress-thickness'?: string;
}

interface LinearProgressProps extends Omit<
  HTMLAttributes<HTMLDivElement>,
  'children' | 'color'
> {
  /** Current progress value. Passing a value renders a determinate bar. */
  value?: number;
  /** Minimum value for determinate progress. */
  min?: number;
  /** Maximum value for determinate progress. */
  max?: number;
  /** Progress mode. Defaults to indeterminate unless a value is provided. */
  variant?: 'indeterminate' | 'determinate';
  /** Custom bar height in pixels. Defaults to 2px. */
  thicknessPx?: number;
}

function clampValue(value: number, min: number, max: number) {
  if (max <= min) return min;
  return Math.min(Math.max(value, min), max);
}

function getPercent(value: number, min: number, max: number) {
  if (max <= min) return 0;
  return ((clampValue(value, min, max) - min) / (max - min)) * 100;
}

function getFiniteValue(value: number, fallback: number) {
  return Number.isFinite(value) ? value : fallback;
}

const LinearProgress = forwardRef<HTMLDivElement, LinearProgressProps>(
  (
    {
      value,
      min = 0,
      max = 100,
      variant,
      thicknessPx,
      className,
      style,
      ...props
    },
    ref
  ) => {
    const isDeterminate = variant === 'determinate' || value !== undefined;
    const safeValue = getFiniteValue(value ?? min, min);
    const clampedValue = clampValue(safeValue, min, max);
    const progressPercent = getPercent(safeValue, min, max);
    const rootStyle: LinearProgressStyle = { ...style };

    if (thicknessPx !== undefined) {
      rootStyle['--linear-progress-thickness'] = `${thicknessPx}px`;
    }

    return (
      <div
        {...props}
        ref={ref}
        role="progressbar"
        aria-valuemin={isDeterminate ? min : undefined}
        aria-valuemax={isDeterminate ? max : undefined}
        aria-valuenow={isDeterminate ? Math.round(clampedValue) : undefined}
        className={cn(
          'relative w-full flex-none overflow-hidden',
          thicknessPx === undefined
            ? 'h-0.5'
            : 'h-[var(--linear-progress-thickness)]',
          'bg-brand-subtle',
          'rounded-none',
          className
        )}
        style={rootStyle}
      >
        <div
          className={cn(
            'absolute inset-y-0 left-0',
            'bg-brand',
            'rounded-none',
            isDeterminate
              ? 'transition-[width] duration-200 ease-out'
              : 'w-[12.5%] animate-linear-progress-indeterminate motion-reduce:animate-none'
          )}
          style={isDeterminate ? { width: `${progressPercent}%` } : undefined}
        />
      </div>
    );
  }
);

LinearProgress.displayName = 'LinearProgress';

export { LinearProgress };
export type { LinearProgressProps };
