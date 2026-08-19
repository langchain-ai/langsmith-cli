import type { ReactNode } from 'react';
import { useCallback, useMemo, useState } from 'react';

import { ChevronDownIcon } from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';
import { Button } from '../Button';
import { CommandMenu } from '../Command/CommandMenu';
import type { CommandMenuGroup, CommandMenuItem } from '../Command/CommandMenu';
import { Popover, PopoverContent, PopoverTrigger } from '../Popover';
import { Text } from '../Text';

export interface SelectOption<T extends string = string> {
  /** The value used for selection and filtering */
  value: T;
  /** Display label (defaults to value if not provided) */
  label?: string;
  /** Secondary description text displayed below the label */
  description?: string;
  /** Element displayed on the right side of the item (e.g. badge, shortcut) */
  rightDecorator?: ReactNode;
  /** Whether this option is disabled */
  disabled?: boolean;
}

export interface SelectGroup<T extends string = string> {
  heading?: string;
  options: SelectOption<T>[];
}

export type SelectSize = 'xs' | 'sm' | 'md' | 'lg';

const CONTROL_HEIGHT_CLASSES = {
  xs: 'h-[1.5625rem]',
  sm: 'h-[1.625rem]',
  md: 'h-[2.1875rem]',
  lg: 'h-[2.4375rem]',
} as const satisfies Record<SelectSize, string>;

interface SelectBaseProps<T extends string = string> {
  /** The currently selected value */
  value?: T;
  /** Callback when selection changes. Receives undefined when deselected. */
  onChange: (value: T | undefined) => void;
  /** Placeholder text when no value is selected */
  placeholder?: string;
  /** Placeholder text for the search input */
  searchPlaceholder?: string;
  /** Whether the select is disabled */
  disabled?: boolean;
  /** Size variant for the trigger. xs/sm/md match Typeahead control heights; lg matches the default Input height. */
  size?: SelectSize;
  /** Text to show when no options match the search */
  emptyText?: string;
  /** Allow deselecting the current value by clicking it again (default: false) */
  allowDeselect?: boolean;
  /** Hide the search input (default: true, search is hidden) */
  hideSearch?: boolean;
  /** Disable client-side filtering by cmdk (default: false). Set true for server-side filtering. */
  disableFilter?: boolean;
  /** Callback when the search input value changes. Useful for server-side filtering. */
  onSearchChange?: (query: string) => void;
  /** Whether more items are currently being loaded */
  loading?: boolean;
  /** Number of skeleton rows to show while loading (default: 5) */
  loadingRows?: number;
  /** Callback when the user scrolls near the end of the list. Use for infinite loading. */
  onEndReached?: () => void;
  /** Distance in pixels from the bottom to trigger onEndReached (default: 50) */
  endReachedThresholdPx?: number;
  /** Additional class name for the trigger button */
  triggerClassName?: string;
  /** Additional class name for the popover content */
  contentClassName?: string;
  /** Custom render function for the selected value in the trigger */
  renderValue?: (option: SelectOption<T>) => ReactNode;
  /** Custom render function for each option in the list */
  renderOption?: (option: SelectOption<T>, isSelected: boolean) => ReactNode;
  /** Side of the trigger to align the popover */
  side?: 'top' | 'bottom' | 'left' | 'right';
  /** Alignment of the popover relative to the trigger */
  align?: 'start' | 'center' | 'end';
  /** aria-label for the trigger button */
  'aria-label'?: string;
  /** Test id for the trigger button */
  'data-testid'?: string;
}

export type SelectProps<T extends string = string> = SelectBaseProps<T> &
  (
    | {
        options: SelectOption<T>[];
        groups?: never;
      }
    | {
        groups: SelectGroup<T>[];
        options?: never;
      }
  ) &
  (
    | {
        /** Controlled open state */
        open: boolean;
        /** Callback when open state changes (required when `open` is provided) */
        onOpenChange: (open: boolean) => void;
      }
    | {
        open?: never;
        onOpenChange?: never;
      }
  );

function optionToItem(opt: SelectOption): CommandMenuItem {
  return {
    value: opt.value,
    label: opt.label,
    description: opt.description,
    rightDecorator: opt.rightDecorator,
    disabled: opt.disabled,
    keywords: opt.label ? [opt.label] : undefined,
  };
}

