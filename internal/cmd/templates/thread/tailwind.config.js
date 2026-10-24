const withAlpha = (cssVar) => ({ opacityValue }) =>
  opacityValue !== undefined
    ? `color-mix(in srgb, ${cssVar} calc(${opacityValue} * 100%), transparent)`
    : cssVar;

const spaceScale = {
  'space-1': '0.25rem',
  'space-2': '0.5rem',
  'space-3': '0.75rem',
  'space-4': '1rem',
  'space-5': '1.25rem',
  'space-6': '1.5rem',
  'space-7': '2rem',
  'space-8': '2.5rem',
  'space-9': '4rem',
};

/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      backgroundColor: {
        // Surfaces
        'surface-level-1': withAlpha('var(--bg-surface-level-1)'),
        'surface-level-2': withAlpha('var(--bg-surface-level-2)'),
        'surface-level-3': withAlpha('var(--bg-surface-level-3)'),
        'surface-level-4': withAlpha('var(--bg-surface-level-4)'),
        'elevated': withAlpha('var(--bg-elevated)'),
        'elevated-hover': withAlpha('var(--bg-elevated-hover)'),
        'disabled': withAlpha('var(--bg-disabled)'),
        // Brand
        'brand': withAlpha('var(--bg-brand)'),
        'brand-hover': withAlpha('var(--bg-brand-hover)'),
        'brand-subtle': withAlpha('var(--bg-brand-subtle)'),
        'brand-subtle-hover': withAlpha('var(--bg-brand-subtle-hover)'),
        'brand-muted': withAlpha('var(--bg-brand-muted)'),
        // Intent
        'success': withAlpha('var(--bg-success)'),
        'success-subtle': withAlpha('var(--bg-success-subtle)'),
        'success-strong': withAlpha('var(--bg-success-strong)'),
        'error': withAlpha('var(--bg-error)'),
        'error-subtle': withAlpha('var(--bg-error-subtle)'),
        'error-strong': withAlpha('var(--bg-error-strong)'),
        'warning': withAlpha('var(--bg-warning)'),
        'warning-subtle': withAlpha('var(--bg-warning-subtle)'),
        'warning-strong': withAlpha('var(--bg-warning-strong)'),
        // Controls
        'control-active': withAlpha('var(--bg-control-active)'),
        'control-thumb': withAlpha('var(--bg-control-thumb)'),
        'control-disabled': withAlpha('var(--bg-control-disabled)'),
        // Selected
        'selected': withAlpha('var(--bg-selected)'),
        'selected-hover': withAlpha('var(--bg-selected-hover)'),
        // Deprecated aliases (for compatibility with existing classes)
        'primary': withAlpha('var(--bg-surface-level-1)'),
        'secondary': withAlpha('var(--bg-surface-level-2)'),
        'tertiary': withAlpha('var(--bg-surface-level-3)'),
        'quaternary': withAlpha('var(--bg-surface-level-4)'),
      },
      borderColor: {
        'default': withAlpha('var(--border-default)'),
        'subtle': withAlpha('var(--border-subtle)'),
        'muted': withAlpha('var(--border-muted)'),
        'faint': withAlpha('var(--border-faint)'),
        'strong': withAlpha('var(--border-strong)'),
        'disabled': withAlpha('var(--border-disabled)'),
        'focus': withAlpha('var(--border-focus)'),
        'error': withAlpha('var(--border-error)'),
        'error-strong': withAlpha('var(--border-error-strong)'),
        'brand': withAlpha('var(--border-brand)'),
        'brand-strong': withAlpha('var(--border-brand-strong)'),
        'brand-subtle': withAlpha('var(--border-brand-subtle)'),
        'warning': withAlpha('var(--border-warning)'),
        'success': withAlpha('var(--border-success)'),
        // Deprecated aliases
        'primary': withAlpha('var(--border-default)'),
        'secondary': withAlpha('var(--border-subtle)'),
        'tertiary': withAlpha('var(--border-muted)'),
        'quaternary': withAlpha('var(--border-faint)'),
      },
      textColor: {
        'primary': withAlpha('var(--text-primary)'),
        'secondary': withAlpha('var(--text-secondary)'),
        'tertiary': withAlpha('var(--text-tertiary)'),
        'quaternary': withAlpha('var(--text-quaternary)'),
        'disabled': withAlpha('var(--text-disabled)'),
        'placeholder': withAlpha('var(--text-placeholder)'),
        'error-primary': withAlpha('var(--text-error-primary)'),
        'error-secondary': withAlpha('var(--text-error-secondary)'),
        'warning-primary': withAlpha('var(--text-warning-primary)'),
        'success-primary': withAlpha('var(--text-success-primary)'),
        'brand-primary': withAlpha('var(--text-brand-primary)'),
        'brand-secondary': withAlpha('var(--text-brand-secondary)'),
        'brand-on-fill': withAlpha('var(--text-brand-on-fill)'),
        'link': withAlpha('var(--text-link)'),
        'link-hover': withAlpha('var(--text-link-hover)'),
        'icon-primary': withAlpha('var(--icon-primary)'),
        'icon-secondary': withAlpha('var(--icon-secondary)'),
        'icon-tertiary': withAlpha('var(--icon-tertiary)'),
        'icon-disabled': withAlpha('var(--icon-disabled)'),
        'icon-brand': withAlpha('var(--icon-brand)'),
        'icon-error': withAlpha('var(--icon-error)'),
        'icon-success': withAlpha('var(--icon-success)'),
        'icon-warning': withAlpha('var(--icon-warning)'),
      },
      borderRadius: {
        'xs': 'var(--radius-xs)',
        'sm': 'var(--radius-sm)',
        'md': 'var(--radius-md)',
        'lg': 'var(--radius-lg)',
        'xl': 'var(--radius-xl)',
        'full': 'var(--radius-full)',
      },
      boxShadow: {
        'sm': 'var(--shadow-sm)',
        'md': 'var(--shadow-md)',
        'lg': 'var(--shadow-lg)',
      },
      gap: spaceScale,
      padding: spaceScale,
      margin: spaceScale,
      space: spaceScale,
    },
  },
  plugins: [],
};
