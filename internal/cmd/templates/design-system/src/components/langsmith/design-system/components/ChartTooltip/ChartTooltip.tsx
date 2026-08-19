import { forwardRef, type HTMLAttributes, type ReactNode } from 'react';

import { cn } from '../../utils/cn';
import { Text } from '../Text';

export type ChartTooltipProps = HTMLAttributes<HTMLDivElement>;

export const ChartTooltip = forwardRef<HTMLDivElement, ChartTooltipProps>(
  ({ className, children, ...props }, ref) => (
    <div
      ref={ref}
      role="tooltip"
      className={cn(
        'flex w-max max-w-[24rem] flex-col rounded-md border border-subtle bg-elevated p-space-1 text-primary shadow-md',
        className
      )}
      {...props}
    >
      {children}
    </div>
  )
);

ChartTooltip.displayName = 'ChartTooltip';

export interface ChartTooltipHeaderProps extends Omit<
  HTMLAttributes<HTMLDivElement>,
  'title'
> {
  title: ReactNode;
  value?: ReactNode;
  leading?: ReactNode;
}

export function ChartTooltipHeader({
  title,
  value,
  leading,
  className,
  ...props
}: ChartTooltipHeaderProps) {
  return (
    <div className={className} {...props}>
      <div className="flex min-w-0 items-center gap-space-6 px-space-2 py-space-1">
        <div className="flex min-w-0 flex-1 items-center gap-space-2">
          {leading}
          <Text
            variant="xs"
            weight="medium"
            className="min-w-0 flex-1 truncate"
          >
            {title}
          </Text>
        </div>
        {value != null && (
          <Text
            variant="xs"
            weight="semibold"
            className="shrink-0 tabular-nums"
          >
            {value}
          </Text>
        )}
      </div>
    </div>
  );
}

export type ChartTooltipBodyProps = HTMLAttributes<HTMLDivElement>;

export function ChartTooltipBody({
  className,
  children,
  ...props
}: ChartTooltipBodyProps) {
  return (
    <div
      className={cn(
        'scroll-mask-t scroll-mask-b flex max-h-[min(300px,70vh)] flex-col overflow-y-auto overflow-x-hidden',
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export interface ChartTooltipRowProps extends Omit<
  HTMLAttributes<HTMLDivElement>,
  'children'
> {
  label: ReactNode;
  value?: ReactNode;
  secondaryValue?: ReactNode;
  markerColor?: string;
  marker?: ReactNode;
  highlighted?: boolean;
  variant?: 'default' | 'total';
  action?: ReactNode;
}

export function ChartTooltipRow({
  label,
  value,
  secondaryValue,
  markerColor,
  marker,
  highlighted = false,
  variant = 'default',
  action,
  className,
  ...props
}: ChartTooltipRowProps) {
  const isTotal = variant === 'total';

  return (
    <>
      {isTotal && (
        <div
          aria-hidden
          className="mx-space-2 my-space-1 border-t border-subtle"
        />
      )}
      <div
        className={cn(
          'flex min-w-0 items-center gap-space-2 rounded-xs px-space-2 py-space-1',
          highlighted && 'bg-surface-level-2',
          className
        )}
        {...props}
      >
        {marker ??
          (markerColor != null && (
            <div
              aria-hidden
              className="size-2 shrink-0 rounded-full"
              style={{ backgroundColor: markerColor }}
            />
          ))}
        <Text
          variant="xs"
          weight={isTotal ? 'semibold' : 'normal'}
          className="min-w-0 flex-1 whitespace-normal break-words"
        >
          {label}
        </Text>
        {value != null && (
          <Text
            variant="xs"
            weight={isTotal ? 'semibold' : 'normal'}
            className="shrink-0 text-right tabular-nums"
          >
            {value}
          </Text>
        )}
        {secondaryValue != null && (
          <Text
            variant="xs"
            color="tertiary"
            className="shrink-0 text-right tabular-nums"
          >
            {secondaryValue}
          </Text>
        )}
        {action}
      </div>
    </>
  );
}
