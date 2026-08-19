// Define styles for each specific combination
export const buttonStyleMap = {
  // Primary variants - enabled
  'primary-normal-enabled':
    'button-primary-bg button-primary-foreground border button-primary-border hover:border-transparent hover:button-primary-bg-hover dark:hover:button-primary-bg-hover dark:hover:border-transparent',
  'primary-outlined-enabled':
    'bg-transparent text-brand-secondary border border-brand-strong hover:text-brand-primary hover:bg-brand-muted dark:border-brand dark:hover:bg-brand-subtle dark:hover:button-primary-border dark:hover:text-brand-primary',
  'primary-plain-enabled':
    'bg-transparent text-brand-400 dark:text-brand-700 border border-transparent hover:text-brand-secondary dark:hover:text-brand-primary hover:bg-brand-muted/80',
  'primary-underlined-enabled':
    'bg-transparent button-primary-underlined-foreground border-b button-primary-underlined-border hover:button-primary-underlined-border-hover hover:button-primary-underlined-foreground-hover',

  // Primary variants - disabled
  'primary-normal-disabled':
    'bg-brand-muted border border-brand-disabled text-brand-disabled',
  'primary-outlined-disabled':
    'bg-brand-muted border border-transparent text-brand-disabled',
  'primary-plain-disabled':
    'bg-brand-muted border border-transparent text-brand-disabled',
  'primary-underlined-disabled':
    'bg-transparent text-brand-disabled border border-transparent',

  // Secondary variants - enabled
  'secondary-normal-enabled':
    'bg-elevated text-secondary border border-subtle dark:border-muted hover:bg-elevated-hover/80',
  'secondary-outlined-enabled':
    'bg-elevated text-secondary border border-subtle dark:border-muted hover:bg-elevated-hover/80',
  'secondary-plain-enabled':
    'bg-transparent text-secondary border border-transparent hover:bg-elevated-hover/80 dark:hover:bg-elevated-hover/40',
  'secondary-underlined-enabled':
    'text-secondary hover:text-secondary-hover border-b button-secondary-underlined-border hover:button-secondary-underlined-border-hover',

  // Secondary variants - disabled
  'secondary-normal-disabled':
    'bg-surface-level-1-hover dark:bg-disabled border border-transparent text-disabled',
  'secondary-outlined-disabled':
    'bg-surface-level-1-hover dark:bg-disabled border border-transparent text-disabled',
  'secondary-plain-disabled':
    'bg-surface-level-1-hover dark:bg-disabled border border-transparent text-disabled',
  'secondary-underlined-disabled': 'text-disabled border border-transparent',

  // Error variants - enabled
  'error-normal-enabled':
    'bg-error-strong dark:bg-error-secondary text-white border border-red-600 hover:bg-red-700 dark:hover:bg-error',
  'error-outlined-enabled':
    'bg-transparent text-error-secondary border border-red-700 hover:bg-error',
  'error-plain-enabled':
    'bg-transparent text-error-secondary border border-transparent hover:bg-error/80',
  'error-underlined-enabled':
    'bg-transparent text-error-secondary border-b button-error-underlined-border hover:button-error-underlined-border-hover hover:button-error-underlined-foreground-hover',

  // Error variants - disabled
  'error-normal-disabled':
    'bg-error border border-transparent button-error-disabled-foreground',
  'error-outlined-disabled':
    'bg-transparent button-error-disabled-foreground border button-error-disabled-border',
  'error-plain-disabled':
    'bg-transparent button-error-disabled-foreground border border-transparent',
  'error-underlined-disabled':
    'bg-transparent button-error-disabled-foreground border border-transparent',
};
