import type { MouseEventHandler } from 'react';
import { Children, cloneElement, isValidElement } from 'react';

import { cn } from '../../utils/cn';
import type { ButtonProps } from '../Button';
import type { IconButtonProps } from '../IconButton';

interface ButtonGroupProps {
  /** Button group color scheme to propagate to children */
  color?: 'primary' | 'secondary';
  /** Button group variant to propagate to children */
  variant?: 'normal' | 'outlined' | 'plain';
  /** Button group size to propagate to children */
  size?: 'sm' | 'md';
  /** Additional CSS classes */
  className?: string;
  /** Children components (Button or IconButton) */
  children: React.ReactNode;
  onMouseEnter?: MouseEventHandler<HTMLDivElement>;
  onMouseLeave?: MouseEventHandler<HTMLDivElement>;
}

function ButtonGroup({
  color,
  variant,
  size,
  className,
  children,
  onMouseEnter,
  onMouseLeave,
}: ButtonGroupProps) {
  const baseStyles = cn(
    'inline-flex items-stretch',
    '[&>.lc-button]:h-auto', // Keep mixed button types the same height
    '[&>*:not(:first-child)]:border-l-0', // Remove left border on all but first child
    '[&>*:not(:first-child):not(:last-child)]:rounded-none', // Remove border radius on middle children
    '[&>*:first-child]:rounded-r-none', // Remove right border radius on first child
    '[&>*:last-child]:rounded-l-none', // Remove left border radius on last child
    '[&>*:only-child]:rounded', // Preserve border radius when only one child
    className
  );

  const clonedChildren = Children.map(children, (child) => {
    if (isValidElement<ButtonProps | IconButtonProps>(child)) {
      // Only override props if they weren't explicitly set on the child
      const props: Partial<ButtonProps & IconButtonProps> = {
        color,
        size,
        variant,
      };

      return cloneElement(child, props);
    }
    return child;
  });

  return (
    <div
      className={baseStyles}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
    >
      {clonedChildren}
    </div>
  );
}

ButtonGroup.displayName = 'ButtonGroup';

export { ButtonGroup };
export type { ButtonGroupProps };
