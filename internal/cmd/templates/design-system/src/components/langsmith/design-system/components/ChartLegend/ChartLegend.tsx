import {
  type HTMLAttributes,
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import {
  FilterFunnel01Icon,
  FilterFunnel01SolidIcon,
} from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';
import { SPACE_SCALE_PX } from '../../utils/spacing';
import { Badge } from '../Badge';
import { Button } from '../Button';
import { Icon } from '../Icon';
import { Popover, PopoverAnchor, PopoverContent } from '../Popover';
import { Text } from '../Text';
import { Tooltip } from '../Tooltip';
import { getAccessibleText, widthsMatch } from './utils';

const LEGEND_GAP_PX = SPACE_SCALE_PX[1];
const VIEW_HIDDEN_TOOLTIP = 'Click to view hidden legends';

export type ChartLegendItem = {
  id: string;
  label: ReactNode;
  value?: ReactNode;
  secondaryValue?: ReactNode;
  'aria-label'?: string;
  markerColor?: string;
  marker?: ReactNode;
  selected?: boolean;
};

export type ChartLegendProps = Omit<
  HTMLAttributes<HTMLDivElement>,
  'children'
> & {
  items: readonly ChartLegendItem[];
  layout?: 'inline' | 'list';
  onItemClick?: (item: ChartLegendItem) => void;
};

type ChartLegendLabelProps = {
  item: ChartLegendItem;
  onClick?: () => void;
  className?: string;
  tabIndex?: number;
  layout?: ChartLegendProps['layout'];
};

const ChartLegendLabel = ({
  item,
  onClick,
  className,
  tabIndex,
  layout = 'inline',
}: ChartLegendLabelProps) => {
  const isList = layout === 'list';
  const labelText = getAccessibleText(item.label);
  const valueText = isList ? getAccessibleText(item.value) : undefined;
  const secondaryValueText = isList
    ? getAccessibleText(item.secondaryValue)
    : undefined;
  const defaultAriaLabel =
    labelText == null
      ? undefined
      : [labelText, valueText, secondaryValueText]
          .filter((value) => value != null)
          .join(', ');
  const content = (
    <>
      {item.marker ??
        (item.markerColor != null && (
          <div
            aria-hidden
            className="size-2 shrink-0 rounded-full"
            style={{ backgroundColor: item.markerColor }}
          />
        ))}
      <Text
        as="span"
        variant="xs"
        className={cn(
          'min-w-0 flex-1 text-left',
          isList ? 'whitespace-normal break-words' : 'truncate'
        )}
      >
        {item.label}
      </Text>
      {isList && item.value != null && (
        <Text
          as="span"
          variant="xs"
          className="shrink-0 text-right tabular-nums"
        >
          {item.value}
        </Text>
      )}
      {isList && item.secondaryValue != null && (
        <Text
          as="span"
          variant="xs"
          color="tertiary"
          className="shrink-0 text-right tabular-nums"
        >
          {item.secondaryValue}
        </Text>
      )}
      {(onClick != null || item.selected) && (
        <Icon
          aria-hidden="true"
          icon={item.selected ? FilterFunnel01SolidIcon : FilterFunnel01Icon}
          size="xs"
          className={cn(
            'size-3 shrink-0 text-icon-tertiary transition-opacity duration-fast',
            item.selected
              ? 'opacity-100'
              : 'opacity-0 group-hover:opacity-100 group-focus-visible:opacity-100'
          )}
        />
      )}
    </>
  );

  const styles = cn(
    'group relative flex min-w-0 items-center rounded-xs py-space-1 text-primary transition-colors duration-fast',
    isList
      ? 'w-full gap-space-2 px-space-2'
      : 'max-w-full shrink-0 gap-space-1 px-space-1',
    item.selected
      ? 'bg-surface-level-2 hover:bg-surface-level-2'
      : onClick != null && 'hover:bg-surface-level-1-hover',
    className
  );

  if (onClick == null) {
    return <div className={styles}>{content}</div>;
  }

  return (
    <Button
      type="button"
      size="xs"
      color="secondary"
      variant="plain"
      className={cn(
        styles,
        'focus-visible:ring-focus border-transparent shadow-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset',
        isList && 'justify-start',
        !item.selected && 'bg-transparent'
      )}
      aria-label={item['aria-label'] ?? defaultAriaLabel}
      aria-pressed={item.selected ?? false}
      tabIndex={tabIndex}
      onClick={onClick}
    >
      {content}
    </Button>
  );
};

type ChartLegendBaseProps = Omit<ChartLegendProps, 'layout'>;

const ChartLegendList = ({
  items,
  onItemClick,
  className,
  role = 'group',
  'aria-label': ariaLabel = 'Chart legend',
  ...props
}: ChartLegendBaseProps) => (
  <div
    role={role}
    aria-label={ariaLabel}
    className={cn('flex w-full min-w-0 flex-col', className)}
    {...props}
  >
    {items.map((item) => (
      <ChartLegendLabel
        key={item.id}
        item={item}
        layout="list"
        onClick={
          onItemClick == null
            ? undefined
            : () => {
                onItemClick(item);
              }
        }
      />
    ))}
  </div>
);

const ChartLegendInline = ({
  items,
  onItemClick,
  className,
  role = 'group',
  'aria-label': ariaLabel = 'Chart legend',
  ...props
}: ChartLegendBaseProps) => {
  const [popoverOpen, setPopoverOpen] = useState(false);
  const [containerWidth, setContainerWidth] = useState<number | null>(null);
  const [itemWidths, setItemWidths] = useState<Record<string, number>>({});
  const [badgeWidths, setBadgeWidths] = useState<Record<string, number>>({});
  const containerRef = useRef<HTMLDivElement>(null);
  const measurementItemRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const measurementBadgeRefs = useRef<Record<string, HTMLDivElement | null>>(
    {}
  );
  const isInteractive = onItemClick != null;

  const itemIds = useMemo(() => items.map((item) => item.id), [items]);

  const setMeasurementItemRef = useCallback(
    (id: string, node: HTMLDivElement | null) => {
      measurementItemRefs.current[id] = node;
    },
    []
  );

  const setMeasurementBadgeRef = useCallback(
    (count: number, node: HTMLDivElement | null) => {
      measurementBadgeRefs.current[String(count)] = node;
    },
    []
  );

  useLayoutEffect(() => {
    const nextItemWidths: Record<string, number> = {};
    const nextBadgeWidths: Record<string, number> = {};

    items.forEach((item, index) => {
      const itemWidth = measurementItemRefs.current[item.id]?.offsetWidth;
      if (itemWidth != null) {
        nextItemWidths[item.id] = Math.ceil(itemWidth);
      }

      const hiddenCount = index + 1;
      const badgeWidth =
        measurementBadgeRefs.current[String(hiddenCount)]?.offsetWidth;
      if (badgeWidth != null) {
        nextBadgeWidths[String(hiddenCount)] = Math.ceil(badgeWidth);
      }
    });

    setItemWidths((current) =>
      widthsMatch(current, nextItemWidths) ? current : nextItemWidths
    );
    setBadgeWidths((current) =>
      widthsMatch(current, nextBadgeWidths) ? current : nextBadgeWidths
    );
  }, [isInteractive, items]);

  useLayoutEffect(() => {
    const container = containerRef.current;

    if (container == null) {
      return;
    }

    const updateWidth = (width: number) => {
      const nextWidth = Math.ceil(width);
      setContainerWidth((current) =>
        current === nextWidth ? current : nextWidth
      );
    };

    updateWidth(container.clientWidth);

    const observer = new ResizeObserver(([entry]) => {
      updateWidth(entry?.contentRect.width ?? container.clientWidth);
    });

    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  const visibleCount = useMemo(() => {
    const hasMeasurements =
      containerWidth != null &&
      itemIds.every((id) => itemWidths[id] != null) &&
      items.every((_, index) => badgeWidths[String(index + 1)] != null);

    if (!hasMeasurements || containerWidth == null) {
      return items.length;
    }

    const allItemsWidth = itemIds.reduce(
      (total, id) => total + (itemWidths[id] ?? 0),
      Math.max(items.length - 1, 0) * LEGEND_GAP_PX
    );

    if (allItemsWidth <= containerWidth) {
      return items.length;
    }

    for (let count = items.length - 1; count >= 0; count -= 1) {
      const hiddenCount = items.length - count;
      const visibleItemsWidth = itemIds
        .slice(0, count)
        .reduce((total, id) => total + (itemWidths[id] ?? 0), 0);
      const gapsWidth = count * LEGEND_GAP_PX;
      const totalWidth =
        visibleItemsWidth + gapsWidth + (badgeWidths[String(hiddenCount)] ?? 0);

      if (totalWidth <= containerWidth) {
        return count;
      }
    }

    return 0;
  }, [badgeWidths, containerWidth, itemIds, itemWidths, items]);

  const hiddenCount = Math.max(items.length - visibleCount, 0);
  const hasOverflow = hiddenCount > 0;
  const visibleItems = items.slice(0, visibleCount);
  const hiddenItems = items.slice(visibleCount);

  useEffect(() => {
    if (!hasOverflow) {
      setPopoverOpen(false);
    }
  }, [hasOverflow]);

  const renderItem = (item: ChartLegendItem) => (
    <ChartLegendLabel
      key={item.id}
      item={item}
      onClick={
        onItemClick == null
          ? undefined
          : () => {
              onItemClick(item);
            }
      }
    />
  );

  return (
    <Popover open={popoverOpen} onOpenChange={setPopoverOpen}>
      <PopoverAnchor asChild>
        <div
          ref={containerRef}
          role={role}
          aria-label={ariaLabel}
          className={cn(
            'relative flex w-full min-w-0 items-center gap-space-1 overflow-hidden rounded-xs',
            className
          )}
          {...props}
        >
          {hasOverflow && (
            <Tooltip title={VIEW_HIDDEN_TOOLTIP}>
              <Button
                type="button"
                size="xs"
                color="secondary"
                variant="plain"
                className={cn(
                  'focus-visible:ring-focus absolute inset-0 size-full rounded-xs border-transparent p-0 shadow-none hover:bg-surface-level-1-hover focus-visible:outline-none focus-visible:ring-2',
                  popoverOpen && 'bg-surface-level-2 hover:bg-surface-level-2'
                )}
                aria-label="View hidden chart legends"
                aria-expanded={popoverOpen}
                aria-haspopup="dialog"
                onClick={() => setPopoverOpen((open) => !open)}
              >
                <Text as="span" variant="xs" className="sr-only">
                  View hidden chart legends
                </Text>
              </Button>
            </Tooltip>
          )}

          {visibleItems.map(renderItem)}

          {hasOverflow && (
            <Tooltip title={VIEW_HIDDEN_TOOLTIP}>
              <Button
                type="button"
                size="xs"
                color="secondary"
                variant="plain"
                className="focus-visible:ring-focus relative rounded-xs border-transparent bg-transparent p-0 shadow-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-1"
                aria-label={`View ${hiddenCount} hidden chart ${
                  hiddenCount === 1 ? 'legend' : 'legends'
                }`}
                aria-expanded={popoverOpen}
                aria-haspopup="dialog"
                onClick={() => setPopoverOpen((open) => !open)}
              >
                <Badge size="xxs" rounded="xs">
                  {`+${hiddenCount}`}
                </Badge>
              </Button>
            </Tooltip>
          )}

          <div
            aria-hidden="true"
            className="pointer-events-none invisible absolute left-0 top-0 flex w-max items-center gap-space-1"
          >
            {items.map((item) => (
              <div
                key={item.id}
                ref={(node) => setMeasurementItemRef(item.id, node)}
                className="shrink-0"
              >
                <ChartLegendLabel
                  item={item}
                  onClick={onItemClick == null ? undefined : () => undefined}
                  tabIndex={-1}
                />
              </div>
            ))}
            {items.map((_, index) => {
              const count = index + 1;
              return (
                <div
                  key={count}
                  ref={(node) => setMeasurementBadgeRef(count, node)}
                  className="shrink-0"
                >
                  <Badge size="xxs" rounded="xs">
                    {`+${count}`}
                  </Badge>
                </div>
              );
            })}
          </div>
        </div>
      </PopoverAnchor>

      {hasOverflow && (
        <PopoverContent
          align="end"
          className="w-max min-w-48 max-w-[min(24rem,calc(100vw-2rem))] overflow-hidden border border-subtle p-0"
          onOpenAutoFocus={(event) => event.preventDefault()}
          onCloseAutoFocus={(event) => event.preventDefault()}
          onInteractOutside={(event) => {
            if (
              event.target instanceof Node &&
              containerRef.current?.contains(event.target)
            ) {
              event.preventDefault();
            }
          }}
        >
          <div className="scroll-mask-t scroll-mask-b flex max-h-[min(300px,70vh)] flex-col gap-space-1 overflow-y-auto overflow-x-hidden p-space-1">
            {hiddenItems.map(renderItem)}
          </div>
        </PopoverContent>
      )}
    </Popover>
  );
};

export const ChartLegend = ({
  layout = 'inline',
  ...props
}: ChartLegendProps) =>
  layout === 'list' ? (
    <ChartLegendList {...props} />
  ) : (
    <ChartLegendInline {...props} />
  );
