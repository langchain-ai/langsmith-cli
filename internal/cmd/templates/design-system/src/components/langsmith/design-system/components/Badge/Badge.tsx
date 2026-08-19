import type { ReactElement } from 'react';
import { forwardRef } from 'react';

import { cn } from '../../utils/cn';
import { Text, type TextProps } from '../Text';

// Only allow string or SVG icon as children
interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  /** Badge style preset */
  variant?: 'default' | 'manifestPreview';
  rounded?: 'full' | 'none' | 'xs' | 'sm';
  /** Badge color scheme */
  color?:
    | 'primary'
    | 'secondary'
    | 'success'
    | 'error'
    | 'warning'
    | 'special'
    | 'plain';
  /** Badge size variant */
  size?: 'xxs' | 'xs' | 'sm' | 'md';
  /** Left decorator icon */
  leftDecorator?: React.ComponentType<React.SVGProps<SVGSVGElement>>;

  /** Right decorator icon */
  rightDecorator?: React.ComponentType<React.SVGProps<SVGSVGElement>>;

  /** Badge content */
  children?: string | ReactElement<SVGSVGElement>;

  /** Font weight of text content */
  textWeight?: TextProps['weight'];
}

type BadgeSize = NonNullable<BadgeProps['size']>;

const SIZE_BOX_CLASSES: Record<BadgeSize, string> = {
  xxs: 'gap-0.5 px-space-1 py-px',
  xs: 'gap-space-1 px-1.5 py-0.5',
  sm: 'gap-space-1 px-1.5 py-0.5',
  md: 'gap-space-1 px-space-2 py-space-1',
};

const SIZE_TEXT_VARIANT: Record<BadgeSize, 'xs' | 'sm'> = {
  xxs: 'xs',
  xs: 'xs',
  sm: 'sm',
  md: 'sm',
};

const SIZE_DEFAULT_WEIGHT: Record<
  BadgeSize,
  NonNullable<TextProps['weight']>
> = {
  xxs: 'normal',
  xs: 'medium',
  sm: 'medium',
  md: 'medium',
};

const Badge = forwardRef<HTMLSpanElement, BadgeProps>(
  (
    {
      variant = 'default',
      color,
      size,
      rounded = 'full',
      leftDecorator: LeftIcon,
      rightDecorator: RightIcon,
      className,
      children,
      textWeight,
      ...props
    },
    ref
  ) => {
    const isString = typeof children === 'string';
    const isManifestPreview = variant === 'manifestPreview';
    const resolvedColor = color ?? (isManifestPreview ? 'plain' : 'secondary');
    const resolvedSize = size ?? (isManifestPreview ? 'sm' : 'md');
    const resolvedTextWeight = textWeight ?? SIZE_DEFAULT_WEIGHT[resolvedSize];
    const baseStyles = cn(
      'inline-flex items-center justify-center border border-transparent',
      SIZE_BOX_CLASSES[resolvedSize],
      {
        // TODO: we're only going to have 2 variants for the rounded prop. need to remove/rename the rounded prop
        'rounded-xs':
          rounded === 'xs' || rounded === 'sm' || rounded === 'none',
        'rounded-full': rounded === 'full',
        border: resolvedColor === 'plain',
      }
    );

    const getColorStyles = () => {
      switch (resolvedColor) {
        case 'primary':
          return 'bg-brand-secondary text-brand-primary';
        case 'success':
          return 'bg-success-secondary text-success-secondary';
        case 'error':
          return 'bg-error-secondary text-error-secondary';
        case 'warning':
          return 'bg-warning-secondary text-warning-secondary';
        case 'special':
          return 'bg-ls-acid-50 text-ls-acid-700 dark:bg-ls-acid-200';
        case 'plain':
          return 'bg-primary text-primary border-secondary';
        case 'secondary':
        default:
          return 'bg-tertiary text-secondary';
      }
    };

    return (
      <div
        className={cn(
          baseStyles,
          getColorStyles(),
          isManifestPreview &&
            'min-w-0 max-w-full overflow-hidden whitespace-nowrap [&>span]:truncate',
          isManifestPreview && color == null && 'bg-transparent text-secondary',
          className
        )}
        {...props}
      >
        {LeftIcon && <LeftIcon className={cn('size-3 flex-shrink-0')} />}
        {isString ? (
          <Text
            ref={ref}
            as="span"
            variant={SIZE_TEXT_VARIANT[resolvedSize]}
            weight={resolvedTextWeight}
          >
            {children}
          </Text>
        ) : (
          children
        )}
        {RightIcon && <RightIcon className={cn('size-3 flex-shrink-0')} />}
      </div>
    );
  }
);

Badge.displayName = 'Badge';

export { Badge };
export type { BadgeProps };
