import * as React from 'react';

import * as ContextMenuPrimitive from '@radix-ui/react-context-menu';

import { cn } from '../../utils/cn';
import { useZIndex } from '../../utils/useZIndex';
import { ZIndexProvider } from '../../utils/ZIndexContext';
import zIndices from '../../utils/zIndices';

export const ContextMenu = ContextMenuPrimitive.Root;

export const ContextMenuTrigger = ContextMenuPrimitive.Trigger;

export const ContextMenuContent = React.forwardRef<
  React.ElementRef<typeof ContextMenuPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof ContextMenuPrimitive.Content>
>(({ className, children, ...props }, ref) => {
  const zIndex = useZIndex(zIndices.popover);

  return (
    <ContextMenuPrimitive.Portal>
      <ContextMenuPrimitive.Content
        ref={ref}
        className={cn(
          'min-w-[8rem] overflow-hidden rounded-md border border-default bg-elevated p-space-1 text-primary shadow-md',
          className
        )}
        {...props}
        style={{ zIndex, ...props.style }}
      >
        <ZIndexProvider value={zIndex}>{children}</ZIndexProvider>
      </ContextMenuPrimitive.Content>
    </ContextMenuPrimitive.Portal>
  );
});
ContextMenuContent.displayName = ContextMenuPrimitive.Content.displayName;

export const ContextMenuItem = React.forwardRef<
  React.ElementRef<typeof ContextMenuPrimitive.Item>,
  React.ComponentPropsWithoutRef<typeof ContextMenuPrimitive.Item> & {
    inset?: boolean;
  }
>(({ className, inset, ...props }, ref) => (
  <ContextMenuPrimitive.Item
    ref={ref}
    className={cn(
      'flex w-full cursor-default select-none items-center rounded-sm px-space-2 py-space-2 text-sm text-primary outline-none transition-colors focus:bg-elevated-hover data-[disabled]:pointer-events-none data-[highlighted]:bg-elevated-hover data-[disabled]:opacity-50',
      inset ? 'pl-space-8' : undefined,
      className
    )}
    {...props}
  />
));
ContextMenuItem.displayName = ContextMenuPrimitive.Item.displayName;

export const ContextMenuSeparator = React.forwardRef<
  React.ElementRef<typeof ContextMenuPrimitive.Separator>,
  React.ComponentPropsWithoutRef<typeof ContextMenuPrimitive.Separator>
>(({ className, ...props }, ref) => (
  <ContextMenuPrimitive.Separator
    ref={ref}
    className={cn('-mx-space-1 my-space-1 border-b border-subtle', className)}
    {...props}
  />
));
ContextMenuSeparator.displayName = ContextMenuPrimitive.Separator.displayName;