function OptionSelect<T extends string = string>({
  value,
  onChange,
  options,
  groups,
  placeholder = 'Select...',
  searchPlaceholder = 'Search...',
  disabled = false,
  size = 'sm',
  emptyText = 'No results found.',
  open: controlledOpen,
  onOpenChange: controlledOnOpenChange,
  allowDeselect,
  hideSearch = true,
  disableFilter,
  onSearchChange,
  loading,
  loadingRows,
  onEndReached,
  endReachedThresholdPx,
  triggerClassName,
  contentClassName,
  renderValue,
  renderOption,
  side,
  align = 'start',
  'aria-label': ariaLabel,
  'data-testid': dataTestId,
}: SelectProps<T>) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? controlledOpen : uncontrolledOpen;
  const setOpen = isControlled ? controlledOnOpenChange : setUncontrolledOpen;

  const allOptions = useMemo<SelectOption<T>[]>(
    () => (groups ? groups.flatMap((g) => g.options) : (options ?? [])),
    [groups, options]
  );

  const selectedOption = useMemo(
    () => allOptions.find((opt) => opt.value === value),
    [allOptions, value]
  );

  const handleSelect = useCallback(
    (optionValue: string) => {
      const match = allOptions.find((o) => o.value === optionValue);
      if (!match) return;
      if (allowDeselect && match.value === value) {
        onChange(undefined);
      } else {
        onChange(match.value);
      }
      setOpen(false);
    },
    [allowDeselect, value, onChange, setOpen, allOptions]
  );

  const items: CommandMenuItem[] = useMemo(
    () => (options ?? []).map(optionToItem),
    [options]
  );

  const commandGroups: CommandMenuGroup[] | undefined = useMemo(
    () =>
      groups?.map((g) => ({
        heading: g.heading,
        items: g.options.map(optionToItem),
      })),
    [groups]
  );

  const renderItem = renderOption
    ? (item: CommandMenuItem, isSelected: boolean) => {
        const option = allOptions.find((o) => o.value === item.value);
        return option ? renderOption(option, isSelected) : null;
      }
    : undefined;
  const searchInputSize = size === 'xs' ? 'sm' : size === 'lg' ? 'md' : size;
  const triggerButtonSize = size === 'lg' ? 'md' : size;

  const commandMenuSharedProps = {
    value,
    onSelect: handleSelect,
    placeholder: searchPlaceholder,
    emptyText,
    hideSearch,
    disableFilter,
    onSearchChange,
    loading,
    loadingRows,
    onEndReached,
    endReachedThresholdPx,
    renderItem,
    inputSize: searchInputSize,
  };

  const triggerContent =
    selectedOption && renderValue ? (
      renderValue(selectedOption)
    ) : (
      <Text
        as="span"
        variant={triggerButtonSize}
        weight="normal"
        className="min-w-0 truncate text-left"
      >
        {selectedOption
          ? (selectedOption.label ?? selectedOption.value)
          : placeholder}
      </Text>
    );

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          role="combobox"
          aria-expanded={open}
          aria-label={ariaLabel ?? placeholder}
          variant="outlined"
          color="secondary"
          size={triggerButtonSize}
          className={cn(
            'w-full justify-between',
            CONTROL_HEIGHT_CLASSES[size],
            triggerClassName
          )}
          disabled={disabled}
          rightDecorator={ChevronDownIcon}
          data-testid={dataTestId}
        >
          {triggerContent}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className={cn(
          'w-max min-w-[var(--radix-popover-trigger-width)] max-w-[min(24rem,calc(100vw-1rem))] p-0 text-primary',
          contentClassName
        )}
        side={side}
        align={align}
        onCloseAutoFocus={(e) => e.preventDefault()}
      >
        {commandGroups ? (
          <CommandMenu groups={commandGroups} {...commandMenuSharedProps} />
        ) : (
          <CommandMenu items={items} {...commandMenuSharedProps} />
        )}
      </PopoverContent>
    </Popover>
  );
}

export function Select<T extends string = string>(
  props: SelectProps<T>
): React.ReactElement<unknown> {
  return <OptionSelect {...props} />;
}
