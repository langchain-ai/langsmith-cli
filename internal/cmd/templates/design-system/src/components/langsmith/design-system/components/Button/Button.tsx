import {
  type ButtonHTMLAttributes,
  type ReactElement,
  cloneElement,
  forwardRef,
  isValidElement,
} from 'react';

import { Loading01Icon } from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';
import { Badge } from '../Badge';
import { Text } from '../Text';
import { buttonStyleMap } from './constants';

// Type to constrain decorators to only accept icons from @langchain/untitled-ui-icons
export type IconComponent = React.ComponentType<React.SVGProps<SVGSVGElement>>;

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** Button size variant */
  size?: 'xs' | 'sm' | 'md';
  /** Button color scheme */
  color?: 'primary' | 'secondary' | 'error';
  /** Button visual variant */
  variant?: 'normal' | 'outlined' | 'plain' | 'underlined';
  /** Click handler */
  onClick?: (event: React.MouseEvent<HTMLButtonElement>) => void;
  /** Icon component from @langchain/untitled-ui-icons to display on the left side */
  leftDecorator?: IconComponent;
  /** Icon component from @langchain/untitled-ui-icons to display on the right side */
  rightDecorator?: IconComponent;
  /** Whether the button is disabled */
  disabled?: boolean;
  /** Tag text to display */
  tagText?: string;
  /** Position of the tag */
  tagPosition?: 'left' | 'right';
  /** Button content */
  children: React.ReactNode;
  /** Whether the button is loading */
  loading?: boolean;
  /** Pass through the `<button>` props to a child component */
  as?: ReactElement<{ children: React.ReactNode; [key: string]: unknown }>;
}

const Spinner = () => {
  return <Loading01Icon className="size-4 animate-spin" />;
};

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      color = 'primary',
      variant = 'normal',
      size = 'sm',
      onClick,
      leftDecorator: LeftIcon,
      rightDecorator: RightIcon,
      disabled = false,
      tagText,
      tagPosition = 'right',
      className,
      children,
      loading = false,
      as,
      ...props
    },
    ref
  ) => {
    const baseStyles = cn(
      'lc-button relative inline-flex flex-none items-center justify-center gap-1.5 truncate transition-all duration-200',
      '[&_*]:[text-box-trim:trim-both]',
      {
        'text-xxs leading-[1.15] tracking-normal': size === 'xs',
        'text-xs leading-tight tracking-snug': size === 'sm',
        'text-sm leading-[1.15] tracking-tighter': size === 'md',
      },
      {
        'py-space-1': size === 'xs' || size === 'sm',
        'py-space-2': size === 'md',
      },
      {
        'px-space-2': variant !== 'underlined',
        'px-0': variant === 'underlined',
      },
      {
        'shadow-[0px_1px_2px_0px_var(--shadow-color-subtle)]':
          (variant === 'normal' || variant === 'outlined') && !disabled,
      },
      variant === 'underlined'
        ? 'rounded-none'
        : size === 'xs'
          ? 'rounded-[3px]'
          : `rounded-${size}`
    );

    // Generate unique class combinations for each color/variant combination
    const getColorVariantStyles = () => {
      const state = disabled ? 'disabled' : 'enabled';
      const key = `${color}-${variant}-${state}` as keyof typeof buttonStyleMap;
      return buttonStyleMap[key] || '';
    };

    // Generate aria-label automatically based on content
    const getAriaLabel = () => {
      if (props['aria-label']) {
        return props['aria-label'];
      }

      // If children is a string, use it as aria-label
      if (typeof children === 'string') {
        return children;
      }
      // If children is not a string but tagText exists, use tagText
      if (tagText) {
        return tagText;
      }

      // Fallback to generic "button"
      return 'button';
    };

    const content = (
      <>
        {LeftIcon && (
          <LeftIcon
            className={cn(
              'size-4 flex-shrink-0',
              '[&_path]:stroke-[1.5px]',
              size === 'xs' && 'size-3'
            )}
          />
        )}

        {tagText && tagPosition === 'left' && (
          <Badge
            color={color}
            size="xs"
            rounded="xs"
            className={cn(disabled && 'opacity-50')}
          >
            {tagText}
          </Badge>
        )}

        {typeof children === 'string' ? (
          <Text variant={size} weight="normal" as="span">
            {children}
          </Text>
        ) : (
          children
        )}

        {tagText && tagPosition === 'right' && (
          <Badge
            color={color}
            size="xs"
            rounded="xs"
            className={cn(disabled && 'opacity-50')}
          >
            {tagText}
          </Badge>
        )}

        {RightIcon && (
          <RightIcon
            className={cn(
              'size-4 flex-shrink-0',
              '[&_path]:stroke-[1.5px]',
              size === 'xs' && 'size-3'
            )}
          />
        )}
      </>
    );

    if (as) {
      if (!isValidElement(as)) return null;

      return cloneElement(as, {
        className: cn(baseStyles, getColorVariantStyles(), className),
        onClick,
        'aria-label': getAriaLabel(),
        'aria-disabled': disabled || undefined,
        ref,
        ...props,
        children: loading ? (
          <>
            <span className="invisible flex items-center">{content}</span>
            <span className="absolute">
              <Spinner />
            </span>
          </>
        ) : (
          content
        ),
      });
    }

    return (
      // eslint-disable-next-line react/forbid-elements
      <button
        ref={ref}
        type="button"
        className={cn(baseStyles, getColorVariantStyles(), className)}
        disabled={disabled}
        onClick={onClick}
        aria-label={getAriaLabel()}
        {...props}
      >
        {loading ? (
          <>
            <span className="invisible flex items-center">{content}</span>
            <span className="absolute">
              <Spinner />
            </span>
          </>
        ) : (
          content
        )}
      </button>
    );
  }
);

Button.displayName = 'Button';

export { Button };
export type { ButtonProps };
