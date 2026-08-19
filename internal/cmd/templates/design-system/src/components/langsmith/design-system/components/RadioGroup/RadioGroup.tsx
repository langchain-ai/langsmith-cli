import * as React from 'react';

import * as RadioGroupPrimitive from '@radix-ui/react-radio-group';

import { cn } from '../../utils/cn';

export type RadioGroupProps = React.ComponentPropsWithoutRef<
  typeof RadioGroupPrimitive.Root
>;

const RadioGroup = React.forwardRef<
  React.ElementRef<typeof RadioGroupPrimitive.Root>,
  RadioGroupProps
>(({ className, ...props }, ref) => {
  return (
    <RadioGroupPrimitive.Root
      className={cn('flex flex-col gap-space-3', className)}
      {...props}
      ref={ref}
    />
  );
});
RadioGroup.displayName = RadioGroupPrimitive.Root.displayName;

export { RadioGroup };
