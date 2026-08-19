import { forwardRef } from 'react';

import { cn } from '../../utils/cn';

interface KbdProps extends React.HTMLAttributes<HTMLElement> {
  /** Color scheme. `inherit` adopts the ambient text color, for chips on colored/dark surfaces. */
  variant?: 'default' | 'inherit';
  /** Key content — a label string or an icon element. */
  children?: React.ReactNode;
}

const VARIANT_CLASSES: Record<NonNullable<KbdProps['variant']>, string> = {
  default: 'border-subtle text-tertiary',
  inherit: 'border-current bg-transparent text-current',
};

const Kbd = forwardRef<HTMLElement, KbdProps>(
  ({ variant = 'default', className, children, ...props }, ref) => (
    <kbd
      ref={ref}
      className={cn(
        'inline-flex h-[1.125rem] min-w-[1.125rem] items-center justify-center rounded border px-0.5 font-sans text-[0.625rem] font-medium leading-none',
        VARIANT_CLASSES[variant],
        className
      )}
      {...props}
    >
      {children}
    </kbd>
  )
);

Kbd.displayName = 'Kbd';

interface KbdGroupProps extends React.HTMLAttributes<HTMLElement> {
  /** The `Kbd` chips making up the shortcut, in order. */
  children?: React.ReactNode;
}

/** Groups multiple `Kbd` chips into a single keyboard shortcut, e.g. `⌘ K`. */
const KbdGroup = forwardRef<HTMLElement, KbdGroupProps>(
  ({ className, children, ...props }, ref) => (
    <kbd
      ref={ref}
      className={cn('inline-flex items-center gap-space-1', className)}
      {...props}
    >
      {children}
    </kbd>
  )
);

KbdGroup.displayName = 'KbdGroup';

export { Kbd, KbdGroup };
export type { KbdProps, KbdGroupProps };
