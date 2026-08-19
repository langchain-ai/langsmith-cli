import type { ComponentPropsWithoutRef, ElementRef } from 'react';
import { forwardRef } from 'react';

import * as PopoverPrimitive from '@radix-ui/react-popover';

import { cn } from '../../utils/cn';
import { useZIndex } from '../../utils/useZIndex';
import { ZIndexProvider } from '../../utils/ZIndexContext';
import zIndices from '../../utils/zIndices';
import { useDialogContainer } from '../Dialog';
import { popoverSurfaceClassName } from './constants';

export const Popover = PopoverPrimitive.Root;

export const PopoverTrigger = PopoverPrimitive.Trigger;
export const PopoverAnchor = PopoverPrimitive.Anchor;
export const PopoverArrow = forwardRef<
  ElementRef<typeof PopoverPrimitive.Arrow>,
  ComponentPropsWithoutRef<typeof PopoverPrimitive.Arrow>
>(({ className, ...props }, ref) => (
  <PopoverPrimitive.Arrow
    ref={ref}
    className={cn('fill-popover', className)}
    {...props}
  />
));
PopoverArrow.displayName = 'PopoverArrow';

export const PopoverContent = forwardRef<
  ElementRef<typeof PopoverPrimitive.Content>,
  ComponentPropsWithoutRef<typeof PopoverPrimitive.Content> & {
    /**
     * Portal target for the popover. When omitted, the popover automatically
     * portals into the enclosing modal `Dialog` (if any) so it stays
     * mouse-interactive, and falls back to `document.body` otherwise.
     */
    container?: HTMLElement | null;
  }
>(
  (
    {
      className,
      align = 'center',
      sideOffset = 4,
      forceMount,
      container,
      children,
      ...props
    },
    ref
  ) => {
    const zIndex = useZIndex(zIndices.popover);
    // Portal into the enclosing modal Dialog (null when not inside one) so the
    // popover stays inside the dialog's pointer-events-interactive region.
    const dialogContainer = useDialogContainer();

    return (
      <PopoverPrimitive.Portal
        forceMount={forceMount}
        // Fall back to `undefined` (not `null`) when there is no dialog: Radix's
        // Portal only defaults to `document.body` when `container` is absent, so
        // an explicit `null` leaves non-forceMount popovers unrendered.
        container={container ?? dialogContainer ?? undefined}
      >
        <PopoverPrimitive.Content
          ref={ref}
          align={align}
          sideOffset={sideOffset}
          forceMount={forceMount}
          className={cn(popoverSurfaceClassName, className)}
          {...props}
          style={{ zIndex, ...props.style }}
        >
          <ZIndexProvider value={zIndex}>{children}</ZIndexProvider>
        </PopoverPrimitive.Content>
      </PopoverPrimitive.Portal>
    );
  }
);
PopoverContent.displayName = PopoverPrimitive.Content.displayName;
