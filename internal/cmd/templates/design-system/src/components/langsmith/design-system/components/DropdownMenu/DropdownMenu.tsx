import * as React from 'react';

import * as DropdownMenuPrimitive from '@radix-ui/react-dropdown-menu';
import { useControllableState } from '@radix-ui/react-use-controllable-state';

import { cn } from '../../utils/cn';
import { useZIndex } from '../../utils/useZIndex';
import { ZIndexProvider } from '../../utils/ZIndexContext';
import zIndices from '../../utils/zIndices';

export const DropdownMenu: typeof DropdownMenuPrimitive.Root = (props) => {
  const [open = false, onOpenChange] = useControllableState({
    prop: props.open,
    defaultProp: props.defaultOpen ?? false,
    onChange: props.onOpenChange,
  });

  return (
    <DropdownMenuPrimitive.Root
      {...props}
      open={open}
      onOpenChange={onOpenChange}
      defaultOpen={undefined}
    />
  );
};

export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;

export const DropdownMenuPortal = DropdownMenuPrimitive.Portal;

export const DropdownMenuGroup = DropdownMenuPrimitive.Group;

export const DropdownMenuLabel = DropdownMenuPrimitive.Label;

/**
 * Stops clicks outside an open dropdown from reaching elements underneath it
 * (e.g. a table row's `onClick`) without covering the viewport, so scrolling
 * the content behind the menu keeps working natively.
 *
 * React delegates `onClick` at the root in the bubble phase, so a capture-phase
 * `click` listener on `document` runs first and can stop the event before it
 * gets there. pointerdown is left alone so Radix's outside-dismiss keeps
 * working; the wheel listener is passive so native scroll is never interrupted,
 * it just also dismisses the menu (an open menu jitters as Radix repositions it
 * to follow its trigger while scrolling).
 *
 * "Inside" is any element with `role="menu"` in its ancestry — Radix gives that
 * role to both the content and its submenus (which portal out separately), so
 * clicks and scrolls inside a submenu are correctly treated as inside.
 */
const OutsideClickGuard = () => {
  const cleanupTimerRef =
    React.useRef<ReturnType<typeof setTimeout>>(undefined);
  const pointerDownOutsideRef = React.useRef(false);

  React.useLayoutEffect(() => {
    clearTimeout(cleanupTimerRef.current);
    pointerDownOutsideRef.current = false;

    const isOutside = (target: EventTarget | null) =>
      !(target instanceof Element) || !target.closest('[role="menu"]');

    // Observe (don't block) outside pointerdowns: Radix dismisses on them, so
    // this tells us the trailing click needs absorbing on cleanup.
    const onPointerDown = (event: PointerEvent) => {
      if (isOutside(event.target)) pointerDownOutsideRef.current = true;
    };
    document.addEventListener('pointerdown', onPointerDown, true);

    // Block clicks that land outside the menu from reaching the content beneath.
    const onClick = (event: MouseEvent) => {
      if (isOutside(event.target)) {
        event.preventDefault();
        event.stopPropagation();
      }
    };
    document.addEventListener('click', onClick, true);

    // Dismiss on scroll of the content behind the menu: an anchored menu that
    // stays open jitters as Radix repositions it to chase its trigger. Passive,
    // so the wheel still scrolls the content natively on the same gesture.
    // Scrolling inside the menu (e.g. a details panel) leaves it open.
    const onWheel = (event: WheelEvent) => {
      if (isOutside(event.target)) {
        document.dispatchEvent(
          new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
        );
      }
    };
    document.addEventListener('wheel', onWheel, { passive: true });

    return () => {
      document.removeEventListener('pointerdown', onPointerDown, true);
      document.removeEventListener('click', onClick, true);
      document.removeEventListener('wheel', onWheel);

      // Radix dismisses on the outside pointerdown and unmounts this guard
      // before the trailing click fires, so absorb that one click here. Only
      // for pointer dismissals — Escape / item selection have no trailing
      // outside click, and absorbing an unrelated later click would be wrong.
      if (!pointerDownOutsideRef.current) return;

      const absorb = (event: Event) => {
        event.preventDefault();
        event.stopPropagation();
      };
      const opts = { capture: true, once: true } as const;
      document.addEventListener('pointerup', absorb, opts);
      document.addEventListener('mouseup', absorb, opts);
      document.addEventListener('click', absorb, opts);

      cleanupTimerRef.current = setTimeout(() => {
        if (typeof document === 'undefined') return;
        document.removeEventListener('pointerup', absorb, { capture: true });
        document.removeEventListener('mouseup', absorb, { capture: true });
        document.removeEventListener('click', absorb, { capture: true });
      }, 300);
    };
  }, []);

  return null;
};

