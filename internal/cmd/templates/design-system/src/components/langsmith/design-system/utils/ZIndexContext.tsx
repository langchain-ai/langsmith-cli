import type { ReactNode } from 'react';

import { ZIndexContext } from './useZIndex';

export function ZIndexProvider({
  value,
  children,
}: {
  value: number;
  children: ReactNode;
}) {
  return (
    <ZIndexContext.Provider value={value}>{children}</ZIndexContext.Provider>
  );
}
