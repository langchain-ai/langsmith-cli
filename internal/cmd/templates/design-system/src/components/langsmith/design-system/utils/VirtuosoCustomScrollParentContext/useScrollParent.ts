import { createContext, useContext } from 'react';

export const useScrollParent = () => {
  const context = useContext(ScrollParentContext);

  return context;
};
type ScrollParentContextType = React.RefObject<HTMLDivElement | null> | null;

export const ScrollParentContext = createContext<ScrollParentContextType>(null);
