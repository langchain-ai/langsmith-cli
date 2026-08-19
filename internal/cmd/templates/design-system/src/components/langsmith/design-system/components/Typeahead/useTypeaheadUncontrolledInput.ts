import { useEffect } from 'react';

import type {
  TypeaheadMultipleValue,
  TypeaheadSelectedValue,
  TypeaheadSingleValue,
} from './Typeahead.types';

export function useTypeaheadUncontrolledInput<TOption>({
  clearOnBlur,
  controlledInputValue,
  freeSolo,
  getLabel,
  multiple,
  open,
  setUncontrolledInputValue,
  value,
}: {
  clearOnBlur?: boolean;
  controlledInputValue?: string;
  freeSolo: boolean;
  getLabel: (option: TypeaheadSelectedValue<TOption>) => string;
  multiple?: boolean;
  open: boolean;
  setUncontrolledInputValue: (value: string) => void;
  value: TypeaheadSingleValue<TOption> | TypeaheadMultipleValue<TOption>;
}) {
  const singleValue: TypeaheadSingleValue<TOption> = Array.isArray(value)
    ? null
    : value;

  useEffect(() => {
    if (controlledInputValue !== undefined || multiple) return;
    setUncontrolledInputValue(singleValue == null ? '' : getLabel(singleValue));
  }, [
    controlledInputValue,
    getLabel,
    multiple,
    setUncontrolledInputValue,
    singleValue,
  ]);

  useEffect(() => {
    if (open || controlledInputValue !== undefined) return;
    if (clearOnBlur ?? !freeSolo) {
      setUncontrolledInputValue(
        multiple || singleValue == null ? '' : getLabel(singleValue)
      );
    }
  }, [
    clearOnBlur,
    controlledInputValue,
    freeSolo,
    getLabel,
    multiple,
    open,
    setUncontrolledInputValue,
    singleValue,
  ]);
}
