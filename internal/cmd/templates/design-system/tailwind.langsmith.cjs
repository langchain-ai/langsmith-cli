const { blackA, green, mauve, slate, violet } = require('@radix-ui/colors');
const plugin = require('tailwindcss/plugin');

/**
 * Wraps a CSS variable so Tailwind's opacity modifier (e.g. /50) works.
 * Without this, `bg-surface-level-2/50` silently produces no output because
 * Tailwind can't inject alpha into a plain var() reference.
 * Uses color-mix() so no separate RGB channel variables are needed.
 */
const withAlpha =
  (cssVar) =>
  ({ opacityValue }) =>
    opacityValue !== undefined
      ? `color-mix(in srgb, ${cssVar} calc(${opacityValue} * 100%), transparent)`
      : cssVar;

const brandPalette = {
  10: withAlpha('var(--brand-10)'),
  25: withAlpha('var(--brand-25)'),
  50: withAlpha('var(--brand-50)'),
  75: withAlpha('var(--brand-75)'),
  100: withAlpha('var(--brand-100)'),
  200: withAlpha('var(--brand-200)'),
  300: withAlpha('var(--brand-300)'),
  400: withAlpha('var(--brand-400)'),
  500: withAlpha('var(--brand-500)'),
  600: withAlpha('var(--brand-600)'),
  700: withAlpha('var(--brand-700)'),
  800: withAlpha('var(--brand-800)'),
  900: withAlpha('var(--brand-900)'),
  950: withAlpha('var(--brand-950)'),
};

