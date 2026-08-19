import type { ReactNode } from 'react';
import { useEffect, useRef, useState } from 'react';

import { cn } from '../../utils/cn';
import { type IconComponent } from '../Icon/Icon';
import { Text } from '../Text';
import type { TooltipProps } from '../Tooltip';
import { Tooltip } from '../Tooltip';

export interface GroupedTabOption<T> {
  display?: ReactNode | ((isActive: boolean) => ReactNode);
  value: T;
  disabled?: boolean;
  tooltip?: string | ReactNode;
  tooltipProps?: Omit<TooltipProps, 'children' | 'title'>;
  icon?: IconComponent;
  leftDecorator?: ReactNode;
  rightDecorator?: ReactNode;
  className?: string;
  size?: 'xs' | 'sm' | 'md';
  textClassName?: string;
  'aria-label'?: string;
}

export type GroupedTabsProps<T extends string | number> = {
  value: T;
  onChange: (value: T) => void;
  options: readonly GroupedTabOption<T>[];
  className?: string;
  size?: 'xs' | 'sm' | 'md';
  disabled?: boolean;
  tooltip?: string | ReactNode;
  selectionClassName?: string;
  /** Optional class for the indicator (e.g. to override top/height for custom margins) */
  indicatorClassName?: string;
};

export const GroupedTabs = <T extends string | number>({
  value,
  onChange,
  options,
  className,
  size = 'sm',
  disabled,
  tooltip,
  selectionClassName,
  indicatorClassName,
}: GroupedTabsProps<T>) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const buttonRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const [clipState, setClipState] = useState<{
    path: string;
    isFirst: boolean;
    animate: boolean;
  }>({ path: '', isFirst: true, animate: false });

  useEffect(() => {
    const container = containerRef.current;
    const activeIndex = options.findIndex((option) => option.value === value);
    const activeButton = buttonRefs.current[activeIndex];

    if (activeButton && container) {
      const containerRect = container.getBoundingClientRect();
      const buttonRect = activeButton.getBoundingClientRect();
      const roundValue = size === 'xs' ? '1px' : size === 'sm' ? '2px' : '4px';

      const top = buttonRect.top - containerRect.top - container.clientTop;
      const bottom =
        containerRect.bottom - buttonRect.bottom - container.clientTop;
      const left = buttonRect.left - containerRect.left - container.clientLeft;
      const right =
        containerRect.right - buttonRect.right - container.clientLeft;

      setClipState((prev) => ({
        path: `inset(${top}px ${right}px ${bottom}px ${left}px round ${roundValue})`,
        isFirst: false,
        animate: !prev.isFirst,
      }));
    }
  }, [size, value, options]);

  const isActive = (option: GroupedTabOption<T>) => option.value === value;
  return (
    <Tooltip title={tooltip}>
      <div
        ref={containerRef}
        className={cn(
          'relative flex w-fit border border-subtle p-0.5 dark:border-muted',
          size === 'xs' ? 'gap-px' : 'gap-0.5',
          size === 'xs'
            ? 'rounded-[2px]'
            : size === 'sm'
              ? 'rounded-[3px]'
              : 'rounded-sm',
          className
        )}
      >
        {/* Clip-path animated background indicator */}
        {clipState.path && (
          <div
            aria-hidden
            className={cn(
              'pointer-events-none absolute inset-0 bg-surface-level-4',
              selectionClassName,
              indicatorClassName
            )}
            style={{
              clipPath: clipState.path,
              transition: clipState.animate
                ? 'clip-path 0.25s ease-out, background-color 0.25s ease-out'
                : 'none',
            }}
          />
        )}

        {options.map((option, idx) => (
          <Tooltip
            key={option.value}
            title={option.tooltip}
            {...option.tooltipProps}
          >
            {/* eslint-disable-next-line react/forbid-elements */}
            <button
              ref={(el) => {
                buttonRefs.current[idx] = el;
              }}
              type="button"
              aria-label={option['aria-label']}
              onClick={() => onChange(option.value)}
              disabled={option.disabled || disabled}
              className={cn(
                'relative flex items-center gap-space-1',
                'transition-colors duration-fast',
                {
                  'rounded-xs px-space-1 py-px': size === 'xs',
                  'rounded-xs px-space-2 py-0.5': size === 'sm',
                  'rounded-sm px-space-2 py-space-1': size === 'md',
                },
                {
                  'text-primary': isActive(option),
                  'text-secondary hover:bg-surface-level-1-hover':
                    !isActive(option),
                },
                (option.disabled || disabled) &&
                  'text-disabled hover:bg-transparent',
                option.className
              )}
            >
              {option.icon && (
                <option.icon className={size === 'xs' ? 'size-3' : 'size-4'} />
              )}
              {option.leftDecorator}
              {option.display &&
                (typeof option.display === 'function' ? (
                  option.display(isActive(option))
                ) : (
                  <Text
                    variant={size === 'xs' ? 'xs' : 'sm'}
                    className={cn('font-medium', option.textClassName)}
                  >
                    {option.display}
                  </Text>
                ))}
              {option.rightDecorator}
            </button>
          </Tooltip>
        ))}
      </div>
    </Tooltip>
  );
};
