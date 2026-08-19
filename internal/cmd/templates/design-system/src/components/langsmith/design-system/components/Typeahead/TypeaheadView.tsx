import type {
  FocusEvent,
  HTMLAttributes,
  KeyboardEvent,
  ReactNode,
  Ref,
} from 'react';
import { useRef } from 'react';

import { Command as CommandPrimitive } from 'cmdk';

import { ChevronDownIcon, SearchMdIcon } from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';
import { Command, CommandEmpty, CommandItem, CommandList } from '../Command';
import { Popover, PopoverAnchor, PopoverContent } from '../Popover';
import { Text } from '../Text';
import type {
  TypeaheadListItem,
  TypeaheadMultipleValue,
  TypeaheadRenderOptionState,
  TypeaheadSelectedValue,
  TypeaheadTagProps,
} from './Typeahead.types';
import { isTypeaheadOption } from './Typeahead.utils';
import {
  TypeaheadClearButton,
  TypeaheadCreateOption,
  TypeaheadDefaultOption,
  TypeaheadDefaultTag,
} from './TypeaheadParts';

interface TypeaheadViewRootProps {
  ref: Ref<HTMLDivElement>;
  props: HTMLAttributes<HTMLDivElement>;
  handleBlur: (event: FocusEvent<HTMLDivElement>) => void;
  dataTestId?: string;
  className?: string;
}

interface TypeaheadViewState {
  size: 'xs' | 'sm' | 'md';
  disabled: boolean;
  isError: boolean;
  multiple?: boolean;
  open: boolean;
  setOpen: (open: boolean) => void;
}

interface TypeaheadViewIds {
  commandLabel: string;
  resolvedInputId: string;
  listId: string;
  selectedSummaryId: string;
  inputAriaLabel?: string;
}

interface TypeaheadViewInputProps {
  value: string;
  onValueChange: (value: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLInputElement>) => void;
  placeholder: string;
  showPlaceholderWithValues: boolean;
  autoFocus?: boolean;
  autoComplete?: string;
  setNode: (node: HTMLInputElement | null) => void;
  focus: () => void;
}

interface TypeaheadViewSelectionProps<TOption> {
  selectedValues: TypeaheadMultipleValue<TOption>;
  selectedKeys: Set<string>;
  getTagProps: (params: { index: number }) => TypeaheadTagProps;
  renderTags?: (
    value: TypeaheadMultipleValue<TOption>,
    getTagProps: (params: { index: number }) => TypeaheadTagProps
  ) => ReactNode;
  getLabel: (option: TypeaheadSelectedValue<TOption>) => string;
  getValue: (option: TypeaheadSelectedValue<TOption>) => string;
  showClearButton: boolean;
  clearValue: () => void;
  forcePopupIcon: boolean;
  selectedSummary?: string;
}

interface TypeaheadViewListProps<TOption> {
  ref: Ref<HTMLDivElement>;
  items: TypeaheadListItem<TOption>[];
  emptyText: string;
  emptyState?: ReactNode;
  listFooter?: ReactNode;
  hideEmptyList: boolean;
  optionIsDisabled: (item: TypeaheadListItem<TOption>) => boolean;
  selectValue: (selected: TypeaheadSelectedValue<TOption>) => void;
  renderOption?: (
    option: TOption,
    state: TypeaheadRenderOptionState
  ) => ReactNode;
}

interface TypeaheadViewCreateOptionProps {
  show: boolean;
  inputValue: string;
  label?: ReactNode;
  onCreate: () => void;
}

interface TypeaheadViewProps<TOption> {
  root: TypeaheadViewRootProps;
  state: TypeaheadViewState;
  ids: TypeaheadViewIds;
  input: TypeaheadViewInputProps;
  selection: TypeaheadViewSelectionProps<TOption>;
  list: TypeaheadViewListProps<TOption>;
  createOption: TypeaheadViewCreateOptionProps;
  popoverContentRef: Ref<HTMLDivElement>;
}

// Keep same-sized Typeahead and Button controls height-compatible.
const CONTROL_HEIGHT_CLASSES = {
  xs: 'h-[1.5625rem]',
  sm: 'h-[1.625rem]',
  md: 'h-[2.1875rem]',
} as const;

const CONTROL_MIN_HEIGHT_CLASSES = {
  xs: 'min-h-[1.5625rem]',
  sm: 'min-h-[1.625rem]',
  md: 'min-h-[2.1875rem]',
} as const;

const CONTROL_PADDING_CLASSES = {
  xs: 'px-space-2',
  sm: 'px-space-2',
  md: 'px-space-3',
} as const;

const CONTROL_RADIUS_CLASSES = {
  xs: 'rounded-xs',
  sm: 'rounded-sm',
  md: 'rounded-md',
} as const;

const INPUT_TEXT_CLASSES = {
  xs: 'text-xxs leading-[1.15]',
  sm: 'text-xs leading-normal',
  md: 'text-sm leading-normal',
} as const;

