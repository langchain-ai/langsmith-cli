import type { CSSProperties, ReactNode } from 'react';
import { createContext, useContext, useState } from 'react';

import { ChevronRightDoubleIcon } from '@langchain/untitled-ui-icons';
import * as RadixDialog from '@radix-ui/react-dialog';

import { cn } from '../../utils/cn';
import { mergeRefs } from '../../utils/merge-refs';
import { useZIndex } from '../../utils/useZIndex';
import { ScrollParentProvider } from '../../utils/VirtuosoCustomScrollParentContext';
import {
  ScrollParentContext,
  useScrollParent,
} from '../../utils/VirtuosoCustomScrollParentContext/useScrollParent';
import { ZIndexProvider } from '../../utils/ZIndexContext';
import zIndices from '../../utils/zIndices';
import { DialogContainerContext } from '../Dialog';
import { IconButton } from '../IconButton';
import { Text } from '../Text';
import UnsavedChangesDialog from '../UnsavedChangesDialog';
import { TopBarPaneSlot } from './Pane.utils';

const PaneDepth = createContext<number>(1);

interface CorePaneProps {
  open: boolean;
  animation?: boolean;
  onClose: () => void;
  children: ReactNode;
  dialogStyle?: CSSProperties;
  transparentBackdrop?: boolean;
  handleScroll?: (e: Event) => void;
  preventInteractOutside?: boolean;
  requireConfirmationOnClose?: boolean;
  zIndex?: number;
}

export interface PaneProps extends CorePaneProps {
  title: string;
  description?: ReactNode;
  noBackArrow?: boolean;
  className?: string;
  headerClassName?: string;
  titleClassName?: string;
  descriptionClassName?: string;
  topBarRightElement?: ReactNode;
  onBack?: () => void;
  scrollRef?: React.RefObject<HTMLDivElement | null>;
}

function PlainPane({
  open,
  animation,
  onClose,
  children,
  dialogStyle,
  transparentBackdrop,
  handleScroll,
  preventInteractOutside,
  requireConfirmationOnClose,
  zIndex,
}: CorePaneProps) {
  const depth = useContext(PaneDepth);
  const scrollParentRef = useScrollParent();
  const [showConfirmModal, setShowConfirmModal] = useState(false);
  const [contentNode, setContentNode] = useState<HTMLDivElement | null>(null);
  const paneZIndex = useZIndex(zIndex ?? zIndices.pane);

  const handleRequestClose = () => {
    if (requireConfirmationOnClose) {
      setShowConfirmModal(true);
    } else {
      onClose();
    }
  };

  return (
    <>
      <RadixDialog.Root
        open={open}
        onOpenChange={(o) => {
          if (!o) handleRequestClose();
        }}
      >
        <RadixDialog.Portal>
          <RadixDialog.Overlay
            className={cn(
              'fixed inset-0 bg-overlay',
              animation &&
                'data-[state=open]:animate-overlay-in data-[state=closed]:animate-overlay-out motion-reduce:animate-none',
              transparentBackdrop ? 'bg-transparent' : ''
            )}
            style={{
              zIndex: paneZIndex,
            }}
          >
            <RadixDialog.Content
              ref={mergeRefs([scrollParentRef, setContentNode])}
              className={cn(
                'relative flex h-[100vh] flex-col items-stretch overflow-y-auto bg-elevated shadow-lg',
                'focus:outline-none focus-visible:outline-none',
                animation &&
                  'data-[state=open]:animate-slide-left data-[state=closed]:animate-slide-right motion-reduce:animate-none'
              )}
              style={{
                marginLeft: `${depth * 5.0}rem`,
                marginRight: `var(--polly-chat-width, 0px)`,
                ...dialogStyle,
              }}
              onScroll={(e) => handleScroll?.(e.nativeEvent)}
              onOpenAutoFocus={(e) => {
                e.preventDefault();
                const target = e.currentTarget as HTMLDivElement;
                if (target) {
                  target.focus();
                }
              }}
              onEscapeKeyDown={
                requireConfirmationOnClose
                  ? (e) => {
                      e.preventDefault();
                      setShowConfirmModal(true);
                    }
                  : undefined
              }
              onPointerDownOutside={
                preventInteractOutside
                  ? (e) => {
                      e.preventDefault();
                    }
                  : undefined
              }
              onInteractOutside={
                preventInteractOutside
                  ? (e) => {
                      e.preventDefault();
                    }
                  : undefined
              }
            >
              <ZIndexProvider value={paneZIndex}>
                <DialogContainerContext.Provider value={contentNode}>
                  <TopBarPaneSlot.Context>
                    <PaneDepth.Provider value={depth + 1}>
                      {children}
                    </PaneDepth.Provider>
                  </TopBarPaneSlot.Context>
                </DialogContainerContext.Provider>
              </ZIndexProvider>
            </RadixDialog.Content>
          </RadixDialog.Overlay>
        </RadixDialog.Portal>
      </RadixDialog.Root>
      <UnsavedChangesDialog
        isOpen={showConfirmModal}
        onClose={() => setShowConfirmModal(false)}
        onDiscard={() => {
          setShowConfirmModal(false);
          onClose();
        }}
        title="Unsaved Changes"
        description="You may have unsaved changes. Are you sure you want to exit?"
        discardCopy="Exit"
      />
    </>
  );
}

