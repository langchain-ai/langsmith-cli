import type { FocusEvent, KeyboardEvent, ReactElement, Ref } from 'react';
import {
  forwardRef,
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from 'react';

import type {
  TypeaheadMultipleValue,
  TypeaheadProps,
  TypeaheadSelectedValue,
  TypeaheadSingleValue,
  TypeaheadTagProps,
} from './Typeahead.types';
import {
  defaultOptionLabel,
  defaultOptionValue,
  getTypeaheadCreateState,
  getTypeaheadDisplayInputValue,
  getTypeaheadListItems,
  getTypeaheadSelectedSummary,
  isActiveOptionSelected,
  isTypeaheadOption,
} from './Typeahead.utils';
import { TypeaheadView } from './TypeaheadView';
import { useTypeaheadDomSync } from './useTypeaheadDomSync';
import { useTypeaheadUncontrolledInput } from './useTypeaheadUncontrolledInput';

function TypeaheadInner<TOption = string>(
  {
    options,
    value,
    onChange,
    placeholder = 'Search...',
    showPlaceholderWithValues = false,
    emptyText = 'No results found.',
    size = 'md',
    disabled = false,
    isError = false,
    freeSolo = false,
    multiple,
    disableClearable = false,
    disableCloseOnSelect = false,
    forcePopupIcon = true,
    clearOnBlur,
    clearValueOnInputClear = false,
    displaySelectedValueWhenInputEmpty = false,
    maxVisibleOptions,
    hideEmptyList = false,
    emptyState,
    listFooter,
    onCreateNew,
    createNewLabel,
    inputValue: controlledInputValue,
    onInputChange,
    getOptionLabel,
    getOptionValue,
    getOptionDisabled,
    renderOption,
    renderTags,
    className,
    inputId,
    autoFocus,
    autoComplete,
    onKeyDown,
    onBlur,
    'aria-label': ariaLabel,
    'data-testid': dataTestId,
    ...rest
  }: TypeaheadProps<TOption>,
  ref: Ref<HTMLDivElement>
) {
  const generatedId = useId();
  const resolvedInputId = inputId ?? generatedId;
  const listId = `${resolvedInputId}-listbox`;
  const selectedSummaryId = `${resolvedInputId}-selected-summary`;
  const commandLabel = ariaLabel ?? placeholder;
  const inputAriaLabel = ariaLabel ?? (inputId ? undefined : placeholder);
  const internalInputRef = useRef<HTMLInputElement | null>(null);
  const listRef = useRef<HTMLDivElement | null>(null);
  const popoverContentRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpenState] = useState(false);
  const [uncontrolledInputValue, setUncontrolledInputValue] = useState('');
  const setOpen = useCallback(
    (nextOpen: boolean) => {
      if (!disabled || !nextOpen) {
        setOpenState(nextOpen);
      }
    },
    [disabled]
  );
  useEffect(() => {
    if (disabled) setOpenState(false);
    if (autoFocus)
      requestAnimationFrame(() => internalInputRef.current?.focus());
  }, [autoFocus, disabled]);

  const inputValue = controlledInputValue ?? uncontrolledInputValue;

  const getLabel = useCallback(
    (option: TypeaheadSelectedValue<TOption>) =>
      getOptionLabel ? getOptionLabel(option) : defaultOptionLabel(option),
    [getOptionLabel]
  );

  const getValue = useCallback(
    (option: TypeaheadSelectedValue<TOption>) =>
      getOptionValue ? getOptionValue(option) : defaultOptionValue(option),
    [getOptionValue]
  );

  const selectedValues = useMemo<TypeaheadMultipleValue<TOption>>(() => {
    if (multiple) {
      return Array.isArray(value) ? value : [];
    }
    return value == null ? [] : [value];
  }, [multiple, value]);

  const selectedKeys = useMemo(
    () => new Set(selectedValues.map((selected) => getValue(selected))),
    [getValue, selectedValues]
  );

  const setInputValue = useCallback(
    (nextValue: string) => {
      if (controlledInputValue === undefined) {
        setUncontrolledInputValue(nextValue);
      }
      onInputChange?.(nextValue);
    },
    [controlledInputValue, onInputChange]
  );

  useTypeaheadUncontrolledInput({
    clearOnBlur,
    controlledInputValue,
    freeSolo,
    getLabel,
    multiple,
    open,
    setUncontrolledInputValue,
    value,
  });

  const visibleItems = getTypeaheadListItems({
    options,
    selectedValues,
    inputValue,
    getLabel,
    getValue,
    maxVisibleOptions,
    includeSelectedValues: multiple,
  });

  const optionIsDisabled = useCallback(
    (item: (typeof visibleItems)[number]) => {
      if (item.source === 'selected') return false;
      const option = item.option as TOption;
      return (
        getOptionDisabled?.(option) ??
        (isTypeaheadOption(option) ? !!option.disabled : false)
      );
    },
    [getOptionDisabled]
  );

  const {
    customCreateInputValue,
    showCreateOption,
    showFreeSoloCreateOption,
    trimmedInputValue,
  } = getTypeaheadCreateState(
    inputValue,
    options,
    selectedValues,
    getLabel,
    freeSolo,
    !!onCreateNew
  );
  const hasPopupContent =
    visibleItems.length > 0 ||
    showCreateOption ||
    emptyState != null ||
    listFooter != null ||
    !hideEmptyList;
  const popupOpen = open && hasPopupContent;

  const focusInput = useCallback(() => {
    requestAnimationFrame(() => internalInputRef.current?.focus());
  }, []);

  // `onChange` can't be narrowed from the single/multiple props union after destructuring; callers below branch on `multiple` first, so funnel both shapes through one callback.
  const emitChange = useCallback(
    (next: TypeaheadSingleValue<TOption> | TypeaheadMultipleValue<TOption>) =>
      (onChange as (value: typeof next) => void)(next),
    [onChange]
  );

  const selectValue = useCallback(
    (selected: TypeaheadSelectedValue<TOption>) => {
      if (disabled) return;
      if (multiple) {
        const selectedKey = getValue(selected);
        const nextValue = selectedKeys.has(selectedKey)
          ? selectedValues.filter((item) => getValue(item) !== selectedKey)
          : [...selectedValues, selected];

        emitChange(nextValue);
        setInputValue('');
        if (disableCloseOnSelect) {
          focusInput();
        } else {
          setOpen(false);
        }
        return;
      }

      emitChange(selected);
      setInputValue(
        displaySelectedValueWhenInputEmpty ? '' : getLabel(selected)
      );
      setOpen(false);
    },
    [
      disableCloseOnSelect,
      disabled,
      emitChange,
      focusInput,
      getLabel,
      getValue,
      displaySelectedValueWhenInputEmpty,
      multiple,
      selectedKeys,
      selectedValues,
      setInputValue,
      setOpen,
    ]
  );

  const createFreeSoloValue = useCallback(() => {
    if (!showFreeSoloCreateOption) return;
    selectValue(trimmedInputValue);
  }, [selectValue, showFreeSoloCreateOption, trimmedInputValue]);

  const createCustomValue = useCallback(() => {
    if (!onCreateNew) return;
    onCreateNew(customCreateInputValue);
    setOpen(false);
  }, [customCreateInputValue, onCreateNew, setOpen]);

  const removeAtIndex = useCallback(
    (index: number) => {
      if (!multiple || disabled || index < 0) return;
      emitChange(selectedValues.filter((_, i) => i !== index));
      focusInput();
    },
    [disabled, emitChange, focusInput, multiple, selectedValues]
  );

  const clearValue = useCallback(() => {
    if (disabled || disableClearable) return;
    if (multiple) {
      emitChange([]);
    } else {
      emitChange(null);
    }
    setInputValue('');
    setOpen(false);
    focusInput();
  }, [
    disableClearable,
    disabled,
    emitChange,
    focusInput,
    multiple,
    setInputValue,
    setOpen,
  ]);

  const handleInputValueChange = useCallback(
    (nextValue: string) => {
      if (disabled) return;
      if (
        clearValueOnInputClear &&
        !multiple &&
        nextValue.length === 0 &&
        selectedValues.length > 0
      ) {
        emitChange(null);
      }
      setInputValue(nextValue);
      setOpen(true);
    },
    [
      clearValueOnInputClear,
      disabled,
      emitChange,
      multiple,
      selectedValues.length,
      setInputValue,
      setOpen,
    ]
  );

  const handleKeyDown = useCallback(
    (event: KeyboardEvent<HTMLInputElement>) => {
      onKeyDown?.(event);
      if (event.defaultPrevented || disabled) return;

      if (event.key === 'Backspace' && multiple && inputValue.length === 0) {
        removeAtIndex(selectedValues.length - 1);
        return;
      }

      if (
        event.key === 'Enter' &&
        multiple &&
        freeSolo &&
        popupOpen &&
        isActiveOptionSelected(listRef.current, selectedValues, getValue)
      ) {
        event.preventDefault();
        if (showFreeSoloCreateOption) createFreeSoloValue();
        else setInputValue('');
        return;
      }

      // When open, cmdk owns Enter so active options and create rows win over free-solo.
      if (event.key === 'ArrowDown' || event.key === 'Enter') {
        if (!open && hasPopupContent) {
          event.preventDefault();
          setOpen(true);
        }
        return;
      }

      if (event.key === 'Escape' && open) {
        if (popupOpen) {
          event.preventDefault();
        }
        setOpen(false);
      }
    },
    [
      createFreeSoloValue,
      disabled,
      freeSolo,
      getValue,
      hasPopupContent,
      inputValue.length,
      multiple,
      onKeyDown,
      open,
      popupOpen,
      removeAtIndex,
      selectedValues,
      setInputValue,
      setOpen,
      showFreeSoloCreateOption,
    ]
  );

  const handleBlur = useCallback(
    (event: FocusEvent<HTMLDivElement>) => {
      onBlur?.(event);
      const nextTarget = event.relatedTarget as Node | null;
      if (
        !nextTarget ||
        (!event.currentTarget.contains(nextTarget) &&
          !popoverContentRef.current?.contains(nextTarget))
      ) {
        setOpen(false);
      }
    },
    [onBlur, setOpen]
  );

  const getTagProps = useCallback(
    ({ index }: { index: number }): TypeaheadTagProps => ({
      key: `${getValue(selectedValues[index])}-${index}`,
      tabIndex: -1,
      'data-tag-index': index,
      onDelete: () => removeAtIndex(index),
    }),
    [getValue, removeAtIndex, selectedValues]
  );

  const hasValue = selectedValues.length > 0;
  const showClearButton =
    !disabled &&
    !disableClearable &&
    (hasValue || (!multiple && inputValue.length > 0));
  const selectedSummary = getTypeaheadSelectedSummary(
    selectedValues,
    getLabel,
    multiple
  );
  const displayInputValue = getTypeaheadDisplayInputValue(
    inputValue,
    selectedValues,
    getLabel,
    multiple,
    displaySelectedValueWhenInputEmpty
  );

  const setInputNode = useCallback((node: HTMLInputElement | null) => {
    internalInputRef.current = node;
  }, []);

  useTypeaheadDomSync({
    autoComplete,
    commandLabel,
    inputAriaLabel,
    inputRef: internalInputRef,
    inputValue,
    isError,
    listId,
    listRef,
    multiple,
    open: popupOpen,
    resolvedInputId,
    selectedSummaryId,
  });

  return (
    <TypeaheadView
      root={{
        ref,
        props: rest,
        handleBlur,
        dataTestId,
        className,
      }}
      state={{
        size,
        disabled,
        isError,
        multiple,
        open: popupOpen,
        setOpen,
      }}
      ids={{
        commandLabel,
        resolvedInputId,
        listId,
        selectedSummaryId,
        inputAriaLabel,
      }}
      input={{
        value: displayInputValue,
        onValueChange: handleInputValueChange,
        onKeyDown: handleKeyDown,
        placeholder,
        showPlaceholderWithValues,
        autoFocus,
        autoComplete,
        setNode: setInputNode,
        focus: focusInput,
      }}
      selection={{
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
      }}
      list={{
        ref: listRef,
        items: visibleItems,
        emptyText,
        emptyState,
        listFooter,
        hideEmptyList,
        optionIsDisabled,
        selectValue,
        renderOption,
      }}
      createOption={{
        show: showCreateOption,
        inputValue: onCreateNew ? customCreateInputValue : trimmedInputValue,
        label: createNewLabel?.(customCreateInputValue),
        onCreate: onCreateNew ? createCustomValue : createFreeSoloValue,
      }}
      popoverContentRef={popoverContentRef}
    />
  );
}

export const Typeahead = forwardRef(TypeaheadInner) as <TOption = string>(
  props: TypeaheadProps<TOption> & { ref?: Ref<HTMLDivElement> }
) => ReactElement<unknown>;
