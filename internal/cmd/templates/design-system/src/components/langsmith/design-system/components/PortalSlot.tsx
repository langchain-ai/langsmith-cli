import type { HTMLAttributes, ReactNode } from 'react';
import {
  createContext,
  useContext,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';

import { createPortal } from 'react-dom';

export const createPortalSlot = () => {
  const SlotContext = createContext<
    [HTMLElement | null, (node: HTMLElement | null) => void] | null
  >(null);

  const Fill = (props: { children?: ReactNode }) => {
    const [ref] = useContext(SlotContext) ?? [];

    if (!ref) return null;
    return createPortal(props.children, ref);
  };

  const useCanFillSlot = () => {
    const context = useContext(SlotContext);
    return context != null;
  };

  const Slot = (props: HTMLAttributes<HTMLDivElement>) => {
    const ref = useRef<HTMLDivElement>(null);
    const [, setStateRef] = useContext(SlotContext) ?? [];
    useLayoutEffect(() => setStateRef?.(ref.current), [setStateRef]);

    return <div {...props} ref={ref} />;
  };

  function Context(props: { children?: ReactNode }) {
    const [state, setState] = useState<HTMLElement | null>(null);

    return (
      <SlotContext.Provider value={[state, setState]}>
        {props.children}
      </SlotContext.Provider>
    );
  }

  return { Fill, Slot, Context, useCanFillSlot };
};
