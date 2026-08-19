import type { ReactNode } from 'react';
import { Fragment, useCallback, useEffect, useRef, useState } from 'react';

import { CheckIcon, SearchMdIcon } from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';
import { Icon } from '../Icon';
import type { InputSize, InputVariant } from '../Input/inputStyles';
import { Text } from '../Text';
import { Tooltip } from '../Tooltip';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from './Command';

export interface CommandMenuItem {
  /** Unique value used for selection and filtering */
  value: string;
  /** Display label (defaults to value if not provided) */
  label?: string;
  /** Secondary description text displayed below the label */
  description?: string;
  /** Element displayed on the right side of the item (e.g. badge, shortcut) */
  rightDecorator?: ReactNode;
  /** Whether this item is disabled */
  disabled?: boolean;
  /** Extra search terms for filtering */
  keywords?: string[];
}

export interface CommandMenuGroup {
  /** Optional heading displayed above the group */
  heading?: string;
  /** Items in this group */
  items: CommandMenuItem[];
}

interface CommandMenuBaseProps {
  /** Callback when an item is selected */
  onSelect: (value: string) => void;
  /** The currently selected value. When set, a check icon is shown next to the selected item. */
  value?: string;
  /** Placeholder text for the search input */
  placeholder?: string;
  /** Text shown when no items match the search */
  emptyText?: string;
  /** Hide the search input (default: false, search is visible) */
  hideSearch?: boolean;
  /** Show the leading selected-indicator slot in the default item layout (default: true). Set false for action menus that track no selection so labels sit flush-left. */
  showSelectedIndicator?: boolean;
  /** Disable client-side filtering by cmdk (default: false). Set true for server-side filtering. */
  disableFilter?: boolean;
  /** Callback when the search input value changes. Useful for server-side filtering. */
  onSearchChange?: (query: string) => void;
  /** Custom render function for each item */
  renderItem?: (item: CommandMenuItem, isSelected: boolean) => ReactNode;
  /** Size variant for the search input */
  inputSize?: InputSize;
  /** Visual variant for the search input */
  inputVariant?: InputVariant;
  /** Additional class name for the Command root */
  className?: string;
  /** Whether more items are currently being loaded */
  loading?: boolean;
  /** Number of skeleton rows to show while loading (default: 5) */
  loadingRows?: number;
  /** Callback when the user scrolls near the end of the list. Use for infinite loading. */
  onEndReached?: () => void;
  /** Distance in pixels from the bottom to trigger onEndReached (default: 50) */
  endReachedThresholdPx?: number;
}

export type CommandMenuProps = CommandMenuBaseProps &
  (
    | { /** Flat list of items */ items: CommandMenuItem[]; groups?: never }
    | {
        /** Grouped items with optional headings */ groups: CommandMenuGroup[];
        items?: never;
      }
  );

function TruncatedLabel({ text }: { text: string }) {
  const textRef = useRef<HTMLElement>(null);
  const [isTruncated, setIsTruncated] = useState(false);

  useEffect(() => {
    const el = textRef.current;
    if (el) {
      setIsTruncated(el.scrollWidth > el.clientWidth);
    }
  }, [text]);

  return (
    <Tooltip title={isTruncated ? text : undefined}>
      <Text ref={textRef} as="span" className="truncate">
        {text}
      </Text>
    </Tooltip>
  );
}

export function CommandMenu({
  items,
  groups,
  onSelect,
  value,
  placeholder = 'Search...',
  emptyText = 'No results found.',
  hideSearch,
  showSelectedIndicator = true,
  disableFilter,
  onSearchChange,
  renderItem,
  inputSize = 'sm',
  inputVariant = 'plain',
  className,
  loading = false,
  loadingRows = 5,
  onEndReached,
  endReachedThresholdPx = 50,
}: CommandMenuProps) {
  const handleScroll = useCallback(
    (e: React.UIEvent<HTMLDivElement>) => {
      if (!onEndReached) return;
      const { scrollTop, scrollHeight, clientHeight } = e.currentTarget;
      if (scrollHeight - scrollTop - clientHeight <= endReachedThresholdPx) {
        onEndReached();
      }
    },
    [onEndReached, endReachedThresholdPx]
  );

  const renderItems = (menuItems: CommandMenuItem[]) =>
    menuItems.map((item) => {
      const isSelected = item.value === value;
      const label = item.label ?? item.value;
      return (
        <CommandItem
          key={item.value}
          value={item.value}
          keywords={
            item.label ? [item.label, ...(item.keywords ?? [])] : item.keywords
          }
          disabled={item.disabled}
          onSelect={() => onSelect(item.value)}
          className="gap-space-2"
        >
          {renderItem ? (
            renderItem(item, isSelected)
          ) : (
            <>
              {showSelectedIndicator && (
                <Icon
                  icon={CheckIcon}
                  size="md"
                  className={cn(
                    'shrink-0',
                    isSelected ? 'opacity-100' : 'opacity-0'
                  )}
                />
              )}
              <div className="flex min-w-0 flex-1 flex-col">
                <TruncatedLabel text={label} />
                {item.description && (
                  <Text
                    as="span"
                    variant="xs"
                    className="truncate text-tertiary"
                  >
                    {item.description}
                  </Text>
                )}
              </div>
              {item.rightDecorator && (
                <div className="ml-auto shrink-0">{item.rightDecorator}</div>
              )}
            </>
          )}
        </CommandItem>
      );
    });

  return (
    <Command
      className={cn('flex flex-col gap-1.5 p-1.5', className)}
      shouldFilter={disableFilter ? false : undefined}
    >
      {!hideSearch && (
        <CommandInput
          placeholder={placeholder}
          size={inputSize}
          variant={inputVariant}
          leftDecorator={<SearchMdIcon className="size-4" />}
          onValueChange={onSearchChange}
        />
      )}
      <CommandList onScroll={handleScroll}>
        {!loading && (
          <CommandEmpty>
            <Text variant="sm">{emptyText}</Text>
          </CommandEmpty>
        )}
        {items && renderItems(items)}
        {groups?.map((group, i) => (
          <Fragment key={group.heading ?? i}>
            {i > 0 && <CommandSeparator />}
            <CommandGroup heading={group.heading}>
              {renderItems(group.items)}
            </CommandGroup>
          </Fragment>
        ))}
        {loading && (
          <div className="flex flex-col gap-space-1 px-space-2 py-space-1">
            {Array.from({ length: loadingRows }, (_, i) => (
              <div
                key={i}
                className="h-7 animate-pulse rounded-md bg-tertiary"
              />
            ))}
          </div>
        )}
      </CommandList>
    </Command>
  );
}
