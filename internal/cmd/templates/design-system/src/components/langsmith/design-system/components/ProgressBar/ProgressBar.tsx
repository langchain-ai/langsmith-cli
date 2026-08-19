import { forwardRef, type HTMLAttributes } from 'react';

import { cn } from '../../utils/cn';
import { Text } from '../Text';

type ProgressBarSize = 'sm' | 'md' | 'lg';
type ProgressBarColor = 'brand' | 'neutral' | 'success' | 'warning' | 'error';
type ProgressBarLabelPosition = 'top' | 'bottom';

interface ProgressBarBaseProps extends Omit<
  HTMLAttributes<HTMLDivElement>,
  | 'aria-label'
  | 'aria-labelledby'
  | 'aria-valuemax'
  | 'aria-valuemin'
  | 'aria-valuenow'
  | 'children'
  | 'color'
  | 'role'
> {
  /** Current progress value. */
  value: number;
  /** Minimum progress value. */
  min?: number;
  /** Maximum progress value. */
  max?: number;
  /** Bar height. Use `sm` in dense table layouts. */
  size?: ProgressBarSize;
  /** Semantic color for the completed portion of the bar. */
  color?: ProgressBarColor;
  /** Optional fixed-style label rendered above or beneath the bar. */
  label?: string;
  /** Vertical label placement. */
  labelPosition?: ProgressBarLabelPosition;
}

interface ProgressBarWithAriaLabel extends ProgressBarBaseProps {
  /** Accessible name describing what is progressing. */
  'aria-label': string;
  'aria-labelledby'?: never;
}

interface ProgressBarWithAriaLabelledBy extends ProgressBarBaseProps {
  /** ID of an element that names what is progressing. */
  'aria-labelledby': string;
  'aria-label'?: never;
}

type ProgressBarProps =
  | ProgressBarWithAriaLabel
  | ProgressBarWithAriaLabelledBy;

const SIZE_CLASSES: Record<ProgressBarSize, string> = {
  sm: 'h-1',
  md: 'h-2',
  lg: 'h-3',
};

const LABEL_VARIANTS: Record<ProgressBarSize, 'xs' | 'sm' | 'md'> = {
  sm: 'xs',
  md: 'sm',
  lg: 'md',
};

const COLOR_GRADIENTS: Record<ProgressBarColor, string> = {
  brand: 'linear-gradient(to left, var(--viz-brand-200), var(--viz-brand-75))',
  neutral:
    'linear-gradient(to left, var(--viz-neutral-150), var(--viz-neutral-75))',
  success:
    'linear-gradient(to left, var(--viz-green-200), var(--viz-green-100))',
  warning:
    'linear-gradient(to left, var(--viz-orange-150), var(--viz-orange-75))',
  error: 'linear-gradient(to left, var(--viz-red-300), var(--viz-red-200))',
};

function getFiniteValue(value: number, fallback: number) {
  return Number.isFinite(value) ? value : fallback;
}

function getNormalizedRange(min: number, max: number) {
  const normalizedMin = getFiniteValue(min, 0);
  const normalizedMax = getFiniteValue(max, normalizedMin + 100);

  return {
    min: normalizedMin,
    max: normalizedMax > normalizedMin ? normalizedMax : normalizedMin + 100,
  };
}

function clampValue(value: number, min: number, max: number) {
  return Math.min(Math.max(getFiniteValue(value, min), min), max);
}

/**
 * A single-value determinate horizontal progress indicator for measurable
 * completion or usage. Use `LinearProgress` for indeterminate loading states.
 */
const ProgressBar = forwardRef<HTMLDivElement, ProgressBarProps>(
  (
    {
      value,
      min = 0,
      max = 100,
      size = 'md',
      color = 'brand',
      label,
      labelPosition = 'bottom',
      className,
      ...props
    },
    ref
  ) => {
    const range = getNormalizedRange(min, max);
    const clampedValue = clampValue(value, range.min, range.max);
    const progressPercent =
      ((clampedValue - range.min) / (range.max - range.min)) * 100;

    const progressBar = (
      <div
        {...props}
        ref={ref}
        role="progressbar"
        aria-valuemin={range.min}
        aria-valuemax={range.max}
        aria-valuenow={clampedValue}
        className={cn(
          'relative w-full flex-none overflow-hidden rounded-xs bg-surface-level-3',
          SIZE_CLASSES[size],
          className
        )}
      >
        <div
          aria-hidden="true"
          className="h-full rounded-xs transition-[width] duration-normal ease-out motion-reduce:transition-none"
          style={{
            width: `${progressPercent}%`,
            backgroundImage: COLOR_GRADIENTS[color],
          }}
        />
      </div>
    );

    if (label === undefined) return progressBar;

    const progressLabel = (
      <Text as="span" variant={LABEL_VARIANTS[size]} color="tertiary">
        {label}
      </Text>
    );

    return (
      <div className="flex w-full flex-col gap-space-1">
        {labelPosition === 'top' && progressLabel}
        {progressBar}
        {labelPosition === 'bottom' && progressLabel}
      </div>
    );
  }
);

ProgressBar.displayName = 'ProgressBar';

export { ProgressBar };
export type {
  ProgressBarColor,
  ProgressBarLabelPosition,
  ProgressBarProps,
  ProgressBarSize,
};
