import type { Ref, MutableRefObject, RefCallback } from 'react';

export function mergeRefs<T = unknown>(
  refs: Array<MutableRefObject<T> | Ref<T> | undefined | null>
): RefCallback<T> {
  return (value) => {
    refs.forEach((ref) => {
      if (typeof ref === 'function') {
        ref(value);
      } else if (ref != null) {
        (ref as MutableRefObject<T | null>).current = value;
      }
    });
  };
}
