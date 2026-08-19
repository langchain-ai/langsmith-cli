/**
 * RadioButton style constants following the Button pattern.
 * Styles are organized by category for maintainability.
 */

/** Base styles applied to all radio buttons */
export const radioButtonBaseStyles =
  'flex shrink-0 items-center justify-center rounded-full border bg-elevated transition-colors';

/** Focus ring styles: brand border + shadow haze */
export const radioButtonFocusStyles =
  'focus-visible:border-brand focus-visible:outline-none focus-visible:shadow-[0_0_0_4px_var(--bg-brand-subtle)]';

/** Size variant styles for the radio button */
export const radioButtonSizeStyles = {
  sm: 'size-[16px]',
  md: 'size-[20px]',
} as const;

/** Size variant styles for the indicator dot */
export const radioButtonDotSizeStyles = {
  sm: 'size-[6px]',
  md: 'size-[8px]',
} as const;

/**
 * State styles using Radix data attributes.
 * These are combined and applied via cn() in the component.
 */
const radioButtonStateStyles = {
  // Default border (unchecked state)
  default: 'border-default',

  // Unchecked hover - slight background change
  uncheckedHover: 'data-[state=unchecked]:hover:bg-elevated-hover',

  // Checked state - filled with brand color
  checked:
    'data-[state=checked]:border-transparent data-[state=checked]:bg-control-active hover:data-[state=checked]:bg-control-active-hover',

  // Disabled base - applies to all states when disabled
  disabledBase:
    'data-[disabled]:cursor-not-allowed data-[disabled]:border-disabled data-[disabled]:bg-disabled',

  // Disabled unchecked - override hover effects
  disabledUnchecked:
    'data-[disabled]:data-[state=unchecked]:hover:border-disabled data-[disabled]:data-[state=unchecked]:hover:shadow-none data-[disabled]:hover:bg-disabled  data-[state=unchecked]:hover:bg-disabled',

  // Disabled checked - override hover effects
  disabledChecked:
    'data-[disabled]:data-[state=checked]:border-disabled data-[disabled]:data-[state=checked]:bg-disabled data-[disabled]:data-[state=checked]:hover:bg-disabled',
} as const;

/**
 * Indicator dot styles. The disabled fill is applied via Radix's `data-disabled`
 * attribute, which is set on the Indicator whenever the radio button or its
 * enclosing group is disabled — so it covers both item-level and group-level
 * disabled states, not just the local `disabled` prop.
 */
export const radioButtonIndicatorStyles =
  'bg-control-thumb data-[disabled]:bg-control-disabled';

/**
 * Combined radio button root styles - convenience export for component use.
 * All state styles combined as an array to be spread in cn().
 */
export const radioButtonRootStateStyles = [
  radioButtonStateStyles.default,
  radioButtonStateStyles.uncheckedHover,
  radioButtonStateStyles.checked,
  radioButtonStateStyles.disabledBase,
  radioButtonStateStyles.disabledUnchecked,
  radioButtonStateStyles.disabledChecked,
] as const;
