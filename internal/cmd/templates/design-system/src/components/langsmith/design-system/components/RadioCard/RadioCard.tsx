import { forwardRef, useId } from 'react';

import * as RadioGroupPrimitive from '@radix-ui/react-radio-group';

import { cn } from '../../utils/cn';
import {
  radioButtonBaseStyles,
  radioButtonDotSizeStyles,
  radioButtonFocusStyles,
  radioButtonIndicatorStyles,
  radioButtonRootStateStyles,
  radioButtonSizeStyles,
} from '../RadioButton/constants';

export interface RadioCardProps {
  value: string;
  children?: React.ReactNode;
  /**
   * Content revealed only when the card is selected.
   * When provided, collapse/expand behavior is enabled automatically
   * using CSS `group-has-[[role=radio][data-state=checked]]`
   */
  collapsible?: React.ReactNode;
  className?: string;
  disabled?: boolean;
  radioPosition?: 'left' | 'right';
  size?: 'sm' | 'md';
  /**
   * Hides the visible radio indicator while keeping it in the DOM so that
   * the selection state (border highlight, collapsible reveal) still works.
   * Use when the card design itself is the only selection affordance needed.
   */
  hideIndicator?: boolean;
  'data-testid'?: string;
  'aria-label'?: string;
  'aria-labelledby'?: string;
}

export const RadioCard = forwardRef<HTMLButtonElement, RadioCardProps>(
  (
    {
      value,
      children,
      collapsible,
      className,
      disabled,
      radioPosition = 'left',
      size = 'md',
      hideIndicator = false,
      'data-testid': dataTestId,
      'aria-label': ariaLabel,
      'aria-labelledby': ariaLabelledBy,
    },
    ref
  ) => {
    const id = useId();

    const radioItem = (
      <RadioGroupPrimitive.Item
        ref={ref}
        id={id}
        value={value}
        disabled={disabled}
        aria-label={ariaLabel}
        aria-labelledby={ariaLabelledBy}
        data-testid={dataTestId}
        className={cn(
          radioButtonBaseStyles,
          radioButtonFocusStyles,
          radioButtonSizeStyles[size],
          radioButtonRootStateStyles,
          'shrink-0'
        )}
      >
        <RadioGroupPrimitive.Indicator
          className={cn(
            'flex items-center justify-center rounded-full',
            radioButtonDotSizeStyles[size],
            radioButtonIndicatorStyles
          )}
        />
      </RadioGroupPrimitive.Item>
    );

    return (
      <label
        htmlFor={id}
        className={cn(
          // Named group so nested collapsible content can reference it safely
          // even when RadioCards are nested inside other groups.
          'group/rc',
          'flex items-center rounded-lg border-2 border-secondary p-space-4 transition-colors has-[[role=radio][data-state=checked]]:border-brand',
          size === 'sm' ? 'gap-space-3' : 'gap-space-4',
          disabled ? 'cursor-not-allowed' : 'cursor-pointer',
          !disabled && 'hover:bg-primary-hover',
          disabled && 'text-disabled',
          className
        )}
      >
        {/* When hideIndicator, keep the radio item in the DOM (so CSS has()
            and screen readers still work) but remove it from layout flow. */}
        {hideIndicator ? (
          <span className="sr-only">{radioItem}</span>
        ) : (
          radioPosition === 'left' && radioItem
        )}

        <div className="flex min-w-0 flex-1 flex-col gap-space-2">
          {children}
          {collapsible != null && (
            <div className="hidden group-has-[[role=radio][data-state=checked]]/rc:block">
              {collapsible}
            </div>
          )}
        </div>

        {!hideIndicator && radioPosition === 'right' && radioItem}
      </label>
    );
  }
);

RadioCard.displayName = 'RadioCard';
