import type { ReactNode } from 'react';

export const getAccessibleText = (value: ReactNode) =>
  typeof value === 'string' ||
  typeof value === 'number' ||
  typeof value === 'bigint'
    ? String(value)
    : undefined;

export const widthsMatch = (
  current: Record<string, number>,
  next: Record<string, number>
) => {
  const currentKeys = Object.keys(current);
  const nextKeys = Object.keys(next);

  return (
    currentKeys.length === nextKeys.length &&
    nextKeys.every((key) => current[key] === next[key])
  );
};
