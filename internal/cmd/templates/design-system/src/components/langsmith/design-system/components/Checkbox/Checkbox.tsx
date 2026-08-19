import { forwardRef, useId } from 'react';

import { CheckIcon, MinusIcon } from '@langchain/untitled-ui-icons';
import * as CheckboxPrimitive from '@radix-ui/react-checkbox';

import { cn } from '../../utils/cn';
import { Text } from '../Text';
import {
  checkboxBaseStyles,
  checkboxFocusStyles,
  checkboxIconSizeStyles,
  checkboxIndicatorStyles,
  checkboxRootStateStyles,
  checkboxSizeStyles,
} from './constants';

/**
 * Base props shared by all Checkbox variants.
 * Extends Radix CheckboxProps to support all native checkbox functionality.
 */
interface CheckboxBaseProps extends Omit<
  CheckboxPrimitive.CheckboxProps,
  'asChild'
> {
  /** Size of the checkbox */
  size?: 'sm' | 'md';
  /** Additional CSS classes for the container wrapper */
  containerClassName?: string;
}

/**
 * Props for Checkbox with a visible label.
 */
interface CheckboxWithLabelProps extends CheckboxBaseProps {
  /** Visible label text or React node displayed next to the checkbox */
  label: React.ReactNode;
  /** Additional CSS classes for the label */
  labelClassName?: string;
  'aria-label'?: never;
  'aria-labelledby'?: never;
}

/**
 * Props for Checkbox with aria-label (no visible label).
 */
interface CheckboxWithAriaLabelProps extends CheckboxBaseProps {
  /** Accessible label for screen readers when no visible label is present */
  'aria-label': string;
  label?: never;
  labelClassName?: never;
  'aria-labelledby'?: never;
}

/**
 * Props for Checkbox with aria-labelledby (labeled by external element).
 */
interface CheckboxWithAriaLabelledByProps extends CheckboxBaseProps {
  /** ID of an element that labels this checkbox */
  'aria-labelledby': string;
  label?: never;
  labelClassName?: never;
  'aria-label'?: never;
}

/**
 * Checkbox component props.
 *
 * For accessibility compliance (WCAG 2.1), checkboxes must have an accessible name.
 * This is enforced at the type level by requiring one of:
 * - `label`: A visible label displayed next to the checkbox
 * - `aria-label`: An accessible label for screen readers (when no visible label)
 * - `aria-labelledby`: Reference to an external labeling element
 *
 * @see https://www.w3.org/WAI/tutorials/forms/labels/
 * @see https://www.w3.org/WAI/ARIA/apg/patterns/checkbox/
 */
export type CheckboxProps =
  | CheckboxWithLabelProps
  | CheckboxWithAriaLabelProps
  | CheckboxWithAriaLabelledByProps;

export const Checkbox = forwardRef<HTMLButtonElement, CheckboxProps>(
  (
    {
      checked,
      label,
      className,
      containerClassName,
      labelClassName,
      disabled,
      size = 'md',
      ...rest
    },
    ref
  ) => {
    const id = useId();

    return (
      <div className={cn('flex items-center gap-space-2', containerClassName)}>
        <CheckboxPrimitive.Root
          ref={ref}
          className={cn(
            checkboxBaseStyles,
            checkboxFocusStyles,
            checkboxSizeStyles[size],
            checkboxRootStateStyles,
            className
          )}
          id={id}
          checked={checked}
          disabled={disabled}
          {...rest}
        >
          <CheckboxPrimitive.Indicator
            className={cn(
              'flex items-center justify-center',
              disabled
                ? checkboxIndicatorStyles.disabled
                : checkboxIndicatorStyles.enabled
            )}
          >
            {checked === 'indeterminate' ? (
              <MinusIcon className={checkboxIconSizeStyles[size]} />
            ) : (
              <CheckIcon className={checkboxIconSizeStyles[size]} />
            )}
          </CheckboxPrimitive.Indicator>
        </CheckboxPrimitive.Root>
        {label && (
          <label
            htmlFor={id}
            className={cn('cursor-pointer', disabled && 'cursor-not-allowed')}
          >
            <Text
              variant="sm"
              className={labelClassName}
              color={disabled ? 'tertiary' : undefined}
            >
              {label}
            </Text>
          </label>
        )}
      </div>
    );
  }
);

Checkbox.displayName = 'Checkbox';