export const DropdownMenuContent = React.forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Content> & {
    /**
     * When true, stops clicks outside the open menu from reaching elements
     * underneath (e.g. table rows), while leaving scrolling of that content
     * intact. Radix's outside-click dismiss still works normally.
     */
    preventClickThrough?: boolean;
  }
>(
  (
    { className, sideOffset = 4, children, preventClickThrough, ...props },
    ref
  ) => {
    const zIndex = useZIndex(zIndices.popover);

    return (
      <DropdownMenuPrimitive.Portal>
        <DropdownMenuPrimitive.Content
          ref={ref}
          sideOffset={sideOffset}
          className={cn(
            'min-w-[8rem] overflow-hidden rounded-md border border-subtle bg-popover p-space-1 shadow-lg',
            className
          )}
          {...props}
          style={{ zIndex, ...props.style }}
        >
          {preventClickThrough && <OutsideClickGuard />}
          <ZIndexProvider value={zIndex}>{children}</ZIndexProvider>
        </DropdownMenuPrimitive.Content>
      </DropdownMenuPrimitive.Portal>
    );
  }
);
DropdownMenuContent.displayName = DropdownMenuPrimitive.Content.displayName;

export const DropdownMenuItem = React.forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.Item>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Item> & {
    inset?: boolean;
  }
>(({ className, inset, ...props }, ref) => (
  <DropdownMenuPrimitive.Item
    ref={ref}
    className={cn(
      'text-md flex cursor-pointer select-none items-center rounded-sm px-space-2 py-1.5 outline-none transition-colors focus:bg-surface-level-2 data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
      inset && 'pl-space-6',
      className
    )}
    {...props}
  />
));
DropdownMenuItem.displayName = DropdownMenuPrimitive.Item.displayName;

export const DropdownMenuSeparator = React.forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.Separator>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Separator>
>(({ className, ...props }, ref) => (
  <DropdownMenuPrimitive.Separator
    ref={ref}
    className={cn('my-space-1 h-px bg-surface-level-2', className)}
    {...props}
  />
));
DropdownMenuSeparator.displayName = DropdownMenuPrimitive.Separator.displayName;

export const DropdownMenuSub = DropdownMenuPrimitive.Sub;

export const DropdownMenuSubTrigger = React.forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.SubTrigger>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.SubTrigger> & {
    inset?: boolean;
  }
>(({ className, inset, children, ...props }, ref) => (
  <DropdownMenuPrimitive.SubTrigger
    ref={ref}
    className={cn(
      'text-md flex cursor-pointer select-none items-center rounded-sm px-space-2 py-1.5 outline-none transition-colors focus:bg-surface-level-2 data-[state=open]:bg-surface-level-2',
      inset && 'pl-space-6',
      className
    )}
    {...props}
  >
    {children}
  </DropdownMenuPrimitive.SubTrigger>
));
DropdownMenuSubTrigger.displayName =
  DropdownMenuPrimitive.SubTrigger.displayName;

export const DropdownMenuSubContent = React.forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.SubContent>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.SubContent>
>(({ className, children, ...props }, ref) => {
  const zIndex = useZIndex(zIndices.popover);

  return (
    <DropdownMenuPrimitive.Portal>
      <DropdownMenuPrimitive.SubContent
        ref={ref}
        className={cn(
          'min-w-[8rem] overflow-hidden rounded-md border border-subtle bg-popover p-space-1 shadow-lg',
          className
        )}
        style={{ zIndex }}
        {...props}
      >
        <ZIndexProvider value={zIndex}>{children}</ZIndexProvider>
      </DropdownMenuPrimitive.SubContent>
    </DropdownMenuPrimitive.Portal>
  );
});
DropdownMenuSubContent.displayName =
  DropdownMenuPrimitive.SubContent.displayName;
