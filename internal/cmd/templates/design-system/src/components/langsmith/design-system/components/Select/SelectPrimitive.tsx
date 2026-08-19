import * as React from 'react';

import {
  CheckIcon,
  ChevronDownIcon,
  ChevronUpIcon,
} from '@langchain/untitled-ui-icons';
import * as RadixSelect from '@radix-ui/react-select';
import { useControllableState } from '@radix-ui/react-use-controllable-state';

import { cn } from '../../utils/cn';
import { useZIndex } from '../../utils/useZIndex';
import { ZIndexProvider } from '../../utils/ZIndexContext';
import zIndices from '../../utils/zIndices';

export const SelectRoot: typeof RadixSelect.Root = (props) => {
  const [open = false, onOpenChange] = useControllableState({
    prop: props.open,
    defaultProp: props.defaultOpen ?? false,
    onChange: props.onOpenChange,
  });

  return (
    <RadixSelect.Root
      {...props}
      open={open}
      onOpenChange={onOpenChange}
      defaultOpen={undefined}
    />
  );
};

export type SelectTriggerProps = React.ComponentPropsWithoutRef<
  typeof RadixSelect.Trigger
> & {
  iconClassName?: string;
};

export const SelectTrigger = React.forwardRef<
  React.ElementRef<typeof RadixSelect.Trigger>,
  SelectTriggerProps
>(({ className, children, iconClassName, ...props }, ref) => {
  if (props.asChild) {
    return (
      <RadixSelect.Trigger
        ref={ref}
        className={cn(
          'flex items-center justify-between gap-space-2 rounded-md border border-default bg-transparent p-1.5 px-2.5 text-sm focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 data-[placeholder]:text-secondary [&>span]:line-clamp-1',
          className
        )}
        {...props}
      >
        {props.asChild ? children : <span>{children}</span>}
      </RadixSelect.Trigger>
    );
  }

  return (
    <RadixSelect.Trigger
      ref={ref}
      className={cn(
        'flex items-center justify-between gap-space-2 rounded-md border border-default bg-transparent p-1.5 px-2.5 text-sm focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 data-[placeholder]:text-secondary [&>span]:line-clamp-1',
        className
      )}
      {...props}
    >
      {children}
      <RadixSelect.Icon asChild>
        <ChevronDownIcon
          className={cn('h-4 w-4 text-icon-primary', iconClassName)}
        />
      </RadixSelect.Icon>
    </RadixSelect.Trigger>
  );
});
SelectTrigger.displayName = RadixSelect.Trigger.displayName;

const SelectScrollUpButton = React.forwardRef<
  React.ElementRef<typeof RadixSelect.ScrollUpButton>,
  React.ComponentPropsWithoutRef<typeof RadixSelect.ScrollUpButton>
>(({ className, ...props }, ref) => (
  <RadixSelect.ScrollUpButton
    ref={ref}
    className={cn(
      'flex cursor-default items-center justify-center py-space-1',
      className
    )}
    {...props}
  >
    <ChevronUpIcon className="size-4" />
  </RadixSelect.ScrollUpButton>
));
SelectScrollUpButton.displayName = RadixSelect.ScrollUpButton.displayName;

const SelectScrollDownButton = React.forwardRef<
  React.ElementRef<typeof RadixSelect.ScrollDownButton>,
  React.ComponentPropsWithoutRef<typeof RadixSelect.ScrollDownButton>
>(({ className, ...props }, ref) => (
  <RadixSelect.ScrollDownButton
    ref={ref}
    className={cn(
      'flex cursor-default items-center justify-center py-space-1',
      className
    )}
    {...props}
  >
    <ChevronDownIcon className="size-4" />
  </RadixSelect.ScrollDownButton>
));
SelectScrollDownButton.displayName = RadixSelect.ScrollDownButton.displayName;

export const SelectContent = React.forwardRef<
  React.ElementRef<typeof RadixSelect.Content>,
  React.ComponentPropsWithoutRef<typeof RadixSelect.Content>
