import { cn } from '../../utils/cn';

export type InputSize = 'sm' | 'md';
export type InputVariant = 'outlined' | 'plain';

/**
 * Shared styling for input containers (border, padding, rounding, variant).
 * Used by both the design system Input and CommandInput.
 */
export function getInputContainerClasses({
  size,
  variant,
  isError = false,
  disabled = false,
  className,
}: {
  size: InputSize;
  variant: InputVariant;
  isError?: boolean;
  disabled?: boolean;
  className?: string;
}) {
  const outlined = variant === 'outlined';
  return cn(
    'flex w-full items-center gap-space-2 transition-[border-color,box-shadow,background-color]',
    {
      'px-space-2 py-space-1': size === 'sm',
      'px-space-3 py-space-2': size === 'md',
    },
    {
      'rounded-sm': size === 'sm',
      'rounded-md': size === 'md',
    },
    'focus-within:outline-none',
    {
      'border border-subtle bg-transparent': variant === 'outlined',
      'border border-transparent bg-surface-level-3': variant === 'plain',
    },
    outlined &&
      !disabled &&
      !isError &&
      '[&:hover:not(:focus-within)]:border-default',
    !isError &&
      (outlined
        ? 'focus-within:border-focus focus-within:shadow-[0_0_0_1px_var(--border-focus)]'
        : 'focus-within:border-focus'),
    isError &&
      (outlined
        ? 'border-error-strong focus-within:border-error-strong focus-within:shadow-[0_0_0_1px_var(--border-error-strong)]'
        : 'border-error-strong focus-within:border-error-strong'),
    disabled && 'cursor-not-allowed bg-disabled opacity-50',
    className
  );
}

/**
 * Shared styling for the inner input element (unstyled, fills container).
 * Used by both the design system Input and CommandInput.
 */
export function getInputElementClasses({
  size,
  disabled = false,
  className,
}: {
  size: InputSize;
  disabled?: boolean;
  className?: string;
}) {
  return cn(
    'min-w-0 max-w-full flex-1 border-none bg-transparent p-0 outline-none',
    {
      'text-xs leading-normal': size === 'sm',
      'text-sm leading-normal': size === 'md',
    },
    'placeholder:text-placeholder',
    disabled && 'cursor-not-allowed',
    className
  );
}

/** Shared styling for decorator wrappers. */
export const DECORATOR_CLASSES = 'flex items-center text-tertiary';
