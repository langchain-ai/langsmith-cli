import type { ClassValue } from 'clsx';
import { clsx } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

/** Numeric brand palette keys from `theme.colors.brand` (Tailwind). */
const BRAND_PALETTE_STEPS = [
  '10',
  '25',
  '50',
  '75',
  '100',
  '200',
  '300',
  '400',
  '500',
  '600',
  '700',
  '800',
  '900',
  '950',
] as const;

const brandBgClasses = BRAND_PALETTE_STEPS.map((step) => `bg-brand-${step}`);
const brandBorderClasses = BRAND_PALETTE_STEPS.map(
  (step) => `border-brand-${step}`
);
const brandTextClasses = BRAND_PALETTE_STEPS.map(
  (step) => `text-brand-${step}`
);
const buttonBorderColorClasses = [
  'button-primary-underlined-border',
  'button-primary-underlined-border-hover',
  'button-secondary-underlined-border',
  'button-secondary-underlined-border-hover',
  'button-error-underlined-border',
  'button-error-underlined-border-hover',
  'button-error-disabled-border',
];
const buttonTextColorClasses = [
  'button-primary-underlined-foreground',
  'button-primary-underlined-foreground-hover',
  'button-error-underlined-foreground-hover',
  'button-error-disabled-foreground',
];

// Lets `space-*` tokens dedupe against built-in spacing (keep in sync with tailwind.config.js).
const SPACE_SCALE_STEPS = [
  'space-1',
  'space-2',
  'space-3',
  'space-4',
  'space-5',
  'space-6',
  'space-7',
  'space-8',
  'space-9',
];

const customTwMerge = extendTailwindMerge({
  extend: {
    theme: {
      spacing: SPACE_SCALE_STEPS,
    },
    classGroups: {
      // Extend font-size to include custom display and caps classes
      'font-size': [
        'display-sm',
        'display-base',
        'display-lg',
        'display-xl',
        'display-2xl',
        'caps-label-sm',
        'caps-label-xs',
        'text-xxs',
        'text-xs',
        'text-sm',
        'text-base',
        'text-lg',
        'text-xl',
      ],
      // Extend bg-color to include custom background colors
      'bg-color': [
        'bg-primary',
        'bg-primary-hover',
        'bg-secondary',
        'bg-secondary-hover',
        'bg-tertiary',
        'bg-quaternary',
        'bg-surface-level-1',
        'bg-surface-level-1-hover',
        'bg-surface-level-2',
        'bg-surface-level-2-hover',
        'bg-surface-level-3',
        'bg-surface-level-4',
        'bg-disabled',
        'bg-elevated',
        'bg-elevated-hover',
        'bg-elevated-selected',
        'bg-overlay',
        'bg-brand',
        'bg-brand-hover',
        'bg-brand-subtle',
        'bg-brand-subtle-gradient',
        'bg-brand-subtle-hover',
        'bg-brand-muted',
        'bg-brand-primary',
        'bg-brand-primary-hover',
        'bg-brand-secondary',
        'bg-brand-secondary-hover',
        'bg-brand-tertiary',
        'bg-purple',
        'bg-success',
        'bg-success-subtle',
        'bg-success-primary',
        'bg-success-secondary',
        'bg-success-strong',
        'bg-error',
        'bg-error-subtle',
        'bg-error-primary',
        'bg-error-secondary',
        'bg-error-strong',
        'bg-warning',
        'bg-warning-subtle',
        'bg-warning-primary',
        'bg-warning-secondary',
        'bg-warning-strong',
        'bg-control-active',
        'bg-control-active-hover',
        'bg-control-thumb',
        'bg-control-disabled',
        'bg-selected',
        'bg-selected-hover',
        ...brandBgClasses,
      ],
      // Extend border-color to include custom border colors
      'border-color': [
        'border-default',
        'border-subtle',
        'border-muted',
        'border-faint',
        'border-primary',
        'border-secondary',
        'border-tertiary',
        'border-quaternary',
        'border-error',
        'border-error-strong',
        'border-brand',
        'border-brand-strong',
        'border-brand-subtle',
        'border-strong',
        'border-disabled',
        'border-focus',
        'border-warning',
        'border-success',
        'border-purple',
        'border-status-green',
        'border-status-orange',
        'border-status-yellow',
        'border-status-red',
        ...brandBorderClasses,
        ...buttonBorderColorClasses,
      ],
      // Extend text-color to include custom text colors
      'text-color': [
        'text-primary',
        'text-secondary',
        'text-tertiary',
        'text-quaternary',
        'text-disabled',
        'text-error-secondary',
        'text-warning-secondary',
        'text-success-secondary',
        'text-placeholder',
        'text-control-active-foreground',
        'text-purple',
        'text-brand-primary',
        'text-brand-secondary',
        'text-brand-tertiary',
        'text-brand-disabled',
        'text-brand-on-fill',
        'text-link',
        'text-link-hover',
        'text-status-green',
        'text-status-orange',
        'text-status-yellow',
        'text-status-red',
        'text-button-primary',
        'text-icon-primary',
        'text-icon-secondary',
        'text-icon-tertiary',
        'text-icon-disabled',
        'text-icon-brand',
        'text-icon-error',
        'text-icon-success',
        'text-icon-warning',
        ...brandTextClasses,
        ...buttonTextColorClasses,
      ],
    },
  },
});

export const cn = (...inputs: ClassValue[]) => {
  return customTwMerge(clsx(inputs));
};
