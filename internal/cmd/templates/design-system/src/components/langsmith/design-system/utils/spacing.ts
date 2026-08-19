/**
 * 4-point spacing scale (`space-1` … `space-9`).
 *
 * For guidelines on how to use, refer to Storybook:
 * https://langsmith-design-system.vercel.app/?path=/story/design-system-foundations-spacing--scale-and-guidelines
 *
 * The scale is surfaced as additive Tailwind utilities via
 * `tailwind.config.js`, so it is used through the property prefix:
 * `gap-space-4`, `px-space-6`, `mt-space-2`, etc.
 * The px mapping mirrors Radix
 * Themes spacing (https://www.radix-ui.com/themes/docs/theme/spacing).
 *
 * This module is the single source of truth. Keep it in sync with:
 *   - `tailwind.config.js` (theme.extend.spacing `space-*` keys)
 *   - `eslint-plugin-custom/rules/no-off-scale-spacing.js`
 *   - `eslint-plugin-custom/rules/require-spacing-tokens.js`
 */

/** Step → pixel value. The step is an ordinal index, not the px value. */
export const SPACE_SCALE_PX = {
  1: 4,
  2: 8,
  3: 12,
  4: 16,
  5: 24,
  6: 32,
  7: 40,
  8: 48,
  9: 64,
} as const;

export type SpaceStep = keyof typeof SPACE_SCALE_PX;

/** Ordered list of steps, e.g. for rendering the scale. */
export const SPACE_STEPS = Object.keys(SPACE_SCALE_PX).map(
  (step) => Number(step) as SpaceStep
);
