import type {
  TypeaheadListItem,
  TypeaheadMultipleValue,
  TypeaheadOption,
  TypeaheadSelectedValue,
} from './Typeahead.types';

export function isTypeaheadOption(option: unknown): option is TypeaheadOption {
  return (
    typeof option === 'object' &&
    option != null &&
    'value' in option &&
    typeof (option as TypeaheadOption).value === 'string'
  );
}

export function defaultOptionLabel<TOption>(
  option: TypeaheadSelectedValue<TOption>
) {
  if (isTypeaheadOption(option)) {
    return option.label ?? option.value;
  }
  return String(option ?? '');
}

export function defaultOptionValue<TOption>(
  option: TypeaheadSelectedValue<TOption>
) {
  if (isTypeaheadOption(option)) {
    return option.value;
  }
  return defaultOptionLabel(option);
}

export const normalize = (value: string) => value.trim().toLowerCase();

export function filterTypeaheadOptions<TOption>(
  options: TOption[],
  inputValue: string,
  getLabel: (option: TOption) => string,
  maxVisibleOptions?: number
) {
  const query = normalize(inputValue);
  const filteredOptions = query
    ? options.filter((option) => {
        const labelMatch = normalize(getLabel(option)).includes(query);
        if (labelMatch) {
          return true;
        }
        if (isTypeaheadOption(option)) {
          return option.keywords?.some((keyword) =>
            normalize(keyword).includes(query)
          );
        }
        return false;
      })
    : options;

  return maxVisibleOptions == null
    ? filteredOptions
    : filteredOptions.slice(0, maxVisibleOptions);
}

export function getTypeaheadListItems<TOption>({
  options,
  selectedValues,
  inputValue,
  getLabel,
  getValue,
  maxVisibleOptions,
  includeSelectedValues,
}: {
  options: TOption[];
  selectedValues: TypeaheadMultipleValue<TOption>;
  inputValue: string;
  getLabel: (option: TypeaheadSelectedValue<TOption>) => string;
  getValue: (option: TypeaheadSelectedValue<TOption>) => string;
  maxVisibleOptions?: number;
  includeSelectedValues?: boolean;
}): TypeaheadListItem<TOption>[] {
  const filteredItems = filterTypeaheadOptions(
    options,
    inputValue,
    getLabel,
    maxVisibleOptions
  ).map<TypeaheadListItem<TOption>>((option) => ({
    option,
    source: 'option',
  }));

  const query = normalize(inputValue);
  if (!includeSelectedValues || query.length === 0) return filteredItems;

  const optionKeys = new Set(options.map((option) => getValue(option)));
  const itemKeys = new Set(filteredItems.map((item) => getValue(item.option)));
  const selectedItems = selectedValues
    .filter((selected) => {
      const selectedKey = getValue(selected);
      return (
        !optionKeys.has(selectedKey) &&
        !itemKeys.has(selectedKey) &&
        normalize(getLabel(selected)).includes(query)
      );
    })
    .map<TypeaheadListItem<TOption>>((option) => ({
      option,
      source: 'selected',
    }));

  return [...filteredItems, ...selectedItems];
}

export function isActiveOptionSelected<TOption>(
  listNode: HTMLElement | null,
  selectedValues: TypeaheadMultipleValue<TOption>,
  getValue: (option: TypeaheadSelectedValue<TOption>) => string
) {
  const activeValue = listNode
    ?.querySelector('[cmdk-item][aria-selected="true"]')
    ?.getAttribute('data-value');
  if (activeValue == null) return false;
  return selectedValues.some(
    (selected) => getValue(selected).trim() === activeValue
  );
}

export function getTypeaheadDisplayInputValue<TOption>(
  inputValue: string,
  selectedValues: TypeaheadMultipleValue<TOption>,
  getLabel: (option: TypeaheadSelectedValue<TOption>) => string,
  multiple?: boolean,
  displaySelectedValueWhenInputEmpty?: boolean
) {
  return displaySelectedValueWhenInputEmpty &&
    !multiple &&
    inputValue.length === 0 &&
    selectedValues.length > 0
    ? getLabel(selectedValues[0])
    : inputValue;
}

export function getTypeaheadSelectedSummary<TOption>(
  selectedValues: TypeaheadMultipleValue<TOption>,
  getLabel: (option: TypeaheadSelectedValue<TOption>) => string,
  multiple?: boolean
) {
  if (!multiple) return undefined;
  if (selectedValues.length === 0) return 'No selected values.';
  return `Selected values: ${selectedValues.map((selected) => getLabel(selected)).join(', ')}.`;
}

export function getTypeaheadCreateState<TOption>(
  inputValue: string,
  options: TOption[],
  selectedValues: TypeaheadMultipleValue<TOption>,
  getLabel: (option: TypeaheadSelectedValue<TOption>) => string,
  freeSolo: boolean,
  hasCustomCreate: boolean
) {
  const trimmedInputValue = inputValue.trim();
  const normalizedInputValue = normalize(inputValue);
  const createMatchesOption =
    normalizedInputValue.length > 0 &&
    options.some(
      (option) => normalize(getLabel(option)) === normalizedInputValue
    );
  const createMatchesSelection =
    normalizedInputValue.length > 0 &&
    selectedValues.some(
      (selected) => normalize(getLabel(selected)) === normalizedInputValue
    );
  const showFreeSoloCreateOption =
    freeSolo &&
    !hasCustomCreate &&
    trimmedInputValue.length > 0 &&
    !createMatchesOption &&
    !createMatchesSelection;

  return {
    customCreateInputValue:
      createMatchesOption || createMatchesSelection ? '' : trimmedInputValue,
    showCreateOption: showFreeSoloCreateOption || hasCustomCreate,
    showFreeSoloCreateOption,
    trimmedInputValue,
  };
}
