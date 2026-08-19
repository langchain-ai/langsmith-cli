import * as React from 'react';

import * as RadioGroupPrimitive from '@radix-ui/react-radio-group';

import { cn } from '../../utils/cn';

export type RadioGroupItemProps = React.ComponentPropsWithoutRef<
  typeof RadioGroupPrimitive.Item
>;

const RadioGroupItem = React.forwardRef<
  React.ElementRef<typeof RadioGroupPrimitive.Item>,
  RadioGroupItemProps
>(({ className, ...props }, ref) => {
  return (
    <RadioGroupPrimitive.Item
      ref={ref}
      className={cn(
        'flex items-start justify-end gap-space-3 self-stretch rounded-full border border-subtle bg-surface-level-1 p-space-3 transition-colors data-[disabled]:cursor-not-allowed data-[disabled]:border-disabled data-[disabled]:bg-disabled data-[disabled]:data-[state=checked]:bg-disabled data-[state=checked]:bg-brand data-[state=checked]:p-space-2',
        className
      )}
      {...props}
    >
      <RadioGroupPrimitive.Indicator className="group flex items-center justify-center">
        <div className="size-2 rounded-full bg-control-thumb group-data-[disabled]:bg-control-disabled" />
      </RadioGroupPrimitive.Indicator>
    </RadioGroupPrimitive.Item>
  );
});
RadioGroupItem.displayName = RadioGroupPrimitive.Item.displayName;

export { RadioGroupItem };
