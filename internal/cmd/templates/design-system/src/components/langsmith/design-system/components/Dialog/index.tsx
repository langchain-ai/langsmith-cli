import {
  type KeyboardEvent as ReactKeyboardEvent,
  type ReactNode,
  createContext,
  forwardRef,
  useContext,
  useState,
} from 'react';

import { XIcon } from '@langchain/untitled-ui-icons';
import {
  DialogClose,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  Dialog as RadixDialog,
  DialogContent as RadixDialogContent,
  type DialogContentProps as RadixDialogContentProps,
} from '@radix-ui/react-dialog';

import { cn } from '../../utils/cn';
import { mergeRefs } from '../../utils/merge-refs';
import { useZIndex } from '../../utils/useZIndex';
import { ZIndexProvider } from '../../utils/ZIndexContext';
import zIndices from '../../utils/zIndices';
import { Icon } from '../Icon';
import type { IconComponent } from '../Icon';
import { IconButton } from '../IconButton';
import { Text } from '../Text';

/**
 * Exposes the modal DialogContent DOM node to descendants so portaled overlays
 * (e.g. Typeahead's Popover) can render inside the dialog. A modal Dialog sets
 * `pointer-events: none` outside DialogContent, so overlays portaled to
 * `document.body` become mouse-inert; portaling into this node keeps them
 * interactive. Null when not inside a Dialog.
 */
// eslint-disable-next-line react-refresh/only-export-components
export const DialogContainerContext = createContext<HTMLElement | null>(null);

// eslint-disable-next-line react-refresh/only-export-components
export function useDialogContainer(): HTMLElement | null {
  return useContext(DialogContainerContext);
}

type TitleIconIntent = 'error' | 'warning' | 'info';

interface DialogContentProps {
  title?: ReactNode;
  description?: ReactNode;
  /** Icon component to display in the title area, inside a colored circular container */
  titleIcon?: IconComponent;
  /** Controls the background and color of the title icon container */
  titleIconIntent?: TitleIconIntent;
  showClose?: boolean;
  children: ReactNode;
  className?: string;
  childrenClassName?: string;
  onPointerDownOutside?: RadixDialogContentProps['onPointerDownOutside'];
  onInteractOutside?: RadixDialogContentProps['onInteractOutside'];
  onOpenAutoFocus?: RadixDialogContentProps['onOpenAutoFocus'];
  onEscapeKeyDown?: RadixDialogContentProps['onEscapeKeyDown'];
  onKeyDown?: RadixDialogContentProps['onKeyDown'];
}

export function Dialog({
  open,
  onOpenChange,
  children,
  className,
  onEscapeKeyDown,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: ReactNode;
  className?: string;
  onEscapeKeyDown?: (e: ReactKeyboardEvent) => void;
}) {
  const zIndex = useZIndex(zIndices.dialogOverlay);
  return (
    <RadixDialog modal={true} open={open} onOpenChange={onOpenChange}>
      <DialogPortal>
        <DialogOverlay
          className={cn(
            'fixed inset-0 bg-overlay',
            'data-[state=open]:animate-overlay-in data-[state=closed]:animate-overlay-out motion-reduce:animate-none',
            className
          )}
          style={{ zIndex }}
          onKeyDown={(e) => {
            if (e.key === 'Escape' && onEscapeKeyDown) {
              onEscapeKeyDown(e);
            }
          }}
        >
          {children}
        </DialogOverlay>
      </DialogPortal>
    </RadixDialog>
  );
}

export const DialogContent = forwardRef<HTMLDivElement, DialogContentProps>(
  (
    {
      title,
      description,
      titleIcon: TitleIcon,
      titleIconIntent = 'info',
      showClose = true,
      children,
      className,
      childrenClassName,
      onPointerDownOutside,
      onInteractOutside,
      onOpenAutoFocus,
      onEscapeKeyDown,
      onKeyDown,
    },
    ref
  ) => {
    const dialogZIndex = useZIndex(zIndices.dialogContent);
    // A modal Dialog sets `pointer-events: none` outside this node, so overlays
    // portaled to `document.body` (e.g. Typeahead's Popover) become mouse-inert.
    // Expose the content node so descendants can portal into it and stay live.
    const [contentNode, setContentNode] = useState<HTMLDivElement | null>(null);

    return (
      <RadixDialogContent
        ref={mergeRefs([ref, setContentNode])}
        className={cn(
          'fixed left-[50%] top-[50%] flex max-h-[calc(100vh-2rem)] w-[37.5rem] translate-x-[-50%] translate-y-[-50%] flex-col rounded-lg bg-background',
          'fade-in',
          'data-[state=closed]:fade-out',
          className
        )}
        style={{ zIndex: dialogZIndex }}
        onPointerDownOutside={onPointerDownOutside}
        onInteractOutside={onInteractOutside}
        onOpenAutoFocus={onOpenAutoFocus}
        onEscapeKeyDown={onEscapeKeyDown}
        onKeyDown={onKeyDown}
      >
        <ZIndexProvider value={dialogZIndex}>
          <DialogContainerContext.Provider value={contentNode}>
            {(title || showClose) && (
              <DialogTitle className="m-0 min-h-14 shrink-0 p-space-4">
                <div className="flex items-center justify-between gap-space-4">
                  <div className="flex min-w-0 items-center gap-space-2">
                    {TitleIcon && (
                      <Icon
                        icon={TitleIcon}
                        color={titleIconIntent}
                        size="md"
                      />
                    )}
                    <div className="flex min-w-0 flex-col gap-space-1">
                      {title && (
                        <Text variant="h3" as="span" weight="semibold">
                          {title}
                        </Text>
                      )}
                      {description && (
                        <Text variant="sm" className="text-quaternary">
                          {description}
                        </Text>
                      )}
                    </div>
                  </div>
                  {showClose && (
                    <DialogClose asChild>
                      <IconButton
                        icon={XIcon}
                        label="Close"
                        color="secondary"
                        variant="plain"
                        className="shrink-0 self-start"
                        data-testid="dialog-close-button"
                        tooltipProps={{ disabled: true }}
                      />
                    </DialogClose>
                  )}
                </div>
              </DialogTitle>
            )}
            <div
              className={cn(
                'flex min-h-0 flex-col gap-space-4 overflow-y-auto p-space-4',
                Boolean(title || showClose) && 'pt-0',
                childrenClassName
              )}
            >
              {children}
            </div>
          </DialogContainerContext.Provider>
        </ZIndexProvider>
      </RadixDialogContent>
    );
  }
);

DialogContent.displayName = 'DialogContent';
