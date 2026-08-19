import type { ReactNode } from 'react';
import React, { useRef } from 'react';

import { ScrollParentContext } from './useScrollParent';

interface ScrollParentProviderProps {
  children: ReactNode;
}

export const ScrollParentProvider: React.FC<ScrollParentProviderProps> = ({
  children,
}) => {
  const scrollParentRef = useRef<HTMLDivElement>(null);

  return (
    <ScrollParentContext.Provider value={scrollParentRef}>
      {children}
    </ScrollParentContext.Provider>
  );
};
