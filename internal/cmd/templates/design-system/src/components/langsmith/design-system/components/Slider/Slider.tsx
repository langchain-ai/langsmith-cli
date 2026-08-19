import * as SliderPrimitive from '@radix-ui/react-slider';

import { cn } from '../../utils/cn';
import { Text } from '../Text';

export interface SliderStepLabel {
  value: number;
  label?: string;
}

export interface SliderProps {
  /** Current value. Pass a number for a single thumb, number[] for a range slider. */
  value?: number | number[];
  /** Default value (uncontrolled). */
  defaultValue?: number | number[];
  /** Called continuously as the value changes. */
  onChange?: (value: number | number[]) => void;
  /** Called when the user finishes dragging (mouse/touch up or keyboard interaction ends). */
  onChangeCommitted?: (value: number | number[]) => void;
  min?: number;
  max?: number;
  step?: number;
  stepLabels?: SliderStepLabel[];
  disabled?: boolean;
  className?: string;
  'aria-label'?: string;
  'aria-labelledby'?: string;
  'data-testid'?: string;
}

function toArray(v: number | number[] | undefined): number[] | undefined {
  if (v === undefined) return undefined;
  return Array.isArray(v) ? v : [v];
}

function fromArray(v: number[], wasArray: boolean): number | number[] {
  return wasArray ? v : v[0];
}

/**
 * Slider built on the Radix UI Slider primitive. Supports single-thumb and
 * range (multi-thumb) modes, optional labeled steps, and disabled state.
 *
 * When stepLabels are present, the Root's hit area extends to cover the label
 * row — clicking a label moves the thumb to that X position.
 */
export const Slider = ({
  value,
  defaultValue,
  onChange,
  onChangeCommitted,
  min = 0,
  max = 100,
  step = 1,
  stepLabels,
  disabled = false,
  className,
  'aria-label': ariaLabel,
  'aria-labelledby': ariaLabelledby,
  'data-testid': testId,
}: SliderProps) => {
  const isRange = Array.isArray(value ?? defaultValue);
  const labeledSteps = stepLabels?.filter((s) => s.label);
  const hasLabels = !!labeledSteps && labeledSteps.length > 0;

  return (
    <SliderPrimitive.Root
      className={cn(
        'relative flex w-full touch-none select-none items-center',
        hasLabels && 'pb-7',
        className
      )}
      value={toArray(value)}
      defaultValue={toArray(defaultValue)}
      min={min}
      max={max}
      step={step}
      disabled={disabled}
      onValueChange={(v) => onChange?.(fromArray(v, isRange))}
      onValueCommit={(v) => onChangeCommitted?.(fromArray(v, isRange))}
      aria-label={ariaLabel}
      aria-labelledby={ariaLabelledby}
      data-testid={testId}
    >
      <SliderPrimitive.Track className="relative h-1.5 w-full grow overflow-hidden rounded-full bg-surface-level-4">
        <SliderPrimitive.Range
          className={cn(
            'absolute h-full',
            disabled ? 'data-[disabled]:bg-brand-subtle' : 'bg-control-active'
          )}
        />
      </SliderPrimitive.Track>

      {(toArray(value) ?? toArray(defaultValue) ?? [0]).map((_, i) => (
        <SliderPrimitive.Thumb
          key={i}
          className={cn(
            'block h-4 w-4 rounded-full border-2 shadow transition',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-300 focus-visible:ring-offset-1',
            disabled
              ? 'border-muted bg-surface-level-3'
              : 'cursor-pointer border-brand-300 bg-control-thumb hover:border-brand-400 dark:border-brand-500 dark:hover:border-brand-600'
          )}
        />
      ))}

      {hasLabels && (
        <div aria-hidden className="absolute bottom-2 w-full cursor-pointer">
          {labeledSteps.map((s) => (
            <Text
              key={s.value}
              as="span"
              variant="xs"
              className="absolute -translate-x-1/2 truncate text-secondary"
              // Left as non-tailwind on purpose for better readability on label positioning
              style={{
                left: `${((s.value - min) / (max - min)) * 100}%`,
                maxWidth: `${100 / labeledSteps.length}%`,
              }}
            >
              {s.label}
            </Text>
          ))}
        </div>
      )}
    </SliderPrimitive.Root>
  );
};
