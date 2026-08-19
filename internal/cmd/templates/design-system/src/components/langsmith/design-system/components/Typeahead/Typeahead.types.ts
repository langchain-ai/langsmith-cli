import type { HTMLAttributes, KeyboardEvent, ReactNode } from 'react';

export interface TypeaheadOption<TValue extends string = string> {
  value: TValue;
  label?: string;
  description?: string;
  disabled?: boolean;
  rightDecorator?: ReactNode;
  keywords?: string[];
}

export type FreeSoloValue = string & Record<never, never>;
export type TypeaheadSelectedValue<TOption> = TOption | FreeSoloValue;
export type TypeaheadSingleValue<TOption> =
  | TypeaheadSelectedValue<TOption>
  | null
  | undefined;
export type TypeaheadMultipleValue<TOption> = Array<
  TypeaheadSelectedValue<TOption>
>;

export interface TypeaheadTagProps {
  key: string;
  tabIndex: number;
  'data-tag-index': number;
  onDelete: () => void;
}

export interface TypeaheadRenderOptionState {
  selected: boolean;
  disabled: boolean;
  inputValue: string;
}

export interface TypeaheadListItem<TOption> {
  option: TypeaheadSelectedValue<TOption>;
  source: 'option' | 'selected';
}

interface TypeaheadBaseProps<TOption> extends Omit<
  HTMLAttributes<HTMLDivElement>,
  'defaultValue' | 'onChange' | 'onKeyDown'
> {
  options: TOption[];
  placeholder?: string;
  /** Keeps the placeholder visible after values have been selected in multiple mode. */
  showPlaceholderWithValues?: boolean;
  emptyText?: string;
  size?: 'xs' | 'sm' | 'md';
  disabled?: boolean;
  isError?: boolean;
  freeSolo?: boolean;
  disableClearable?: boolean;
  disableCloseOnSelect?: boolean;
  forcePopupIcon?: boolean;
  clearOnBlur?: boolean;
  clearValueOnInputClear?: boolean;
  displaySelectedValueWhenInputEmpty?: boolean;
  maxVisibleOptions?: number;
  hideEmptyList?: boolean;
  emptyState?: ReactNode;
  listFooter?: ReactNode;
  onCreateNew?: (inputValue: string) => void;
  createNewLabel?: (inputValue: string) => ReactNode;
  inputValue?: string;
  onInputChange?: (value: string) => void;
  getOptionLabel?: (option: TypeaheadSelectedValue<TOption>) => string;
  getOptionValue?: (option: TypeaheadSelectedValue<TOption>) => string;
  getOptionDisabled?: (option: TOption) => boolean;
  renderOption?: (
    option: TOption,
    state: TypeaheadRenderOptionState
  ) => ReactNode;
  inputId?: string;
  autoFocus?: boolean;
  autoComplete?: string;
  onKeyDown?: (event: KeyboardEvent<HTMLInputElement>) => void;
  'aria-label'?: string;
  'data-testid'?: string;
}

interface TypeaheadSingleProps<TOption> extends TypeaheadBaseProps<TOption> {
  /** Searchable single-select mode. */
  multiple?: false;
  value?: TypeaheadSingleValue<TOption>;
  onChange: (value: TypeaheadSingleValue<TOption>) => void;
  renderTags?: never;
}

interface TypeaheadMultipleProps<TOption> extends TypeaheadBaseProps<TOption> {
  /**
   * Enables the canonical searchable multi-select pattern. Selected values are
   * rendered as removable tags and the option list stays searchable.
   */
  multiple: true;
  value?: TypeaheadMultipleValue<TOption>;
  onChange: (value: TypeaheadMultipleValue<TOption>) => void;
  renderTags?: (
    value: TypeaheadMultipleValue<TOption>,
    getTagProps: (params: { index: number }) => TypeaheadTagProps
  ) => ReactNode;
}

/**
 * Searchable selection input. Use `multiple` for searchable multi-selects and
 * add `freeSolo` only when values outside the provided options are valid.
 */
export type TypeaheadProps<TOption = string> =
  | TypeaheadSingleProps<TOption>
  | TypeaheadMultipleProps<TOption>;
