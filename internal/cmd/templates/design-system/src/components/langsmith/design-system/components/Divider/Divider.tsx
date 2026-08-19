import { forwardRef } from 'react';

import { cn } from '../../utils/cn';

interface DividerProps extends React.HTMLAttributes<HTMLElement> {
  orientation?: 'horizontal' | 'vertical';
  decorative?: boolean;
}

const Divider = forwardRef<HTMLElement, DividerProps>(
  (
    {
      orientation = 'horizontal',
      decorative = true,
      className,
      role,
      ...props
    },
    ref
  ) => {
    const accessibilityProps = decorative
      ? { 'aria-hidden': true }
      : {
          role: role ?? 'separator',
          'aria-orientation': orientation,
        };

    if (orientation === 'vertical') {
      return (
        <div
          ref={ref as React.Ref<HTMLDivElement>}
          className={cn('self-stretch border-l border-subtle', className)}
          {...accessibilityProps}
          {...props}
        />
      );
    }

    return (
      <hr
        ref={ref as React.Ref<HTMLHRElement>}
        className={cn('w-full border-0 border-t border-subtle', className)}
        {...accessibilityProps}
        {...props}
      />
    );
  }
);

Divider.displayName = 'Divider';

export { Divider };
export type { DividerProps };
