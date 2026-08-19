import {
  forwardRef,
  useId,
  type ButtonHTMLAttributes,
  type HTMLAttributes,
  type ReactNode,
  type SVGProps,
} from 'react';

import {
  AlertTriangleSolidIcon,
  DotsVerticalIcon,
  DragIcon as UntitledDragIcon,
  Expand01Icon,
  Minimize01Icon,
  XCloseIcon,
} from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '../DropdownMenu';
import { EmptyState } from '../EmptyState';
import { IconButton } from '../IconButton';
import { Skeleton } from '../Skeleton';
import { Text } from '../Text';

const ChartCardDragIcon = (props: SVGProps<SVGSVGElement>) => (
  // The package icon's path uses a 16px grid despite its default 24px viewBox.
  <UntitledDragIcon {...props} viewBox="0 0 16 16" />
);

export interface ChartCardActionButtonProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  'children' | 'color'
> {
  /** Accessible label and tooltip text. */
  label?: string;
}

export type ChartCardVariant = 'default' | 'full-width';

export type ChartCardState = 'ready' | 'loading' | 'error';

const ChartCardLoadingState = () => (
  <div
    role="status"
    className="flex min-h-[13.5rem] flex-1 flex-col gap-space-2"
  >
    <div className="relative min-h-0 flex-1">
      <Skeleton className="size-full rounded-md" />
      <Text
        variant="sm"
        color="placeholder"
        className="absolute inset-0 flex items-center justify-center"
      >
        Loading...
      </Text>
    </div>
    <div aria-hidden="true" className="flex min-w-0 items-center gap-space-2">
      <Skeleton className="h-4 w-20" />
      <Skeleton className="h-4 w-24" />
      <Skeleton className="h-4 w-40" />
      <Skeleton className="h-4 w-12" />
    </div>
  </div>
);

const ChartCardErrorState = () => (
  <div role="alert" className="flex min-h-0 flex-1 rounded-md">
    <EmptyState
      icon={AlertTriangleSolidIcon}
      title="There was an error while loading the chart."
      description="Refresh the page to try again."
      className="flex-1"
      size="sm"
    />
  </div>
);

export interface ChartCardProps extends Omit<
  HTMLAttributes<HTMLElement>,
  'title'
> {
  /** Visible chart title rendered in the card header. */
  title: ReactNode;
  /**
   * Optional secondary text rendered beneath the title. Truncated to a single
   * line to match the title's overflow behavior.
   */
  description?: ReactNode;
  /** Visual state of the card. Fetching and retry behavior remain external. */
  state?: ChartCardState;
  /** Layout state for the card. Full-width cards hide their move handle. */
  variant?: ChartCardVariant;
  /** Whether to show the chart's move handle. */
  isMovable?: boolean;
  /**
   * Props for the move handle. Pass the listeners and attributes from the
   * dashboard's drag-and-drop implementation; the card owns only the visual
   * affordance.
   */
  dragHandleProps?: ChartCardActionButtonProps;
  /** Props for the optional expand button. Expansion state remains external. */
  expandButtonProps?: ChartCardActionButtonProps;
  /** Props for the optional close button. Closing behavior remains external. */
  closeButtonProps?: ChartCardActionButtonProps;
  /** Feature-owned controls rendered in the top-right header action area. */
  headerActions?: ReactNode;
  /**
   * Feature-owned menu items rendered inside the card's overflow menu. The
   * card supplies the trigger, positioning, and menu surface.
   */
  menuItems?: ReactNode;
  /** Classes applied to the chart-and-legend content region. */
  contentClassName?: string;
}

/**
 * A lightweight, chart-library-agnostic container for dashboard
 * visualizations. ChartCard standardizes the shell and header actions while
 * leaving data fetching, chart rendering, drag behavior, and expansion
 * behavior to the consuming feature.
 */
