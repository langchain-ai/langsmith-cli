import type { CheckedState } from '@radix-ui/react-checkbox';

/**
 * Derives a Radix CheckedState from "all selected" and "some selected" booleans.
 * Useful for "select all" checkboxes that need to show an indeterminate state.
 */
export function getCheckedState(
  allSelected: boolean,
  someSelected: boolean
): CheckedState {
  if (allSelected) return true;
  if (someSelected) return 'indeterminate';
  return false;
}