const INPUT_MIN_WIDTH_CLASSES = {
  xs: 'min-w-12',
  sm: 'min-w-16',
  md: 'min-w-16',
} as const;

export function TypeaheadView<TOption>({
  root,
  state,
  ids,
  input,
  selection,
  list,
  createOption,
  popoverContentRef,
}: TypeaheadViewProps<TOption>) {
  const controlRef = useRef<HTMLDivElement | null>(null);
  const preventInputDismiss = (event: Event) => {
    if (
      open &&
      event.target instanceof HTMLInputElement &&
      event.target.getAttribute('role') === 'combobox' &&
      controlRef.current?.contains(event.target)
    ) {
      event.preventDefault();
    }
  };
  const {
    ref: rootRef,
    props: rootProps,
    handleBlur,
    dataTestId,
    className,
  } = root;
  const { size, disabled, isError, multiple, open, setOpen } = state;
  const {
    commandLabel,
    resolvedInputId,
    listId,
    selectedSummaryId,
    inputAriaLabel,
  } = ids;
  const {
    value: inputValue,
    onValueChange: handleInputValueChange,
    onKeyDown: handleKeyDown,
    placeholder,
    showPlaceholderWithValues,
    autoFocus,
    autoComplete,
    setNode: setInputNode,
    focus: focusInput,
  } = input;
  const {
    selectedValues,
    selectedKeys,
    getTagProps,
    renderTags,
    getLabel,
    getValue,
    showClearButton,
    clearValue,
    forcePopupIcon,
    selectedSummary,
  } = selection;
  const {
    ref: listRef,
    items,
    emptyText,
    emptyState,
    listFooter,
    hideEmptyList,
    optionIsDisabled,
    selectValue,
    renderOption,
  } = list;
  const hasValue = selectedValues.length > 0;
  const visibleItems = open ? items : [];
  const visibleShowCreateOption = open && createOption.show;
  const showEmptyState =
    open &&
    visibleItems.length === 0 &&
    !visibleShowCreateOption &&
    (emptyState != null || !hideEmptyList);
  const controlHeightClass = multiple
    ? CONTROL_MIN_HEIGHT_CLASSES[size]
    : CONTROL_HEIGHT_CLASSES[size];
  const renderedTags =
    multiple &&
    (renderTags
      ? renderTags(selectedValues, getTagProps)
      : selectedValues.map((selected, index) => {
          const tagProps = getTagProps({ index });
          return (
            <TypeaheadDefaultTag
              key={`${getValue(selected)}-${index}`}
              selected={selected}
              index={index}
              disabled={disabled}
              getLabel={getLabel}
              getValue={getValue}
              onRemove={tagProps.onDelete}
            />
          );
        }));
  const inputField = (
    <div
      className={cn(
        'flex items-center gap-space-2',
        multiple ? 'min-w-0 flex-1' : 'flex-1',
        !multiple && INPUT_MIN_WIDTH_CLASSES[size]
      )}
    >
      {!multiple && !hasValue && (
        <div className="flex items-center text-icon-tertiary">
          <SearchMdIcon
            aria-hidden="true"
            className="size-4 text-icon-tertiary"
          />
        </div>
      )}
      {/*
        cmdk rewrites some of these combobox attributes as the query changes, so
        useTypeaheadDomSync re-asserts them imperatively. Keep the two attribute
        sets in sync when editing either here or in that hook.
      */}
      <CommandPrimitive.Input
        ref={setInputNode}
        id={resolvedInputId}
        value={inputValue}
        onValueChange={handleInputValueChange}
        onFocus={() => {
          if (!disabled) {
            setOpen(true);
          }
        }}
        onKeyDown={handleKeyDown}
        placeholder={
          hasValue && multiple && !showPlaceholderWithValues
            ? undefined
            : placeholder
        }
        disabled={disabled}
        autoFocus={autoFocus}
        autoComplete={autoComplete}
        role="combobox"
        aria-expanded={open}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-haspopup="listbox"
        aria-invalid={isError || undefined}
        aria-describedby={multiple ? selectedSummaryId : undefined}
        aria-label={inputAriaLabel}
        className={cn(
          'min-w-0 max-w-full border-none bg-transparent p-0 outline-none placeholder:text-placeholder',
          INPUT_TEXT_CLASSES[size],
          disabled && 'cursor-not-allowed',
          multiple ? 'h-6 w-full' : 'h-full w-full flex-1'
        )}
      />
    </div>
  );
  const actionControls = (
    <div
      className={cn(
        'flex shrink-0 items-center gap-space-1 text-icon-tertiary',
        multiple ? 'self-center' : 'ml-auto'
      )}
    >
      {showClearButton && (
        <TypeaheadClearButton
          label={multiple ? 'Clear selections' : 'Clear selection'}
          onClick={(event) => {
            event.stopPropagation();
            clearValue();
          }}
        />
      )}
      {forcePopupIcon && (
        <ChevronDownIcon
          aria-hidden="true"
          className={cn('size-4 transition-transform', open && 'rotate-180')}
        />
      )}
    </div>
  );

  return (
    <Command
      ref={rootRef}
      shouldFilter={false}
      label={commandLabel}
      className="overflow-visible bg-transparent"
    >
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverAnchor asChild>
          <div
            ref={controlRef}
            {...rootProps}
            data-testid={dataTestId}
            onBlur={handleBlur}
            className={cn(
              'w-full cursor-text border border-default bg-elevated text-primary transition-colors',
              multiple
                ? 'flex flex-wrap items-center gap-space-1 p-space-1'
                : 'flex items-center gap-space-1 overflow-hidden',
              'focus-within:border-focus focus-within:bg-elevated hover:bg-elevated-hover',
              controlHeightClass,
              !multiple && CONTROL_PADDING_CLASSES[size],
              CONTROL_RADIUS_CLASSES[size],
              isError && 'border-error focus-within:border-error',
              disabled &&
                'cursor-not-allowed border-disabled bg-disabled text-disabled opacity-70',
              className
            )}
            onClick={(event) => {
              rootProps.onClick?.(event);
              if (!disabled) {
                focusInput();
                setOpen(true);
              }
            }}
          >
            {multiple ? (
              <>
                {renderedTags}
                <div className="flex min-w-16 flex-1 items-center gap-space-1">
                  {inputField}
                  {actionControls}
                </div>
              </>
            ) : (
              <>
                {inputField}
                {actionControls}
              </>
            )}
          </div>
        </PopoverAnchor>

        {multiple && (
          <span
            id={selectedSummaryId}
            className="sr-only"
            aria-live="polite"
            aria-atomic="true"
          >
            {selectedSummary}
          </span>
        )}

        <PopoverContent
          ref={popoverContentRef}
          hidden={!open}
          className={cn(
            'w-[var(--radix-popover-trigger-width)] min-w-48 max-w-[calc(100vw-1rem)] bg-elevated p-space-1 text-primary',
            !open && 'hidden'
          )}
          align="start"
          sideOffset={4}
          forceMount
          onOpenAutoFocus={(event) => event.preventDefault()}
          onCloseAutoFocus={(event) => event.preventDefault()}
          onFocusOutside={preventInputDismiss}
          onInteractOutside={preventInputDismiss}
        >
          <CommandList
            ref={listRef}
            id={listId}
            aria-label={`${commandLabel} options`}
            aria-multiselectable={multiple ? true : undefined}
            className="max-h-[18.75rem]"
          >
            {showEmptyState && (
              <CommandEmpty>
                {emptyState ?? (
                  <Text variant="sm" color="secondary">
                    {emptyText}
                  </Text>
                )}
              </CommandEmpty>
            )}

            {visibleItems.map((item) => {
              const { option } = item;
              const optionKey = getValue(option);
              const optionLabel = getLabel(option);
              const selected = selectedKeys.has(optionKey);
              const optionDisabled = optionIsDisabled(item);
              const optionState = {
                selected,
                disabled: optionDisabled,
                inputValue,
              };

              return (
                <CommandItem
                  key={optionKey}
                  value={optionKey}
                  disabled={optionDisabled}
                  keywords={
                    isTypeaheadOption(option)
                      ? [optionLabel, ...(option.keywords ?? [])]
                      : [optionLabel]
                  }
                  onSelect={() => selectValue(option)}
                  aria-checked={multiple ? selected : undefined}
                  className={cn(
                    'flex items-center gap-space-2 rounded-sm px-space-2 py-space-2 text-sm text-primary hover:bg-surface-level-2 aria-selected:bg-surface-level-2',
                    optionDisabled && 'text-disabled'
                  )}
                >
                  {renderOption && item.source === 'option' ? (
                    renderOption(option as TOption, optionState)
                  ) : (
                    <TypeaheadDefaultOption
                      option={option}
                      state={optionState}
                      getLabel={getLabel}
                    />
                  )}
                </CommandItem>
              );
            })}

            {visibleShowCreateOption && (
              <CommandItem
                value={`__typeahead_create__${createOption.inputValue}`}
                keywords={[createOption.inputValue]}
                onSelect={createOption.onCreate}
                aria-label={
                  createOption.inputValue
                    ? `Add ${createOption.inputValue}`
                    : 'Add new'
                }
                className="flex items-center gap-space-2 rounded-sm p-space-2 text-sm text-primary hover:bg-surface-level-2 aria-selected:bg-surface-level-2"
              >
                <TypeaheadCreateOption
                  inputValue={createOption.inputValue}
                  label={createOption.label}
                />
              </CommandItem>
            )}

            {listFooter}
          </CommandList>
        </PopoverContent>
      </Popover>
    </Command>
  );
}
