import { forwardRef } from 'react';

import { cn } from '../../utils/cn';
import type { TooltipProps } from '../Tooltip';
import { Tooltip } from '../Tooltip';

// Type to constrain icons to only accept icons from @langchain/untitled-ui-icons
export type IconComponent = React.ComponentType<React.SVGProps<SVGSVGElement>>;

export interface IconProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Icon component from @langchain/untitled-ui-icons to display */
  icon: IconComponent;
  /** Color scheme for the icon */
  color?:
    | 'neutral'
    | 'brand'
    | 'success'
    | 'warning'
    | 'info'
    | 'error'
    | 'special';
  /** Whether the background should be rounded */
  rounded?: boolean;
  /** Optional label - renders a tooltip when provided */
  label?: React.ReactNode;
  /** Size of the icon container */
  size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl';
  /** Class name for the icon */
  iconClassName?: string;
  /** Tooltip props */
  tooltipProps?: Omit<TooltipProps, 'children' | 'title'>;
}

const Icon = forwardRef<HTMLDivElement, IconProps>(
  (
    {
      icon: IconElement,
      color,
      rounded = false,
      label,
      size = 'md',
      className,
      iconClassName,
      tooltipProps,
      ...props
    },
    ref
  ) => {
    const baseStyles = cn(
      'inline-flex flex-shrink-0 items-center justify-center [&_path]:stroke-[1.5px]',
      color && {
        'p-[2px]': size === 'xs',
        'p-space-1': size === 'sm' || size === 'md',
        'p-space-2': size === 'lg',
        'p-space-3': size === 'xl',
      },
      color && {
        'rounded-full': rounded,
        'rounded-md': !rounded,
      }
    );

    const getColorStyles = () => {
      switch (color) {
        case 'success':
          return 'bg-success-subtle text-status-green dark:bg-success';
        case 'warning':
          return 'bg-warning-subtle dark:bg-warning text-status-orange';
        case 'brand':
        case 'info':
          return 'bg-brand-subtle text-brand-primary';
        case 'error':
          return 'bg-error-subtle text-status-red dark:bg-error';
        case 'special':
          return 'bg-ls-acid-50 text-ls-acid-700';
        case 'neutral':
          return 'bg-surface-level-4 text-secondary';
        default:
          return 'text-primary';
      }
    };

    const getIconSize = () => {
      switch (size) {
        case 'xs':
          return 'h-3 w-3';
        case 'sm':
          return 'h-3 w-3';
        case 'lg':
          return 'h-5 w-5';
        case 'xl':
          return 'h-6 w-6';
        case 'md':
        default:
          return 'h-4 w-4';
      }
    };

    const content = (
      <span
        ref={ref}
        className={cn(baseStyles, getColorStyles(), className)}
        {...props}
      >
        <IconElement
          className={cn(getIconSize(), 'flex-shrink-0', iconClassName)}
        />
      </span>
    );

    if (label) {
      return (
        <Tooltip
          {...tooltipProps}
          title={label}
          side={tooltipProps?.side ?? 'top'}
        >
          {content}
        </Tooltip>
      );
    }

    return content;
  }
);

Icon.displayName = 'Icon';

export { Icon };
