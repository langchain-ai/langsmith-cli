/**
 * Checkbox style constants following the Button pattern.
 * Styles are organized by category for maintainability.
 */

/** Base styles applied to all checkboxes */
export const checkboxBaseStyles =
  'flex shrink-0 items-center justify-center rounded-[4px] border bg-elevated transition-colors';

/** Focus ring styles: brand border + shadow haze */
export const checkboxFocusStyles =
  'focus-visible:border-brand focus-visible:outline-none focus-visible:shadow-[0_0_0_4px_var(--bg-brand-subtle)]';

/** Size variant styles */
export const checkboxSizeStyles = {
  sm: 'size-[14px]',
  md: 'size-[18px]',
} as const;

/** Icon size styles for the indicator */
export const checkboxIconSizeStyles = {
  sm: 'h-[10px] w-[10px]',
  md: 'h-[14px] w-[14px]',
} as const;

/**
 * State styles using Radix data attributes.
 * These are combined and applied via cn() in the component.
 */
const checkboxStateStyles = {
  // Default border (unchecked state)
  default: 'border-default',

  // Unchecked hover - slight background change
  uncheckedHover: 'data-[state=unchecked]:hover:bg-elevated-hover',

  // Checked state - filled with brand color
  checked:
    'data-[state=checked]:border-transparent data-[state=checked]:bg-brand-300 hover:data-[state=checked]:bg-brand-400',

  // Indeterminate state - same as checked
  indeterminate:
    'data-[state=indeterminate]:border-transparent data-[state=indeterminate]:bg-brand-300 hover:data-[state=indeterminate]:bg-brand-400',

  // Disabled base - applies to all states when disabled
  disabledBase:
    'data-[disabled]:cursor-not-allowed data-[disabled]:border-ls-neutral-300 data-[disabled]:bg-disabled',

  // Disabled unchecked - override hover effects
  disabledUnchecked:
    'data-[disabled]:data-[state=unchecked]:hover:border-ls-neutral-300 data-[disabled]:data-[state=unchecked]:hover:shadow-none data-[disabled]:hover:bg-disabled',

  // Disabled checked - override hover effects
  disabledChecked:
    'data-[disabled]:data-[state=checked]:border-ls-neutral-300 data-[disabled]:data-[state=checked]:bg-disabled data-[disabled]:data-[state=checked]:hover:bg-disabled',

  // Disabled indeterminate - override hover effects
  disabledIndeterminate:
    'data-[disabled]:data-[state=indeterminate]:border-ls-neutral-300 data-[disabled]:data-[state=indeterminate]:bg-disabled data-[disabled]:data-[state=indeterminate]:hover:bg-disabled',
} as const;

/** Indicator styles for enabled/disabled states */
export const checkboxIndicatorStyles = {
  enabled: 'text-control-active-foreground',
  disabled: 'text-icon-disabled',
} as const;

/**
 * Combined checkbox root styles - convenience export for component use.
 * All state styles combined as an array to be spread in cn().
 */
export const checkboxRootStateStyles = [
  checkboxStateStyles.default,
  checkboxStateStyles.uncheckedHover,
  checkboxStateStyles.checked,
  checkboxStateStyles.indeterminate,
  checkboxStateStyles.disabledBase,
  checkboxStateStyles.disabledUnchecked,
  checkboxStateStyles.disabledChecked,
  checkboxStateStyles.disabledIndeterminate,
] as const;