export const ChartCard = forwardRef<HTMLElement, ChartCardProps>(
  (
    {
      title,
      description,
      children,
      state = 'ready',
      variant = 'default',
      isMovable = false,
      dragHandleProps,
      expandButtonProps,
      closeButtonProps,
      headerActions,
      menuItems,
      className,
      contentClassName,
      'aria-labelledby': ariaLabelledBy,
      'aria-busy': ariaBusy,
      ...props
    },
    ref
  ) => {
    const titleId = useId();
    const isFullWidth = variant === 'full-width';
    const isLoading = state === 'loading';

    const {
      label: dragHandleLabel = 'Move chart',
      className: dragHandleClassName,
      onClick: onDragHandleClick,
      ...dragButtonProps
    } = dragHandleProps ?? {};
    const {
      label: expandButtonLabel = isFullWidth
        ? 'Minimize chart'
        : 'Expand chart',
      className: expandButtonClassName,
      onClick: onExpandClick,
      ...expandActionProps
    } = expandButtonProps ?? {};
    const {
      label: closeButtonLabel = 'Close chart',
      className: closeButtonClassName,
      onClick: onCloseClick,
      ...closeActionProps
    } = closeButtonProps ?? {};

    return (
      <section
        ref={ref}
        aria-labelledby={ariaLabelledBy ?? titleId}
        aria-busy={ariaBusy ?? (isLoading || undefined)}
        className={cn(
          'flex min-w-0 flex-col overflow-hidden border border-muted bg-surface-level-1',
          isFullWidth && 'w-full',
          className
        )}
        {...props}
      >
        <div className="flex min-w-0 items-center justify-between gap-space-3 pl-space-3 pr-space-2 pt-space-2">
          <div className="flex min-w-0 flex-1 items-center">
            {isMovable && !isFullWidth && !isLoading && (
              <IconButton
                {...dragButtonProps}
                icon={ChartCardDragIcon}
                label={dragHandleLabel}
                size="xs"
                variant="plain"
                color="secondary"
                iconClassName="size-4 text-icon-primary"
                className={cn(
                  'cursor-grab touch-none active:cursor-grabbing',
                  dragHandleClassName
                )}
                onClick={(event) => {
                  event.stopPropagation();
                  onDragHandleClick?.(event);
                }}
              />
            )}
            <div className="flex min-w-0 flex-1 flex-col gap-space-1">
              <Text
                id={titleId}
                as="h3"
                variant="sm"
                weight="semibold"
                className="min-w-0 truncate"
              >
                {title}
              </Text>
              {description != null && description !== '' && (
                <Text
                  variant="xs"
                  color="quaternary"
                  className="min-w-0 truncate"
                >
                  {description}
                </Text>
              )}
            </div>
          </div>

          {!isLoading && (
            <div
              className={cn(
                'flex shrink-0 items-center',
                Boolean(headerActions) && 'gap-space-1'
              )}
            >
              {!!headerActions && (
                <div className="flex items-center pr-space-1">
                  {headerActions}
                </div>
              )}
              {expandButtonProps != null && (
                <IconButton
                  {...expandActionProps}
                  icon={isFullWidth ? Minimize01Icon : Expand01Icon}
                  label={expandButtonLabel}
                  size="xs"
                  variant="plain"
                  color="secondary"
                  iconClassName="size-4 text-icon-primary"
                  className={expandButtonClassName}
                  onClick={(event) => {
                    event.stopPropagation();
                    onExpandClick?.(event);
                  }}
                />
              )}
              {!!menuItems && (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <IconButton
                      icon={DotsVerticalIcon}
                      label="More chart actions"
                      size="xs"
                      variant="plain"
                      color="secondary"
                      iconClassName="size-4 text-icon-primary"
                      onClick={(event) => event.stopPropagation()}
                    />
                  </DropdownMenuTrigger>
                  <DropdownMenuContent
                    align="end"
                    preventClickThrough
                    onClick={(event) => event.stopPropagation()}
                  >
                    {menuItems}
                  </DropdownMenuContent>
                </DropdownMenu>
              )}
              {closeButtonProps != null && (
                <IconButton
                  {...closeActionProps}
                  icon={XCloseIcon}
                  label={closeButtonLabel}
                  size="xs"
                  variant="plain"
                  color="secondary"
                  iconClassName="size-4 text-icon-primary"
                  className={closeButtonClassName}
                  onClick={(event) => {
                    event.stopPropagation();
                    onCloseClick?.(event);
                  }}
                />
              )}
            </div>
          )}
        </div>

        <div
          className={cn(
            'flex min-h-0 flex-1 flex-col gap-space-2 px-space-3 py-space-2',
            contentClassName
          )}
        >
          {state === 'loading' ? (
            <ChartCardLoadingState />
          ) : state === 'error' ? (
            <ChartCardErrorState />
          ) : (
            children
          )}
        </div>
      </section>
    );
  }
);

ChartCard.displayName = 'ChartCard';
