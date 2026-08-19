import { forwardRef } from 'react';

import { SearchSmIcon } from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';
import { Icon } from '../Icon';
import type { IconComponent, IconProps } from '../Icon';
import { Link } from '../Link';
import { Text } from '../Text';
import type { TextProps } from '../Text';

type EmptyStateSize = 'sm' | 'md' | 'lg';
type EmptyStateVariant = 'neutral' | 'brand';

interface EmptyStateSizeStyles {
  container: string;
  iconSize: NonNullable<IconProps['size']>;
  content: string;
  titleVariant: NonNullable<TextProps['variant']>;
  titleWeight?: TextProps['weight'];
  descriptionVariant: NonNullable<TextProps['variant']>;
  footer: string;
  linkVariant: NonNullable<TextProps['variant']>;
}

const sizeStyles: Record<EmptyStateSize, EmptyStateSizeStyles> = {
  sm: {
    container: 'min-h-0 gap-space-3 py-space-3',
    iconSize: 'lg',
    content: 'max-w-xs gap-space-1',
    titleVariant: 'md',
    titleWeight: 'semibold',
    descriptionVariant: 'sm',
    footer: 'gap-space-3',
    linkVariant: 'sm',
  },
  md: {
    container: 'min-h-[13.5rem] gap-space-4 py-space-4',
    iconSize: 'xl',
    content: 'max-w-xs gap-space-2',
    titleVariant: 'h3',
    descriptionVariant: 'body',
    footer: 'gap-space-4',
    linkVariant: 'body',
  },
  lg: {
    container: 'min-h-[13.5rem] gap-space-4 py-space-5',
    iconSize: 'xl',
    content: 'max-w-md gap-space-3',
    titleVariant: 'h2',
    descriptionVariant: 'body',
    footer: 'gap-space-4',
    linkVariant: 'body',
  },
};

interface EmptyStateProps {
  /** Icon rendered above the title. Defaults to a search icon. */
  icon?: IconComponent;
  /** Primary heading. */
  title?: string;
  /** Supporting copy below the title. */
  description?: React.ReactNode;
  /** Action node (e.g. a create button) rendered in the footer row. */
  action?: React.ReactNode;
  /** Controls the icon's semantic color treatment. */
  variant?: EmptyStateVariant;
  /** When set, renders a "Learn more" link to this docs URL in the footer row. */
  docsLink?: string;
  /** Extra classes for the outer container. */
  className?: string;
  /** Controls the typography, icon, spacing, and text-block width. */
  size?: EmptyStateSize;
}

const EmptyState = forwardRef<HTMLDivElement, EmptyStateProps>(
  (
    {
      icon,
      title,
      description,
      action,
      variant = 'neutral',
      docsLink,
      className,
      size = 'md',
    },
    ref
  ) => {
    const styles = sizeStyles[size];

    return (
      <div
        ref={ref}
        className={cn(
          'flex flex-col items-center justify-center',
          styles.container,
          className
        )}
      >
        <Icon
          icon={icon ?? SearchSmIcon}
          color={variant}
          rounded
          size={styles.iconSize}
          className={variant === 'neutral' ? 'bg-surface-level-2' : undefined}
        />
        <div className={cn('flex flex-col', styles.content)}>
          <Text
            as="h3"
            variant={styles.titleVariant}
            weight={styles.titleWeight}
            className="text-center"
          >
            {title}
          </Text>
          <Text
            variant={styles.descriptionVariant}
            color="tertiary"
            className="text-center"
          >
            {description}
          </Text>
        </div>
        <div className={cn('flex items-center', styles.footer)}>
          {action}
          {docsLink && (
            <Link
              href={docsLink}
              target="_blank"
              rel="noopener noreferrer"
              variant={styles.linkVariant}
            >
              Learn more
            </Link>
          )}
        </div>
      </div>
    );
  }
);

EmptyState.displayName = 'EmptyState';

export { EmptyState };
export type { EmptyStateProps, EmptyStateSize, EmptyStateVariant };
