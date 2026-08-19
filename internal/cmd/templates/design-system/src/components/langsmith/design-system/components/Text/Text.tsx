import { forwardRef, type JSX } from 'react';

import { cn } from '../../utils/cn';

interface TextProps extends React.HTMLAttributes<HTMLElement> {
  variant?:
    | 'h1'
    | 'h2'
    | 'h3'
    | 'h4'
    | 'h5'
    | 'h6'
    | 'md'
    | 'sm'
    | 'xs'
    | 'body';
  weight?: 'semibold' | 'medium' | 'normal';
  as?: 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6' | 'p' | 'span' | 'div' | 'label';
  color?:
    | 'primary'
    | 'secondary'
    | 'tertiary'
    | 'quaternary'
    | 'placeholder'
    | 'error'
    | 'success';
  children: React.ReactNode;
  /** Only if using as="label" to attach the label to the input */
  htmlFor?: string;
}

const textVariantClasses: Record<NonNullable<TextProps['variant']>, string> = {
  h1: 'text-2xl font-medium leading-tight tracking-tighter', // 24px, 500, 100%, -4%
  h2: 'text-lg font-medium leading-tight tracking-tighter', // 18px, 500, 120%, -4%
  h3: 'text-base font-semibold leading-tight tracking-tight', // 16px, 600 120%, -3%
  h4: 'text-sm font-normal leading-tight tracking-wide uppercase', // 14px, 400, 120%, 3%
  h5: 'text-sm font-normal leading-tight tracking-tight', // 14px, 400, 120%, -3%
  h6: 'text-sm font-normal leading-tight tracking-wide', // 14px, 400, 120%, 3%
  md: 'text-sm leading-[1.15] tracking-tighter', // 14px, 115%, -4%
  sm: 'text-xs leading-tight tracking-snug', // 13px, 120%, -2%
  xs: 'text-xxs leading-[1.15] tracking-normal', // 12px, 115%, 0%
  body: 'text-sm leading-normal tracking-normal', // 14px, 150%, 0%
};

const Text = forwardRef<HTMLElement, TextProps>(
  (
    { variant = 'body', weight, as, className, children, color, ...props },
    ref
  ) => {
    // Default semantic mapping
    const defaultElement = {
      h1: 'h1',
      h2: 'h2',
      h3: 'h3',
      h4: 'h4',
      h5: 'h5',
      h6: 'h6',
      md: 'span',
      sm: 'span',
      xs: 'span',
      body: 'p',
    }[variant] as keyof JSX.IntrinsicElements;

    const Component = (as || defaultElement) as React.ElementType;

    const variants = textVariantClasses;

    const weights = {
      semibold: 'font-semibold',
      medium: 'font-medium',
      normal: 'font-normal',
    };

    const colors = {
      primary: 'text-primary',
      secondary: 'text-secondary',
      tertiary: 'text-tertiary',
      quaternary: 'text-quaternary',
      placeholder: 'text-placeholder',
      error: 'text-error-secondary',
      success: 'text-success-secondary',
    };

    const weightClass = weight ? weights[weight] : '';

    return (
      <Component
        ref={ref}
        className={cn(
          variants[variant],
          weightClass,
          color ? colors[color] : '',
          className
        )}
        {...props}
      >
        {children}
      </Component>
    );
  }
);

Text.displayName = 'Text';

export { Text, textVariantClasses };
export type { TextProps };
