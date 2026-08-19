import { useId } from 'react';

import * as SwitchPrimitive from '@radix-ui/react-switch';

import { cn } from '../../utils/cn';
import { Text } from '../Text';

interface SwitchBaseProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  id?: string;
  className?: string;
  size?: 'xs' | 'sm' | 'md';
  disabled?: boolean;
  onClick?: (e: React.MouseEvent) => void;
}

interface SwitchWithLabelProps extends SwitchBaseProps {
  label: string;
  labelClassName?: string;
  labelPosition?: 'left' | 'right';
  'aria-label'?: never;
  'aria-labelledby'?: never;
}

interface SwitchWithAriaLabelProps extends SwitchBaseProps {
  'aria-label': string;
  label?: never;
  labelClassName?: never;
  labelPosition?: never;
  'aria-labelledby'?: never;
}

interface SwitchWithAriaLabelledByProps extends SwitchBaseProps {
  'aria-labelledby': string;
  label?: never;
  labelClassName?: never;
  labelPosition?: never;
  'aria-label'?: never;
}

export type SwitchProps =
  | SwitchWithLabelProps
  | SwitchWithAriaLabelProps
  | SwitchWithAriaLabelledByProps;

const ROOT_SIZE_CLASSES: Record<
  NonNullable<SwitchBaseProps['size']>,
  string
> = {
  xs: 'h-3 w-6',
  sm: 'h-4 w-8',
  md: 'h-5 w-10',
};

const THUMB_SIZE_CLASSES: Record<
  NonNullable<SwitchBaseProps['size']>,
  string
> = {
  xs: 'size-2 data-[state=checked]:translate-x-[14px]',
  sm: 'size-3 data-[state=checked]:translate-x-[18px]',
  md: 'size-4 data-[state=checked]:translate-x-[22px]',
};

export const Switch = ({
  checked,
  onChange,
  id,
  className,
  size,
  disabled,
  onClick,
  label,
  labelClassName,
  labelPosition = 'right',
  ...rest
}: SwitchProps) => {
  const generatedId = useId();
  const switchId = id ?? generatedId;

  return (
    <div className={cn('flex items-center gap-space-2', className)}>
      {labelPosition === 'left' && label && (
        <label htmlFor={switchId}>
          <Text as="span" variant={size ?? 'md'} className={labelClassName}>
            {label}
          </Text>
        </label>
      )}

      <SwitchPrimitive.Root
        className={cn(
          'inline-flex items-center rounded-full bg-surface-level-4 transition',
          disabled
            ? 'cursor-not-allowed data-[state=checked]:bg-brand-subtle'
            : 'data-[state=checked]:bg-control-active hover:data-[state=checked]:bg-control-active-hover',
          ROOT_SIZE_CLASSES[size ?? 'md']
        )}
        id={switchId}
        checked={checked}
        onCheckedChange={onChange}
        disabled={disabled}
        onClick={onClick}
        {...rest}
      >
        <SwitchPrimitive.Thumb
          className={cn(
            'translate-x-[2px] rounded-full bg-control-thumb transition',
            THUMB_SIZE_CLASSES[size ?? 'md'],
            disabled &&
              'bg-disabled data-[state=checked]:bg-control-thumb dark:data-[state=checked]:bg-disabled'
          )}
        />
      </SwitchPrimitive.Root>

      {labelPosition === 'right' && label && (
        <label htmlFor={switchId}>
          <Text as="span" variant={size ?? 'md'} className={labelClassName}>
            {label}
          </Text>
        </label>
      )}
    </div>
  );
};
