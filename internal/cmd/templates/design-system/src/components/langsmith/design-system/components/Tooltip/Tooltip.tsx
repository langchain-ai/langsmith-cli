import type { ReactNode } from 'react';

import * as TooltipPrimitive from '@radix-ui/react-tooltip';

import { cn } from '../../utils/cn';
import { useZIndex } from '../../utils/useZIndex';
import { ZIndexProvider } from '../../utils/ZIndexContext';
import zIndices from '../../utils/zIndices';
import { Text } from '../Text';

export type TooltipProps = {
  title?: string | ReactNode;
  description?: ReactNode;
  children: ReactNode;
  tooltipClassName?: string;
  side?: 'top' | 'bottom' | 'left' | 'right';
  sideOffset?: number;
  align?: 'start' | 'center' | 'end';
  alignOffset?: number;
  invert?: boolean;
  delayDuration?: number;
  disabled?: boolean;
  blockTriggerEvents?: boolean;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  disableHoverableContent?: boolean;
};

// Tooltip sets two opinionated defaults — override with caution:
// - delayDuration=300: prevents tooltips from blocking clicks
// - disableHoverableContent=true: dismisses the tooltip when hovering off the trigger
export function Tooltip({
  title,
  description,
  children,
  tooltipClassName,
  side = 'top',
  sideOffset = 10,
  align = 'center',
  alignOffset = 0,
  invert = false,
  delayDuration = 300,
  disabled = false,
  blockTriggerEvents = false,
  open,
  onOpenChange,
  disableHoverableContent = true,
}: TooltipProps) {
  // Tooltips have no fixed z-index floor — they always render one level above
  // their parent caller so they naturally stack above whichever layer triggered them.
  const zIndex = useZIndex(zIndices.tooltip);

  if ((!title && !description) || disabled) {
    return children;
  }

  return (
    <TooltipPrimitive.Root
      delayDuration={delayDuration}
      open={open}
      onOpenChange={onOpenChange}
      disableHoverableContent={disableHoverableContent}
    >
      <TooltipPrimitive.Trigger
        asChild
        onPointerDown={(e) => {
          if (blockTriggerEvents) {
            e.preventDefault();
            e.stopPropagation();
          }
        }}
      >
        {children}
      </TooltipPrimitive.Trigger>
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Content
          sideOffset={sideOffset}
          side={side}
          align={align}
          alignOffset={alignOffset}
          collisionPadding={8}
          className={cn(
            'max-w-[320px] overflow-hidden rounded-md border border-secondary bg-background p-space-2 text-sm shadow-md dark:bg-secondary',
            'animate-in fade-in-0 zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2',
            tooltipClassName,
            invert && 'invert'
          )}
          style={{ zIndex }}
        >
          <ZIndexProvider value={zIndex}>
            <div className="flex flex-col gap-space-2 text-xxs">
              {typeof title === 'string' ? (
                <Text
                  variant="sm"
                  data-target="title"
                  className={!tooltipClassName ? 'text-primary' : ''}
                >
                  {title}
                </Text>
              ) : (
                title
              )}
              {description && (
                <Text
                  variant="sm"
                  className={!tooltipClassName ? 'text-secondary' : ''}
                  data-target="description"
                >
                  {description}
                </Text>
              )}
            </div>
          </ZIndexProvider>
        </TooltipPrimitive.Content>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}