>(({ className, children, position = 'popper', ...props }, ref) => {
  const zIndex = useZIndex(zIndices.popover);

  return (
    <RadixSelect.Portal>
      <RadixSelect.Content
        ref={ref}
        className={cn(
          'relative max-h-96 min-w-[8rem] overflow-hidden rounded-md border border-default bg-elevated shadow-md',
          position === 'popper' &&
            'data-[side=bottom]:translate-y-1 data-[side=left]:-translate-x-1 data-[side=right]:translate-x-1 data-[side=top]:-translate-y-1',
          className
        )}
        position={position}
        {...props}
        style={{ zIndex, ...props.style }}
      >
        <ZIndexProvider value={zIndex}>
          <SelectScrollUpButton />
          <RadixSelect.Viewport
            className={cn(
              'p-space-1',
              position === 'popper' &&
                'h-[var(--radix-select-trigger-height)] w-full min-w-[var(--radix-select-trigger-width)]'
            )}
          >
            {children}
          </RadixSelect.Viewport>
          <SelectScrollDownButton />
        </ZIndexProvider>
      </RadixSelect.Content>
    </RadixSelect.Portal>
  );
});
SelectContent.displayName = RadixSelect.Content.displayName;

export const SelectLabel = React.forwardRef<
  React.ElementRef<typeof RadixSelect.Label>,
  React.ComponentPropsWithoutRef<typeof RadixSelect.Label>
>(({ className, ...props }, ref) => (
  <RadixSelect.Label
    ref={ref}
    className={cn(
      'py-1.5 pl-space-6 pr-space-2 text-sm font-semibold',
      className
    )}
    {...props}
  />
));
SelectLabel.displayName = RadixSelect.Label.displayName;

export const SelectItem = React.forwardRef<
  React.ElementRef<typeof RadixSelect.Item>,
  React.ComponentPropsWithoutRef<typeof RadixSelect.Item> & {
    checkmarkPosition?: 'left' | 'right';
    selectedIndicator?: 'checkmark' | 'background';
  }
>(
  (
    {
      className,
      children,
      checkmarkPosition = 'left',
      selectedIndicator = 'checkmark',
      ...props
    },
    ref
  ) => (
    <RadixSelect.Item
      ref={ref}
      className={cn(
        'relative flex w-full cursor-default select-none items-center rounded-sm py-1.5 pr-space-2 text-sm outline-none focus:bg-elevated-hover data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
        selectedIndicator === 'background'
          ? 'pl-space-2 data-[state=checked]:bg-selected data-[state=checked]:focus:bg-selected-hover'
          : checkmarkPosition === 'left'
            ? 'pl-space-6'
            : 'pl-space-2',
        className
      )}
      {...props}
    >
      {selectedIndicator === 'checkmark' && (
        <span
          className={cn(
            'absolute flex h-3.5 w-3.5 items-center justify-center',
            checkmarkPosition === 'left' ? 'left-2' : 'right-2'
          )}
        >
          <RadixSelect.ItemIndicator>
            <CheckIcon className="size-4" />
          </RadixSelect.ItemIndicator>
        </span>
      )}

      <RadixSelect.ItemText>{children}</RadixSelect.ItemText>
    </RadixSelect.Item>
  )
);
SelectItem.displayName = RadixSelect.Item.displayName;

export const SelectSeparator = React.forwardRef<
  React.ElementRef<typeof RadixSelect.Separator>,
  React.ComponentPropsWithoutRef<typeof RadixSelect.Separator>
>(({ className, ...props }, ref) => (
  <RadixSelect.Separator
    ref={ref}
    className={cn(
      '-mx-space-1 my-space-1 h-px border-b border-subtle',
      className
    )}
    {...props}
  />
));
SelectSeparator.displayName = RadixSelect.Separator.displayName;

export const SelectValue = RadixSelect.Value;
