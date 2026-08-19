import type { ComponentPropsWithoutRef, ElementRef, ReactNode } from 'react';
import { forwardRef } from 'react';

import { Command as CommandPrimitive } from 'cmdk';

import { cn } from '../../utils/cn';
import {
  DECORATOR_CLASSES,
  getInputContainerClasses,
  getInputElementClasses,
} from '../Input/inputStyles';
import type { InputSize, InputVariant } from '../Input/inputStyles';

export const Command = forwardRef<
  ElementRef<typeof CommandPrimitive>,
  ComponentPropsWithoutRef<typeof CommandPrimitive>
>(({ className, ...props }, ref) => (
  <CommandPrimitive
    ref={ref}
    className={cn(
      'flex h-full w-full flex-col overflow-hidden rounded-md bg-popover',
      className
    )}
    {...props}
  />
));
Command.displayName = CommandPrimitive.displayName;

export interface CommandInputProps extends Omit<
  ComponentPropsWithoutRef<typeof CommandPrimitive.Input>,
  'size'
> {
  /** Left side decorator (icon or element) */
  leftDecorator?: ReactNode;
  /** Right side decorator (icon or element) */
  rightDecorator?: ReactNode;
  /** Size variant — uses shared Input styling */
  size?: InputSize;
  /** Visual variant — uses shared Input styling */
  variant?: InputVariant;
}

export const CommandInput = forwardRef<
  ElementRef<typeof CommandPrimitive.Input>,
  CommandInputProps
>(
  (
    {
      className,
      leftDecorator,
      rightDecorator,
      size = 'md',
      variant = 'outlined',
      ...props
    },
    ref
  ) => {
    const containerClasses = getInputContainerClasses({
      size,
      variant,
      disabled: props.disabled,
      className,
    });

    const inputClasses = getInputElementClasses({
      size,
      disabled: props.disabled,
    });

    return (
      <div className={containerClasses} {...{ 'cmdk-input-wrapper': '' }}>
        {leftDecorator && (
          <div className={DECORATOR_CLASSES}>{leftDecorator}</div>
        )}
        <CommandPrimitive.Input ref={ref} className={inputClasses} {...props} />
        {rightDecorator && (
          <div className={DECORATOR_CLASSES}>{rightDecorator}</div>
        )}
      </div>
    );
  }
);

CommandInput.displayName = CommandPrimitive.Input.displayName;

export const CommandList = forwardRef<
  ElementRef<typeof CommandPrimitive.List>,
  ComponentPropsWithoutRef<typeof CommandPrimitive.List>
>(({ className, ...props }, ref) => (
  <CommandPrimitive.List
    ref={ref}
    className={cn('max-h-[300px] overflow-y-auto overflow-x-hidden', className)}
    {...props}
  />
));

CommandList.displayName = CommandPrimitive.List.displayName;

export const CommandEmpty = forwardRef<
  ElementRef<typeof CommandPrimitive.Empty>,
  ComponentPropsWithoutRef<typeof CommandPrimitive.Empty>
>((props, ref) => (
  <CommandPrimitive.Empty
    ref={ref}
    className="py-space-5 text-center"
    {...props}
  />
));

CommandEmpty.displayName = CommandPrimitive.Empty.displayName;

export const CommandGroup = forwardRef<
  ElementRef<typeof CommandPrimitive.Group>,
  ComponentPropsWithoutRef<typeof CommandPrimitive.Group>
>(({ className, ...props }, ref) => (
  <CommandPrimitive.Group
    ref={ref}
    className={cn(
      'overflow-hidden',
      '[&_[cmdk-group-heading]]:px-space-2 [&_[cmdk-group-heading]]:py-space-2 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-semibold [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:text-tertiary',
      className
    )}
    {...props}
  />
));

CommandGroup.displayName = CommandPrimitive.Group.displayName;

export const CommandSeparator = forwardRef<
  ElementRef<typeof CommandPrimitive.Separator>,
  ComponentPropsWithoutRef<typeof CommandPrimitive.Separator>
>(({ className, ...props }, ref) => (
  <CommandPrimitive.Separator
    ref={ref}
    className={cn('bg-border -mx-space-1 h-px', className)}
    {...props}
  />
));
CommandSeparator.displayName = CommandPrimitive.Separator.displayName;

export const CommandItem = forwardRef<
  ElementRef<typeof CommandPrimitive.Item>,
  ComponentPropsWithoutRef<typeof CommandPrimitive.Item>
>(({ className, ...props }, ref) => (
  <CommandPrimitive.Item
    ref={ref}
    className={cn(
      'relative flex cursor-pointer select-none items-center rounded-md px-space-2 py-1.5 outline-none hover:bg-secondary aria-disabled:pointer-events-none aria-disabled:opacity-50 aria-selected:bg-secondary',
      className
    )}
    {...props}
  />
));

CommandItem.displayName = CommandPrimitive.Item.displayName;
