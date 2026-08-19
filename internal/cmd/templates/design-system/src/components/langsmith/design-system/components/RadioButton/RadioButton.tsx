import { forwardRef, useId } from 'react';

import * as RadioGroupPrimitive from '@radix-ui/react-radio-group';

import { cn } from '../../utils/cn';
import { Text } from '../Text';
import {
  radioButtonBaseStyles,
  radioButtonDotSizeStyles,
  radioButtonFocusStyles,
  radioButtonIndicatorStyles,
  radioButtonRootStateStyles,
  radioButtonSizeStyles,
} from './constants';

/**
 * Base props shared by all RadioButton variants.
 */
interface RadioButtonBaseProps {
  /** Current value of this radio button */
  value: string;
  /** Additional CSS classes for the container */
  className?: string;
  /** Whether the radio button is disabled */
  disabled?: boolean;
  /** Size of the radio button */
  size?: 'sm' | 'md';
  /** Test ID for testing purposes */
  'data-testid'?: string;
}

/**
 * Props for RadioButton with a visible label.
 */
interface RadioButtonWithLabelProps extends RadioButtonBaseProps {
  /** Visible label text or React node displayed next to the radio button */
  label: React.ReactNode;
  /** Additional CSS classes for the label */
  labelClassName?: string;
  'aria-label'?: never;
  'aria-labelledby'?: never;
}

/**
 * Props for RadioButton with aria-label (no visible label).
 */
interface RadioButtonWithAriaLabelProps extends RadioButtonBaseProps {
  /** Accessible label for screen readers when no visible label is present */
  'aria-label': string;
  label?: never;
  labelClassName?: never;
  'aria-labelledby'?: never;
}

/**
 * Props for RadioButton with aria-labelledby (labeled by external element).
 */
interface RadioButtonWithAriaLabelledByProps extends RadioButtonBaseProps {
  /** ID of an element that labels this radio button */
  'aria-labelledby': string;
  label?: never;
  labelClassName?: never;
  'aria-label'?: never;
}

/**
 * RadioButton component props.
 *
 * For accessibility compliance (WCAG 2.1), radio buttons must have an accessible name.
 * This is enforced at the type level by requiring one of:
 * - `label`: A visible label displayed next to the radio button
 * - `aria-label`: An accessible label for screen readers (when no visible label)
 * - `aria-labelledby`: Reference to an external labeling element
 *
 * @see https://www.w3.org/WAI/tutorials/forms/labels/
 * @see https://www.w3.org/WAI/ARIA/apg/patterns/radio/
 */
export type RadioButtonProps =
  | RadioButtonWithLabelProps
  | RadioButtonWithAriaLabelProps
  | RadioButtonWithAriaLabelledByProps;

export const RadioButton = forwardRef<HTMLButtonElement, RadioButtonProps>(
  (
    {
      value,
      label,
      className,
      labelClassName,
      disabled,
      size = 'md',
      'data-testid': dataTestId,
      'aria-label': ariaLabel,
      'aria-labelledby': ariaLabelledBy,
    },
    ref
  ) => {
    const id = useId();

    return (
      <div className={cn('flex items-center gap-space-2', className)}>
        <RadioGroupPrimitive.Item
          ref={ref}
          className={cn(
            'peer',
            radioButtonBaseStyles,
            radioButtonFocusStyles,
            radioButtonSizeStyles[size],
            radioButtonRootStateStyles
          )}
          value={value}
          id={id}
          disabled={disabled}
          data-testid={dataTestId}
          aria-label={ariaLabel}
          aria-labelledby={ariaLabelledBy}
        >
          <RadioGroupPrimitive.Indicator
            className={cn(
              'flex items-center justify-center rounded-full',
              radioButtonDotSizeStyles[size],
              radioButtonIndicatorStyles
            )}
          />
        </RadioGroupPrimitive.Item>
        {label && (
          <label
            htmlFor={id}
            className={cn(
              'cursor-pointer peer-data-[disabled]:cursor-not-allowed peer-data-[disabled]:text-tertiary'
            )}
          >
            <Text variant="sm" className={labelClassName}>
              {label}
            </Text>
          </label>
        )}
      </div>
    );
  }
);

RadioButton.displayName = 'RadioButton';