export function Pane({
  open,
  animation = true,
  onClose,
  onBack,
  title,
  description,
  children,
  className,
  noBackArrow,
  headerClassName,
  titleClassName,
  descriptionClassName,
  topBarRightElement,
  dialogStyle,
  transparentBackdrop,
  scrollRef,
  handleScroll,
  preventInteractOutside,
  requireConfirmationOnClose,
  zIndex,
}: PaneProps) {
  const [showConfirmModal, setShowConfirmModal] = useState(false);

  const handleCloseClick = () => {
    if (onBack) {
      onBack();
    } else if (requireConfirmationOnClose) {
      setShowConfirmModal(true);
    } else {
      onClose();
    }
  };

  const paneComponent = (
    <>
      <PlainPane
        open={open}
        animation={animation}
        onClose={onClose}
        dialogStyle={dialogStyle}
        transparentBackdrop={transparentBackdrop}
        handleScroll={handleScroll}
        preventInteractOutside={preventInteractOutside}
        requireConfirmationOnClose={requireConfirmationOnClose}
        zIndex={zIndex}
      >
        <header
          className={cn(
            'sticky top-0 flex items-center gap-space-1 border-b border-b-subtle bg-elevated py-1.5 pl-[10px] pr-space-4',
            headerClassName
          )}
          data-testid={`pane-header-${title}`}
          style={{ zIndex: zIndices.paneHeader }}
        >
          {!noBackArrow && (
            <IconButton
              label="Close"
              icon={ChevronRightDoubleIcon}
              onClick={handleCloseClick}
              variant="plain"
              size="xs"
              iconClassName="size-5 text-quaternary"
            />
          )}
          <div className="flex min-w-0 flex-col gap-space-1">
            <RadixDialog.Title asChild>
              <Text
                as="h2"
                variant="h3"
                color="secondary"
                className={cn('min-w-0', titleClassName)}
              >
                {title}
              </Text>
            </RadixDialog.Title>
            {description && (
              <RadixDialog.Description asChild>
                <Text
                  as="p"
                  variant="sm"
                  color="tertiary"
                  className={descriptionClassName}
                >
                  {description}
                </Text>
              </RadixDialog.Description>
            )}
          </div>
          <div className="flex min-w-0 flex-1 items-center justify-end gap-space-2">
            {topBarRightElement}
            <TopBarPaneSlot.Slot className="flex items-center gap-space-2 empty:hidden" />
          </div>
        </header>
        <div
          className={cn(
            'flex-1 pb-space-8 pl-space-7 pr-space-4 pt-space-4',
            className
          )}
        >
          {children}
        </div>
      </PlainPane>
      {!onBack && (
        <UnsavedChangesDialog
          isOpen={showConfirmModal}
          onClose={() => setShowConfirmModal(false)}
          onDiscard={() => {
            setShowConfirmModal(false);
            onClose();
          }}
          title="Unsaved Changes"
          description="You may have unsaved changes. Are you sure you want to exit?"
          discardCopy="Exit"
        />
      )}
    </>
  );
  return scrollRef ? (
    <ScrollParentContext.Provider value={scrollRef}>
      {paneComponent}
    </ScrollParentContext.Provider>
  ) : (
    <ScrollParentProvider>{paneComponent}</ScrollParentProvider>
  );
}
