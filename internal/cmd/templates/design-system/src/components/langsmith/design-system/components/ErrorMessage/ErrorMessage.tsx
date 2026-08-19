import { useId, useLayoutEffect, useRef, useState } from 'react';

import { AlertSolidIcon, ChevronRightIcon } from '@langchain/untitled-ui-icons';
import { useControllableState } from '@radix-ui/react-use-controllable-state';

import { cn } from '../../utils/cn';
import { prettifyErrorPayload } from '../../utils/prettify-error-payload';
import { CopyIconButton } from '../CopyButton';
import { Icon } from '../Icon';
import type { IconComponent } from '../Icon';
import { IconButton } from '../IconButton';
import { Text } from '../Text';

export interface ErrorMessageProps {
  message: React.ReactNode;
  title?: React.ReactNode;
  details?: React.ReactNode;
  defaultExpanded?: boolean;
  expanded?: boolean;
  onExpandedChange?: (expanded: boolean) => void;
  copyable?: boolean;
  icon?: IconComponent;
  size?: 'sm' | 'md';
  className?: string;
  prettify?: boolean;
}

const sizeStyles = {
  sm: {
    container: 'gap-space-1 pl-1.5 pr-space-1 py-0.5',

    textVariant: 'xs',
  },
  md: {
    container: 'gap-space-2 pl-2.5 pr-space-2 py-space-1',
    textVariant: 'sm',
  },
} as const;

function nodeToText(node: React.ReactNode): string {
  if (node == null || typeof node === 'boolean') return '';
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(nodeToText).join('');
  if (
    typeof node === 'object' &&
    'props' in node &&
    node.props &&
    typeof node.props === 'object' &&
    'children' in node.props
  ) {
    return nodeToText((node.props as { children: React.ReactNode }).children);
  }
  return '';
}

function ErrorMessage({
  message,
  title,
  details,
  defaultExpanded = false,
  expanded: controlledExpanded,
  onExpandedChange,
  copyable = true,
  icon = AlertSolidIcon,
  size = 'sm',
  className,
  prettify = true,
}: ErrorMessageProps) {
  const styles = sizeStyles[size];
  const displayMessage =
    prettify && typeof message === 'string'
      ? prettifyErrorPayload(message)
      : message;
  const [expanded = false, setExpanded] = useControllableState({
    prop: controlledExpanded,
    defaultProp: defaultExpanded,
    onChange: onExpandedChange,
  });

  const messageRef = useRef<HTMLElement>(null);
  const [messageOverflows, setMessageOverflows] = useState(false);
  useLayoutEffect(() => {
    const el = messageRef.current;
    if (el == null) return;

    function recompute() {
      const node = messageRef.current;
      if (node == null) return;
      setMessageOverflows(node.scrollWidth > node.clientWidth);
    }

    recompute();
    const observer = new ResizeObserver(recompute);
    observer.observe(el);
    return () => observer.disconnect();
  }, [displayMessage, size]);
  const toggleExpanded = () => setExpanded(!expanded);

  const copyText = [nodeToText(displayMessage), nodeToText(details)]
    .filter(Boolean)
    .join('\n\n');

  const hasMessage =
    displayMessage != null && displayMessage !== false && displayMessage !== '';
  const hasTitle = title != null && title !== false && title !== '';
  const titleNode = hasTitle ? title : 'Error';
  const hasDetails = details != null && details !== false && details !== '';
  const messageIsMultiline =
    typeof displayMessage === 'string' && displayMessage.includes('\n');
  const canExpand =
    hasDetails || messageOverflows || messageIsMultiline || expanded;
  const showCopy = copyable && copyText.length > 0;

  const expandableAreaId = useId();
  const expandLabel = expanded
    ? 'Collapse'
    : hasDetails
      ? 'Show error details'
      : 'Show full error message';

  if (!hasMessage) return null;

  return (
    <div
      role="alert"
      className={cn('w-full rounded-sm bg-error', styles.container, className)}
    >
      <div className="flex w-full items-center gap-space-2">
        <span aria-hidden="true" className="shrink-0">
          <Icon icon={icon} color="error" size="xs" rounded />
        </span>
        <div className={cn('flex min-w-0 flex-1 flex-row gap-space-1')}>
          <Text
            variant={styles.textVariant}
            weight="medium"
            className={cn(
              'flex shrink-0 items-center whitespace-nowrap leading-4 text-error-secondary dark:text-error-tertiary'
            )}
          >
            {titleNode}
          </Text>
          <Text
            ref={messageRef}
            aria-hidden={expanded}
            variant={styles.textVariant}
            className={cn(
              'min-w-0 truncate whitespace-nowrap leading-4 text-error-secondary dark:text-error-primary',
              expanded && 'pointer-events-none h-0'
            )}
          >
            {displayMessage}
          </Text>
        </div>
        {(showCopy || canExpand) && (
          <div className={cn('flex shrink-0 items-center')}>
            {showCopy && (
              <CopyIconButton
                copy={copyText}
                copyText="Copy error"
                size="xs"
                variant="plain"
                color="secondary"
              />
            )}
            {canExpand && (
              <IconButton
                icon={ChevronRightIcon}
                label={expandLabel}
                aria-expanded={expanded}
                aria-controls={expandableAreaId}
                variant="plain"
                color="secondary"
                size="xs"
                onClick={toggleExpanded}
                iconClassName={cn(
                  'transition-transform',
                  expanded && 'rotate-90'
                )}
              />
            )}
          </div>
        )}
      </div>
      {expanded && (
        <div
          id={expandableAreaId}
          className={cn(
            'flex flex-col gap-space-1 pb-space-1 pl-space-5 pr-space-1'
          )}
        >
          <Text
            variant={styles.textVariant}
            className={cn(
              'min-w-0 whitespace-pre-wrap break-words leading-4 text-error-secondary dark:text-error-primary'
            )}
          >
            {displayMessage}
          </Text>
          {hasDetails && (
            <Text
              variant={styles.textVariant}
              className="whitespace-pre-wrap break-words font-light text-error-tertiary"
            >
              {details}
            </Text>
          )}
        </div>
      )}
    </div>
  );
}

export { ErrorMessage };
