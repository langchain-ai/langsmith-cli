import type { ComponentPropsWithoutRef, ElementRef } from 'react';
import { forwardRef } from 'react';

import * as HoverCardPrimitive from '@radix-ui/react-hover-card';

import { cn } from '../utils/cn';
import { useZIndex } from '../utils/useZIndex';
import { ZIndexProvider } from '../utils/ZIndexContext';
import zIndices from '../utils/zIndices';
import { popoverSurfaceClassName } from './Popover/constants';

export const HoverCard = HoverCardPrimitive.Root;

export const HoverCardTrigger = HoverCardPrimitive.Trigger;

export const HoverCardContent = forwardRef<
  ElementRef<typeof HoverCardPrimitive.Content>,
  ComponentPropsWithoutRef<typeof HoverCardPrimitive.Content>
>(
  (
    { className, align = 'center', sideOffset = 4, children, ...props },
    ref
  ) => {
    const zIndex = useZIndex(zIndices.popover);

    return (
      <HoverCardPrimitive.Portal>
        <HoverCardPrimitive.Content
          ref={ref}
          align={align}
          sideOffset={sideOffset}
          className={cn(popoverSurfaceClassName, className)}
          {...props}
          style={{ zIndex, ...props.style }}
        >
          <ZIndexProvider value={zIndex}>{children}</ZIndexProvider>
        </HoverCardPrimitive.Content>
      </HoverCardPrimitive.Portal>
    );
  }
);
HoverCardContent.displayName = HoverCardPrimitive.Content.displayName;
