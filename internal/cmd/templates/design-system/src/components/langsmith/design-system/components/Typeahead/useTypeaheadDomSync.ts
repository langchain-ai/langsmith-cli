import type { RefObject } from 'react';
import { useEffect } from 'react';

export function useTypeaheadDomSync({
  autoComplete,
  commandLabel,
  inputAriaLabel,
  inputRef,
  inputValue,
  isError,
  listId,
  listRef,
  multiple,
  open,
  resolvedInputId,
  selectedSummaryId,
}: {
  autoComplete?: string;
  commandLabel: string;
  inputAriaLabel?: string;
  inputRef: RefObject<HTMLInputElement | null>;
  inputValue: string;
  isError: boolean;
  listId: string;
  listRef: RefObject<HTMLDivElement | null>;
  multiple?: boolean;
  open: boolean;
  resolvedInputId: string;
  selectedSummaryId: string;
}) {
  useEffect(() => {
    // cmdk owns these DOM nodes and may rewrite combobox attributes as input changes.
    const input = inputRef.current;
    const list = listRef.current;

    input?.setAttribute('id', resolvedInputId);
    input?.setAttribute('aria-controls', listId);
    input?.setAttribute('aria-expanded', String(open));
    input?.setAttribute('aria-haspopup', 'listbox');
    input?.setAttribute('autocomplete', autoComplete ?? 'off');
    if (inputAriaLabel) {
      input?.setAttribute('aria-label', inputAriaLabel);
    } else {
      input?.removeAttribute('aria-label');
      input?.removeAttribute('aria-labelledby');
    }
    if (isError) {
      input?.setAttribute('aria-invalid', 'true');
    } else {
      input?.removeAttribute('aria-invalid');
    }
    if (multiple) {
      input?.setAttribute('aria-describedby', selectedSummaryId);
    } else {
      input?.removeAttribute('aria-describedby');
    }

    list?.setAttribute('id', listId);
    list?.setAttribute('aria-label', `${commandLabel} options`);
    if (multiple) {
      list?.setAttribute('aria-multiselectable', 'true');
    } else {
      list?.removeAttribute('aria-multiselectable');
    }
  }, [
    autoComplete,
    commandLabel,
    inputAriaLabel,
    inputRef,
    inputValue,
    isError,
    listId,
    listRef,
    multiple,
    open,
    resolvedInputId,
    selectedSummaryId,
  ]);
}