// Keep in sync with src/design-system/utils/spacing.ts and the spacing lint
// rules in eslint-plugin-custom/rules/.
const spaceScale = {
  'space-1': '0.25rem', // 4px
  'space-2': '0.5rem', // 8px
  'space-3': '0.75rem', // 12px
  'space-4': '1rem', // 16px
  'space-5': '1.5rem', // 24px
  'space-6': '2rem', // 32px
  'space-7': '2.5rem', // 40px
  'space-8': '3rem', // 48px
  'space-9': '4rem', // 64px
};

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [],
  darkMode: 'class',
  theme: {
    extend: {
      fontSize: {
        xxs: [
          '0.75rem', // 12px
          {
            lineHeight: '1.125rem', // 18px
          },
        ],
        xs: [
          '0.8125rem', // 13px
          {
            lineHeight: '1.125rem', // 18px
          },
        ],
        sm: [
          '0.875rem', // 14px
          {
            lineHeight: '1.25rem', // 20px
          },
        ],
        base: [
          '1rem', // 16px
          {
            lineHeight: '1.5rem', // 24px
          },
        ],
        lg: [
          '1.125rem', // 18px
          {
            lineHeight: '1.75rem', // 28px
            letterSpacing: '-0.01em', // tracking-tight
          },
        ],
        xl: [
          '1.25rem', // 20px
          {
            lineHeight: '1.875rem', // 30px
            letterSpacing: '-0.01em', // tracking-tight
          },
        ],
      },
      fontFamily: {
        lausanne: [`"TWK Lausanne"`, `Inter`, `sans-serif`],
        'aeonik-mono': [
          `"Aeonik Mono"`,
          `"Fira Code"`,
          `ui-monospace`,
          `monospace`,
        ],
        mono: [
          `"Fira Code"`,
          `ui-monospace`,
          `SFMono-Regular`,
          `Menlo`,
          `Monaco`,
          `Consolas`,
          `"Liberation Mono"`,
          `"Courier New"`,
          `monospace`,
        ],
      },
      letterSpacing: {
        tighter: '-0.04em',
        tight: '-0.03em',
        snug: '-0.02em',
        normal: '0',
        wide: '0.03em',
      },
      lineHeight: {
        tight: '1.20',
      },
      // Shrink `h-screen` / `min-h-screen` by the total height of any platform
      // banners that are currently mounted (see the `:has()` rules below).
      // 0px when no banner is in the DOM.
      height: {
        screen: 'calc(100vh - var(--banner-height, 0px))',
      },
      minHeight: {
        screen: 'calc(100vh - var(--banner-height, 0px))',
      },
      maxHeight: {
        screen: 'calc(100vh - var(--banner-height, 0px))',
      },
      backgroundImage: {
        navMenu: 'linear-gradient(132deg, #4499F7 0%, #3FCDD6 100%)',
        'brand-subtle-gradient': 'var(--bg-brand-subtle-gradient)',
      },
      borderRadius: {
        xs: 'var(--radius-xs)',
        sm: 'var(--radius-sm)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
        xl: 'var(--radius-xl)',
        full: 'var(--radius-full)',
      },
      boxShadow: {
        xs: '0 1px 1px 0 rgba(16, 24, 40, 0.02)',
        sm: 'var(--shadow-sm)',
        md: 'var(--shadow-md)',
        lg: 'var(--shadow-lg)',
      },
      transitionDuration: {
        fast: 'var(--duration-fast)',
        normal: 'var(--duration-normal)',
        slow: 'var(--duration-slow)',
        slower: 'var(--duration-slower)',
      },
      gap: spaceScale,
      padding: spaceScale,
      margin: spaceScale,
      space: spaceScale,
      backgroundColor: {
        'surface-level-1': withAlpha('var(--bg-surface-level-1)'),
        'surface-level-1-hover': withAlpha('var(--bg-surface-level-1-hover)'),
        'surface-level-2': withAlpha('var(--bg-surface-level-2)'),
        'surface-level-2-hover': withAlpha('var(--bg-surface-level-2-hover)'),
        'surface-level-3': withAlpha('var(--bg-surface-level-3)'),
        'surface-level-4': withAlpha('var(--bg-surface-level-4)'),
        elevated: withAlpha('var(--bg-elevated)'),
        'elevated-hover': withAlpha('var(--bg-elevated-hover)'),
        'elevated-selected': withAlpha('var(--bg-elevated-selected)'),
        'popover-urgent': withAlpha('var(--bg-popover-urgent)'),

        disabled: withAlpha('var(--bg-disabled)'),

        overlay: withAlpha('var(--bg-overlay)'),

        /* brand */
        brand: {
          DEFAULT: withAlpha('var(--bg-brand)'),
          ...brandPalette,
        },
        'brand-hover': withAlpha('var(--bg-brand-hover)'),
        'brand-subtle': withAlpha('var(--bg-brand-subtle)'),
        'brand-subtle-hover': withAlpha('var(--bg-brand-subtle-hover)'),
        'brand-muted': withAlpha('var(--bg-brand-muted)'),
        purple: withAlpha('var(--bg-purple)'),

        /* intent */
        success: withAlpha('var(--bg-success)'),
        'success-subtle': withAlpha('var(--bg-success-subtle)'),
        'success-strong': withAlpha('var(--bg-success-strong)'),
        error: withAlpha('var(--bg-error)'),
        'error-subtle': withAlpha('var(--bg-error-subtle)'),
        'error-strong': withAlpha('var(--bg-error-strong)'),
        warning: withAlpha('var(--bg-warning)'),
        'warning-subtle': withAlpha('var(--bg-warning-subtle)'),
        'warning-strong': withAlpha('var(--bg-warning-strong)'),

        /* control */
        'control-active': withAlpha('var(--bg-control-active)'),
        'control-active-hover': withAlpha('var(--bg-control-active-hover)'),
        'control-thumb': withAlpha('var(--bg-control-thumb)'),
        'control-disabled': withAlpha('var(--bg-control-disabled)'),

        /* selected */
        selected: withAlpha('var(--bg-selected)'),
        'selected-hover': withAlpha('var(--bg-selected-hover)'),

        /* data visualization */
        'chart-categorical-line-1': withAlpha(
          'var(--chart-categorical-line-1)'
        ),
        'chart-categorical-line-2': withAlpha(
          'var(--chart-categorical-line-2)'
        ),
        'chart-categorical-line-3': withAlpha(
          'var(--chart-categorical-line-3)'
        ),
        'chart-categorical-line-4': withAlpha(
          'var(--chart-categorical-line-4)'
        ),
        'chart-categorical-line-5': withAlpha(
          'var(--chart-categorical-line-5)'
        ),
        'chart-categorical-line-6': withAlpha(
          'var(--chart-categorical-line-6)'
        ),
        'chart-categorical-line-7': withAlpha(
          'var(--chart-categorical-line-7)'
        ),
        'chart-categorical-line-8': withAlpha(
          'var(--chart-categorical-line-8)'
        ),
        'chart-categorical-fill-1': withAlpha(
          'var(--chart-categorical-fill-1)'
        ),
        'chart-categorical-fill-2': withAlpha(
          'var(--chart-categorical-fill-2)'
        ),
        'chart-categorical-fill-3': withAlpha(
          'var(--chart-categorical-fill-3)'
        ),
        'chart-categorical-fill-4': withAlpha(
          'var(--chart-categorical-fill-4)'
        ),
        'chart-categorical-fill-5': withAlpha(
          'var(--chart-categorical-fill-5)'
        ),
        'chart-categorical-fill-6': withAlpha(
          'var(--chart-categorical-fill-6)'
        ),
        'chart-categorical-fill-7': withAlpha(
          'var(--chart-categorical-fill-7)'
        ),
        'chart-categorical-fill-8': withAlpha(
          'var(--chart-categorical-fill-8)'
        ),
        'table-heatmap-negative-1': withAlpha(
          'var(--table-heatmap-negative-1)'
        ),
        'table-heatmap-negative-2': withAlpha(
          'var(--table-heatmap-negative-2)'
        ),
        'table-heatmap-negative-3': withAlpha(
          'var(--table-heatmap-negative-3)'
        ),
        'table-heatmap-negative-4': withAlpha(
          'var(--table-heatmap-negative-4)'
        ),
        'table-heatmap-negative-5': withAlpha(
          'var(--table-heatmap-negative-5)'
        ),
        'table-heatmap-neutral': withAlpha('var(--table-heatmap-neutral)'),
        'table-heatmap-positive-1': withAlpha(
          'var(--table-heatmap-positive-1)'
        ),
        'table-heatmap-positive-2': withAlpha(
          'var(--table-heatmap-positive-2)'
        ),
        'table-heatmap-positive-3': withAlpha(
          'var(--table-heatmap-positive-3)'
        ),
        'table-heatmap-positive-4': withAlpha(
          'var(--table-heatmap-positive-4)'
        ),
        'table-heatmap-positive-5': withAlpha(
          'var(--table-heatmap-positive-5)'
        ),

        primary: withAlpha('var(--bg-primary)'),
        'primary-hover': withAlpha('var(--bg-primary-hover)'),
        secondary: withAlpha('var(--bg-secondary)'),
        'secondary-hover': withAlpha('var(--bg-secondary-hover)'),
        tertiary: withAlpha('var(--bg-tertiary)'),
        quaternary: withAlpha('var(--bg-quaternary)'),
        'brand-primary': withAlpha('var(--bg-brand-primary)'),
        'brand-primary-hover': withAlpha('var(--bg-brand-primary_hover)'),
        'brand-secondary': withAlpha('var(--bg-brand-secondary)'),
        'brand-secondary-hover': withAlpha('var(--bg-brand-secondary_hover)'),
        'brand-tertiary': withAlpha('var(--bg-brand-tertiary)'),
        'success-primary': withAlpha('var(--bg-success-primary)'),
        'success-secondary': withAlpha('var(--bg-success-secondary)'),
        'error-primary': withAlpha('var(--bg-error-primary)'),
        'error-secondary': withAlpha('var(--bg-error-secondary)'),
        'warning-primary': withAlpha('var(--bg-warning-primary)'),
        'warning-secondary': withAlpha('var(--bg-warning-secondary)'),
      },
      borderColor: {
        default: withAlpha('var(--border-default)'),
        subtle: withAlpha('var(--border-subtle)'),
        muted: withAlpha('var(--border-muted)'),
        faint: withAlpha('var(--border-faint)'),
        error: withAlpha('var(--border-error)'),
        'error-strong': withAlpha('var(--border-error-strong)'),
        brand: withAlpha('var(--border-brand)'),
        'brand-strong': withAlpha('var(--border-brand-strong)'),
        'brand-subtle': withAlpha('var(--border-brand-subtle)'),
        'brand-disabled': withAlpha('var(--text-brand-disabled)'),
        'brand-10': withAlpha('var(--brand-10)'),
        'brand-25': withAlpha('var(--brand-25)'),
        'brand-50': withAlpha('var(--brand-50)'),
        'brand-75': withAlpha('var(--brand-75)'),
        'brand-100': withAlpha('var(--brand-100)'),
        'brand-200': withAlpha('var(--brand-200)'),
        'brand-300': withAlpha('var(--brand-300)'),
        'brand-400': withAlpha('var(--brand-400)'),
        'brand-500': withAlpha('var(--brand-500)'),
        'brand-600': withAlpha('var(--brand-600)'),
        'brand-700': withAlpha('var(--brand-700)'),
        'brand-800': withAlpha('var(--brand-800)'),
        'brand-900': withAlpha('var(--brand-900)'),
        'brand-950': withAlpha('var(--brand-950)'),
        strong: withAlpha('var(--border-strong)'),
        disabled: withAlpha('var(--border-disabled)'),
        focus: withAlpha('var(--border-focus)'),
        warning: withAlpha('var(--border-warning)'),
        success: withAlpha('var(--border-success)'),
        purple: withAlpha('var(--border-purple)'),
        'status-green': withAlpha('var(--border-status-green)'),
        'status-orange': withAlpha('var(--border-status-orange)'),
        'status-yellow': withAlpha('var(--border-status-yellow)'),
        'status-red': withAlpha('var(--border-status-red)'),
        primary: withAlpha('var(--border-primary)'),
        secondary: withAlpha('var(--border-secondary)'),
        tertiary: withAlpha('var(--border-tertiary)'),
        quaternary: withAlpha('var(--border-quaternary)'),
      },
      textColor: {
        primary: withAlpha('var(--text-primary)'),
        secondary: withAlpha('var(--text-secondary)'),
        'secondary-hover': withAlpha('var(--text-secondary-hover)'),
        tertiary: withAlpha('var(--text-tertiary)'),
        'tertiary-hover': withAlpha('var(--text-tertiary-hover)'),
        quaternary: withAlpha('var(--text-quaternary)'),
        disabled: withAlpha('var(--text-disabled)'),
        'error-primary': withAlpha('var(--text-error-primary)'),
        'error-secondary': withAlpha('var(--text-error-secondary)'),
        'error-tertiary': withAlpha('var(--text-error-tertiary)'),
        'warning-primary': withAlpha('var(--text-warning-primary)'),
        'warning-secondary': withAlpha('var(--text-warning-secondary)'),
        'warning-tertiary': withAlpha('var(--text-warning-tertiary)'),
        'success-primary': withAlpha('var(--text-success-primary)'),
        'success-secondary': withAlpha('var(--text-success-secondary)'),
        'success-tertiary': withAlpha('var(--text-success-tertiary)'),
        placeholder: withAlpha('var(--text-placeholder)'),
        'control-active-foreground': withAlpha(
          'var(--text-control-active-foreground)'
        ),
        purple: withAlpha('var(--text-purple)'),
        'brand-primary': withAlpha('var(--text-brand-primary)'),
        'brand-secondary': withAlpha('var(--text-brand-secondary)'),
        'brand-tertiary': withAlpha('var(--text-brand-tertiary)'),
        'brand-disabled': withAlpha('var(--text-brand-disabled)'),
        'brand-on-fill': withAlpha('var(--text-brand-on-fill)'),
        'chart-on-color': withAlpha('var(--chart-on-color)'),
        link: withAlpha('var(--text-link)'),
        'link-hover': withAlpha('var(--text-link-hover)'),
        'status-green': withAlpha('var(--text-status-green)'),
        'status-orange': withAlpha('var(--text-status-orange)'),
        'status-yellow': withAlpha('var(--text-status-yellow)'),
        'status-red': withAlpha('var(--text-status-red)'),
        'button-primary': withAlpha('var(--text-button-primary)'),
        /* icon colors — separate namespace so icons can diverge from text */
        'icon-primary': withAlpha('var(--icon-primary)'),
        'icon-secondary': withAlpha('var(--icon-secondary)'),
        'icon-tertiary': withAlpha('var(--icon-tertiary)'),
        'icon-disabled': withAlpha('var(--icon-disabled)'),
        'icon-brand': withAlpha('var(--icon-brand)'),
        'icon-brand-fill': withAlpha('var(--icon-brand-fill)'),
        'icon-brand-background': withAlpha('var(--icon-brand-background)'),
        'icon-error': withAlpha('var(--icon-error)'),
        'icon-success': withAlpha('var(--icon-success)'),
        'icon-warning': withAlpha('var(--icon-warning)'),
      },
      colors: {
        popover: {
          DEFAULT: withAlpha('var(--bg-elevated)'),
        },
        background: {
          DEFAULT: withAlpha('var(--bg-elevated)'),
        },
        // Theme-invariant promo palette (see --promo-* in index.css): keeps the
        // "Get more" upsell cards fixed navy across light/dark. Usable as
        // bg-/text-/border-/fill-promo-* via the base colors scale.
        promo: {
          surface: withAlpha('var(--promo-surface)'),
          accent: withAlpha('var(--promo-accent)'),
          title: withAlpha('var(--promo-title)'),
        },
        ls: {
          blue: 'hsl(211.5, 91.8%, 61.8%)',
          white: 'var(--white)',
          black: 'var(--black)',
          red: {
            25: withAlpha('var(--red-25)'),
            50: withAlpha('var(--red-50)'),
            100: withAlpha('var(--red-100)'),
            200: withAlpha('var(--red-200)'),
            300: withAlpha('var(--red-300)'),
            400: withAlpha('var(--red-400)'),
            500: withAlpha('var(--red-500)'),
            600: withAlpha('var(--red-600)'),
            700: withAlpha('var(--red-700)'),
            800: withAlpha('var(--red-800)'),
            900: withAlpha('var(--red-900)'),
            950: withAlpha('var(--red-950)'),
          },
          orange: {
            25: withAlpha('var(--orange-25)'),
            50: withAlpha('var(--orange-50)'),
            100: withAlpha('var(--orange-100)'),
            200: withAlpha('var(--orange-200)'),
            300: withAlpha('var(--orange-300)'),
            400: withAlpha('var(--orange-400)'),
            500: withAlpha('var(--orange-500)'),
            600: withAlpha('var(--orange-600)'),
            700: withAlpha('var(--orange-700)'),
            800: withAlpha('var(--orange-800)'),
            900: withAlpha('var(--orange-900)'),
            950: withAlpha('var(--orange-950)'),
          },
          neutral: {
            25: withAlpha('var(--neutral-25)'),
            35: withAlpha('var(--neutral-35)'),
            50: withAlpha('var(--neutral-50)'),
            100: withAlpha('var(--neutral-100)'),
            200: withAlpha('var(--neutral-200)'),
            300: withAlpha('var(--neutral-300)'),
            400: withAlpha('var(--neutral-400)'),
            500: withAlpha('var(--neutral-500)'),
            600: withAlpha('var(--neutral-600)'),
            700: withAlpha('var(--neutral-700)'),
            800: withAlpha('var(--neutral-800)'),
            900: withAlpha('var(--neutral-900)'),
            950: withAlpha('var(--neutral-950)'),
          },
          green: {
            25: withAlpha('var(--green-25)'),
            50: withAlpha('var(--green-50)'),
            100: withAlpha('var(--green-100)'),
            200: withAlpha('var(--green-200)'),
            300: withAlpha('var(--green-300)'),
            400: withAlpha('var(--green-400)'),
            500: withAlpha('var(--green-500)'),
            600: withAlpha('var(--green-600)'),
            700: withAlpha('var(--green-700)'),
            800: withAlpha('var(--green-800)'),
            900: withAlpha('var(--green-900)'),
            950: withAlpha('var(--green-950)'),
          },
          acid: {
            10: withAlpha('var(--acid-10)'),
            25: withAlpha('var(--acid-25)'),
            50: withAlpha('var(--acid-50)'),
            100: withAlpha('var(--acid-100)'),
            200: withAlpha('var(--acid-200)'),
            300: withAlpha('var(--acid-300)'),
            400: withAlpha('var(--acid-400)'),
            500: withAlpha('var(--acid-500)'),
            600: withAlpha('var(--acid-600)'),
            700: withAlpha('var(--acid-700)'),
            800: withAlpha('var(--acid-800)'),
            900: withAlpha('var(--acid-900)'),
            950: withAlpha('var(--acid-950)'),
          },
          purple: {
            10: withAlpha('var(--purple-10)'),
            25: withAlpha('var(--purple-25)'),
            50: withAlpha('var(--purple-50)'),
            100: withAlpha('var(--purple-100)'),
            200: withAlpha('var(--purple-200)'),
            300: withAlpha('var(--purple-300)'),
            400: withAlpha('var(--purple-400)'),
            500: withAlpha('var(--purple-500)'),
            600: withAlpha('var(--purple-600)'),
            700: withAlpha('var(--purple-700)'),
            800: withAlpha('var(--purple-800)'),
            900: withAlpha('var(--purple-900)'),
            950: withAlpha('var(--purple-950)'),
          },
        },
        brand: {
          ...brandPalette,
        },
        border: {
          default: withAlpha('var(--border-default)'),
          subtle: withAlpha('var(--border-subtle)'),
          muted: withAlpha('var(--border-muted)'),
          faint: withAlpha('var(--border-faint)'),
          strong: withAlpha('var(--border-strong)'),
        },
      },
      keyframes: {
        hide: {
          from: { opacity: 1 },
          to: { opacity: 0 },
        },
        marquee: {
          from: { transform: 'translateX(0)' },
          to: { transform: 'translateX(-50%)' },
        },
        slideIn: {
          from: {
            transform: 'translateX(calc(100% + var(--viewport-padding)))',
          },
          to: { transform: 'translateX(0)' },
        },
        swipeOut: {
          from: { transform: 'translateX(var(--radix-toast-swipe-end-x))' },
          to: { transform: 'translateX(calc(100% + var(--viewport-padding)))' },
        },
        'logo-pulse': {
          '0%, 100%': { opacity: '0.2' },
          '30%': { opacity: '1' },
          '60%': { opacity: '0.2' },
        },
        'collapsible-down': {
          from: { height: '0', opacity: '0' },
          to: {
            height: 'var(--radix-collapsible-content-height)',
            opacity: '1',
          },
        },
        'collapsible-up': {
          from: {
            height: 'var(--radix-collapsible-content-height)',
            opacity: '1',
          },
          to: { height: '0', opacity: '0' },
        },
        'linear-progress-indeterminate': {
          '0%, 100%': { left: '0%', width: '12.5%' },
          '25%, 75%': { width: '25%' },
          '50%': { left: '87.5%', width: '12.5%' },
        },
      },
      animation: {
        hide: 'hide 100ms ease-in',
        marquee: 'marquee 24s linear infinite',
        slideIn: 'slideIn 150ms cubic-bezier(0.16, 1, 0.3, 1)',
        swipeOut: 'swipeOut 100ms ease-out',
        'logo-pulse': 'logo-pulse 1.8s ease-in-out infinite',
        'collapsible-down':
          'collapsible-down 300ms cubic-bezier(0.16, 1, 0.3, 1)',
        'collapsible-up': 'collapsible-up 200ms cubic-bezier(0.4, 0, 0.6, 1)',
        'linear-progress-indeterminate':
          'linear-progress-indeterminate 2.5s ease-in-out infinite',
      },
    },
    typography: {
      playground: {
        css: {
          'h1, h2, h3, h4, h5, h6': {
            fontWeight: 'bold',
          },
          h1: {
            fontSize: '24px',
          },
          h2: {
            fontSize: '20px',
          },
          h3: {
            fontSize: '18px',
          },
          h4: {
            fontSize: '16px',
          },
          h5: {
            fontSize: '14px',
          },
          h6: {
            fontSize: '12px',
          },
          ul: {
            marginLeft: '20px !important',
            listStyleType: 'disc !important',
          },
          ol: {
            marginLeft: '20px !important',
            listStyleType: 'decimal !important',
          },
          a: {
            color: '#287977',
            textDecoration: 'underline',
            '&:hover': {
              textDecoration: 'underline',
            },
          },
          table: {
            width: '100%',
            borderCollapse: 'collapse',
            th: {
              padding: '0.5rem',
              border: '1px solid var(--neutral-100)',
              fontWeight: 'bold',
              textAlign: 'left',
            },
            td: {
              padding: '0.5rem',
              border: '1px solid var(--neutral-100)',
            },
          },
          blockquote: {
            borderLeft: '2px solid var(--neutral-100)',
            paddingLeft: '1rem',
            marginLeft: '0',
            fontStyle: 'italic',
          },

          's, strike, del': {
            textDecoration: 'line-through',
          },
        },
      },
    },
  },
  plugins: [
    require('@tailwindcss/container-queries'),
    require('@tailwindcss/typography'),
    require('@tailwindcss/forms'),
    require('tailwindcss-animate'),
    require('@headlessui/tailwindcss'),
    plugin(({ addUtilities, addBase }) => {
      addBase({
        // Publish the total height of any mounted platform banner rows on
        // `:root` so the `h-screen` / `min-h-screen` override above can
        // subtract it from 100vh. Each `.platform-banner-row` is `h-[52px]`;
        // at most two platform banners can stack today (SaaS shows
        // IncidentBanner + TenantUsageLimitsBanner concurrently). Keep
        // `52px` in sync with `h-[52px]` on the banner rows.
        ':root:has(.platform-banner-row)': {
          '--banner-height': '52px',
        },
        ':root:has(.platform-banner-row + .platform-banner-row)': {
          '--banner-height': '104px',
        },
        input: {
          borderWidth: '0',
          padding: '0',
        },
        // Global scrollbar styles for all scrollable elements
        'html, body, *': {
          'scrollbar-width': 'thin',
          'scrollbar-color': 'var(--scrollbar-thumb) transparent',
        },
        'html::-webkit-scrollbar, body::-webkit-scrollbar, *::-webkit-scrollbar':
          {
            width: '8px',
            background: 'transparent',
          },
        'html::-webkit-scrollbar-track, body::-webkit-scrollbar-track, *::-webkit-scrollbar-track':
          {
            background: 'transparent',
          },
        'html::-webkit-scrollbar-thumb, body::-webkit-scrollbar-track, *::-webkit-scrollbar-thumb':
          {
            background: 'var(--scrollbar-thumb)',
            'border-radius': '4px',
          },
        'html::-webkit-scrollbar-thumb:hover, body::-webkit-scrollbar-thumb:hover, *::-webkit-scrollbar-thumb:hover':
          {
            background: 'var(--scrollbar-thumb-hover)',
          },
      });
      addUtilities({
        '.no-scrollbar': {
          'scrollbar-width': 'none',
          '&::-webkit-scrollbar': {
            display: 'none',
          },
        },
        '.scrollbar-thin': {
          'scrollbar-width': 'thin',
        },
      });

      // https://github.com/tailwindlabs/tailwindcss/discussions/12127
      addUtilities({
        '.break-anywhere': {
          '@supports (overflow-wrap: anywhere)': {
            'overflow-wrap': 'anywhere',
          },
          '@supports not (overflow-wrap: anywhere)': {
            'word-break': 'break-word',
          },
        },
      });

      addUtilities({
        '.no-number-spinner': {
          MozAppearance: 'textfield',
          '&::-webkit-outer-spin-button': {
            WebkitAppearance: 'none !important',
            margin: 0,
          },
          '&::-webkit-inner-spin-button': {
            WebkitAppearance: 'none !important',
            margin: 0,
          },
        },
      });

      addUtilities({
        '.text-security': {
          textSecurity: 'disc',
          WebkitTextSecurity: 'disc',
          MozTextSecurity: 'disc',
        },
      });

      addUtilities({
        '.button-primary-foreground': {
          color: 'var(--button-primary-foreground)',
        },
        '.button-primary-foreground-hover': {
          color: 'var(--button-primary-foreground-hover)',
        },
        '.button-primary-bg': { backgroundColor: 'var(--button-primary-bg)' },
        '.button-primary-bg-hover': {
          backgroundColor: 'var(--button-primary-bg-hover)',
        },
        '.button-primary-border': {
          borderColor: 'var(--button-primary-border)',
        },
        '.button-primary-border-hover': {
          borderColor: 'var(--button-primary-border-hover)',
        },
        '.button-primary-underlined-foreground': {
          color: 'var(--button-primary-underlined-foreground)',
        },
        '.button-primary-underlined-foreground-hover': {
          color: 'var(--button-primary-underlined-foreground-hover)',
        },
        '.button-primary-underlined-border': {
          borderColor: 'var(--button-primary-underlined-border)',
        },
        '.button-primary-underlined-border-hover': {
          borderColor: 'var(--button-primary-underlined-border-hover)',
        },

        '.button-primary-error-foreground': {
          color: 'var(--button-primary-error-foreground)',
        },
        '.button-primary-error-foreground-hover': {
          color: 'var(--button-primary-error-foreground-hover)',
        },
        '.button-primary-error-bg': {
          backgroundColor: 'var(--button-primary-error-bg)',
        },
        '.button-primary-error-bg-hover': {
          backgroundColor: 'var(--button-primary-error-bg-hover)',
        },
        '.button-primary-error-border': {
          borderColor: 'var(--button-primary-error-border)',
        },
        '.button-primary-error-border-hover': {
          borderColor: 'var(--button-primary-error-border-hover)',
        },

        '.button-secondary-foreground': {
          color: 'var(--button-secondary-foreground)',
        },
        '.button-secondary-foreground-hover': {
          color: 'var(--button-secondary-foreground-hover)',
        },
        '.button-secondary-bg': {
          backgroundColor: 'var(--button-secondary-bg)',
        },
        '.button-secondary-bg-hover': {
          backgroundColor: 'var(--button-secondary-bg-hover)',
        },
        '.button-secondary-border': {
          borderColor: 'var(--button-secondary-border)',
        },
        '.button-secondary-border-hover': {
          borderColor: 'var(--button-secondary-border-hover)',
        },
        '.button-secondary-underlined-border': {
          borderColor: 'var(--button-secondary-underlined-border)',
        },
        '.button-secondary-underlined-border-hover': {
          borderColor: 'var(--button-secondary-underlined-border-hover)',
        },

        '.button-secondary-color-foreground': {
          color: 'var(--button-secondary-color-foreground)',
        },
        '.button-secondary-color-foreground-hover': {
          color: 'var(--button-secondary-color-foreground-hover)',
        },
        '.button-secondary-color-bg': {
          backgroundColor: 'var(--button-secondary-color-bg)',
        },
        '.button-secondary-color-bg-hover': {
          backgroundColor: 'var(--button-secondary-color-bg-hover)',
        },
        '.button-secondary-color-border': {
          borderColor: 'var(--button-secondary-color-border)',
        },
        '.button-secondary-color-border-hover': {
          borderColor: 'var(--button-secondary-color-border-hover)',
        },

        '.button-error-underlined-foreground-hover': {
          color: 'var(--button-error-underlined-foreground-hover)',
        },
        '.button-error-underlined-border': {
          borderColor: 'var(--button-error-underlined-border)',
        },
        '.button-error-underlined-border-hover': {
          borderColor: 'var(--button-error-underlined-border-hover)',
        },
        '.button-error-disabled-foreground': {
          color: 'var(--button-error-disabled-foreground)',
        },
        '.button-error-disabled-border': {
          borderColor: 'var(--button-error-disabled-border)',
        },
      });

      addUtilities({
        '.display-sm': {
          fontSize: '1rem', // 16px
          lineHeight: '1.5rem', // 24px
          fontWeight: '600', // semibold
        },
        '.display-base': {
          fontSize: '1.5rem', // 24px
          lineHeight: '2rem', // 32px
          letterSpacing: '-0.01em', // tracking-tight
        },
        '.display-lg': {
          fontSize: '1.875rem', // 30px
          lineHeight: '2.375rem', // 38px
          letterSpacing: '-0.01em', // tracking-tight
        },
        '.display-xl': {
          fontSize: '2.25rem', // 36px
          lineHeight: '2.75rem', // 44px
          letterSpacing: '-0.01em', // tracking-tight
        },
        '.display-2xl': {
          fontSize: '3rem', // 48px
          lineHeight: '3.75rem', // 60px
          letterSpacing: '-0.01em', // tracking-tight
        },
        '.caps-label-sm': {
          fontSize: '0.875rem', // 14px
          lineHeight: '1.25rem', // 20px
          letterSpacing: '0.02625rem', // 0.42px
          textTransform: 'uppercase',
        },
        '.caps-label-xs': {
          fontSize: '0.75rem', // 14px
          lineHeight: '1.125rem', // 20px
          letterSpacing: '0.0225rem', // 0.42px
          textTransform: 'uppercase',
        },
      });
    }),
  ],
};
