import type { ReactNode } from 'react';
import { useState } from 'react';

import {
  AlertSolidIcon,
  AlertTriangleSolidIcon,
  CheckCircleSolidIcon,
  InfoCircleSolidIcon,
  XIcon,
} from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';
import { Icon } from '../Icon';
import { IconButton } from '../IconButton';
import { Text } from '../Text';

export type BannerIntent = 'warning' | 'info' | 'success' | 'error' | 'neutral';

export type BannerProps = {
  title?: string;
  children?: ReactNode;
  dismissible?: boolean;
  className?: string;
  intent?: BannerIntent;
  shadow?: boolean;
  onDismiss?: () => void;
  icon?: ReactNode;
  action?: ReactNode;
  flush?: boolean;
};

export const Banner = ({
  title,
  children,
  dismissible = false,
  className,
  intent = 'warning',
  shadow = false,
  onDismiss,
  icon,
  action,
  flush = false,
}: BannerProps) => {
  const [show, setShow] = useState(true);

  const renderIcon = () => {
    if (icon !== undefined) {
      return icon;
    }
    switch (intent) {
      case 'warning':
        return (
          <Icon
            icon={AlertTriangleSolidIcon}
            size="md"
            className="text-warning-secondary"
          />
        );
      case 'info':
        return (
          <Icon
            icon={InfoCircleSolidIcon}
            size="md"
            className="text-brand-100 dark:text-brand-600"
          />
        );
      case 'success':
        return (
          <Icon
            icon={CheckCircleSolidIcon}
            size="md"
            className="text-success-secondary"
          />
        );
      case 'error':
        return (
          <Icon
            icon={AlertSolidIcon}
            size="md"
            className="text-error-secondary"
          />
        );
      case 'neutral':
        return (
          <Icon
            icon={InfoCircleSolidIcon}
            size="md"
            className="text-tertiary"
          />
        );
      default:
        return null;
    }
  };

  const titleElement =
    title == null ? null : (
      <Text
        as="span"
        variant="sm"
        weight="medium"
        color="primary"
        className={cn(
          'whitespace-normal break-words',
          children != null && 'mr-space-2'
        )}
      >
        {title}
      </Text>
    );

  const contentElement =
    children == null ? null : typeof children === 'string' ? (
      <Text
        as="span"
        variant="sm"
        weight="normal"
        color="secondary"
        className="whitespace-normal break-words"
      >
        {children}
      </Text>
    ) : (
      children
    );

  const iconElement = renderIcon();

  const dismissButton = dismissible ? (
    <IconButton
      color="secondary"
      variant="plain"
      icon={XIcon}
      label="Close"
      className="shrink-0 self-center text-secondary hover:bg-transparent"
      onClick={() => {
        setShow(false);
        onDismiss?.();
      }}
    />
  ) : null;

  if (!show) return null;
  return (
    <div
      className={cn(
        'm-0 flex items-center gap-space-2 border border-transparent px-space-4 py-space-3 text-sm',
        shadow && 'shadow-lg',
        intent === 'warning' && 'bg-warning',
        intent === 'info' && 'bg-brand-subtle-gradient',
        intent === 'success' && 'bg-success',
        intent === 'error' && 'bg-error',
        intent === 'neutral' && 'bg-elevated',
        intent === 'neutral' && !flush && 'border-default',
        !flush && 'rounded-md',
        className
      )}
    >
      {iconElement != null && (
        <div className="mr-space-1 flex shrink-0 items-center">
          {iconElement}
        </div>
      )}
      <div className="min-w-0 flex-1">
        {titleElement}
        {contentElement}
      </div>
      {action ? (
        <div className="flex shrink-0 items-center">{action}</div>
      ) : null}
      {dismissButton}
    </div>
  );
};
