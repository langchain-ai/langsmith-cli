import React, { useCallback } from 'react';

import {
  AlertSolidIcon,
  CheckIcon,
  InfoCircleSolidIcon,
  XCloseIcon,
} from '@langchain/untitled-ui-icons';
import * as ToastLib from '@radix-ui/react-toast';

import { cn } from '../../utils/cn';
import { ZIndexProvider } from '../../utils/ZIndexContext';
import zIndices from '../../utils/zIndices';
import { CopyIconButton } from '../CopyButton';
import { Icon } from '../Icon';
import { IconButton } from '../IconButton';
import { ToastContext } from './ToastContext';

const DEFAULT_DURATION_MS = 5000;
// Error toasts do not auto-dismiss; the user must click the close button.
const ERROR_DURATION_MS = Infinity;

// Provider wraps whole application
export function ToastProvider({ children }: { children: React.ReactNode }) {
  // note this only supports one toast at a time currently
  const [open, setOpen] = React.useState(false);
  const [config, setConfig] = React.useState<ToastConfig | null>(null);

  const createToast = useCallback((titleOrConfig: ToastConfig | string) => {
    const preConfig =
      typeof titleOrConfig === 'string'
        ? { title: titleOrConfig }
        : titleOrConfig;
    const isError = preConfig.error || preConfig.type === 'error';
    const config: ToastConfig = {
      duration: isError ? ERROR_DURATION_MS : DEFAULT_DURATION_MS,
      ...preConfig,
    };

    setConfig(config);
    setOpen(true);
  }, []);

  return (
    <ToastContext.Provider value={{ createToast }}>
      <ToastLib.Provider swipeDirection="right">
        {children}
        <ZIndexProvider value={zIndices.toast}>
          <Toast
            title={config?.title}
            description={config?.description}
            open={open}
            setOpen={setOpen}
            action={config?.action}
            error={config?.error}
            duration={config?.duration}
            type={config?.type}
          />
        </ZIndexProvider>
        <ToastLib.Viewport
          className={cn(
            'fixed bottom-0 right-0 m-0 flex w-[390px] max-w-[100vw] list-none flex-col gap-[10px] p-[var(--viewport-padding)] outline-none [--viewport-padding:_25px]',
            config?.viewPortClassName
          )}
          style={{ zIndex: zIndices.toast }}
        />
      </ToastLib.Provider>
    </ToastContext.Provider>
  );
}

export type ToastConfig = {
  title?: string | React.ReactNode;
  description?: React.ReactNode;
  action?: React.ReactNode;
  duration?: number;
  viewPortClassName?: string;
  error?: boolean;
  type?: 'error' | 'info' | 'warning' | 'success';
};

const stopSwipeOnTextSelect = (e: React.PointerEvent) => e.stopPropagation();

const SURFACE_BY_TYPE = {
  default: 'bg-surface-level-2 border-subtle',
  error: 'bg-error border-error',
  warning: 'bg-warning border-warning',
  success: 'bg-success border-success',
  info: 'bg-brand-subtle border-brand-75',
} as const;

function renderToastIcon(isError: boolean, type: ToastConfig['type']) {
  if (isError) return <Icon icon={AlertSolidIcon} rounded color="error" />;
  if (type === 'success')
    return <Icon icon={CheckIcon} rounded color="success" />;
  if (type === 'warning')
    return <Icon icon={AlertSolidIcon} rounded color="warning" />;
  if (type === 'info')
    return <Icon icon={InfoCircleSolidIcon} rounded color="info" />;
  return <Icon icon={InfoCircleSolidIcon} rounded color="neutral" />;
}

function Toast({
  title,
  open,
  setOpen,
  description,
  action,
  error,
  duration,
  type,
}: {
  title?: string | React.ReactNode;
  open: boolean;
  setOpen?: (open: boolean) => void;
  description?: React.ReactNode;
  action?: React.ReactNode;
  error?: boolean;
  duration?: number;
  type?: 'error' | 'info' | 'warning' | 'success';
}) {
  const isError = error || type === 'error';
  const surface = SURFACE_BY_TYPE[isError ? 'error' : (type ?? 'default')];
  const hasTitle = title !== undefined && title !== null && title !== '';
  const copyText =
    isError && typeof title === 'string'
      ? typeof description === 'string'
        ? `${title}\n${description}`
        : title
      : null;

  const descriptionNode = description && (
    <ToastLib.Description asChild>
      <div
        className={cn(
          'max-h-32 cursor-text select-text overflow-y-auto whitespace-pre-wrap',
          hasTitle
            ? 'ml-space-6 text-xxs text-secondary'
            : 'text-sm text-primary'
        )}
        onPointerDown={stopSwipeOnTextSelect}
      >
        {description}
      </div>
    </ToastLib.Description>
  );

  const trailingButtons = (
    <div className="flex gap-space-2">
      {copyText && (
        <CopyIconButton
          size="xs"
          copy={copyText}
          copyText="Copy error message"
        />
      )}
      <ToastLib.Close asChild>
        <IconButton
          icon={XCloseIcon}
          label="Dismiss"
          size="xs"
          variant="plain"
          color="secondary"
        />
      </ToastLib.Close>
    </div>
  );

  return (
    <ToastLib.Root
      className={cn(
        'flex items-start gap-space-2 rounded-md border p-space-3',
        surface,
        'shadow-[hsl(206_22%_7%_/_35%)_0px_10px_38px_-10px,_hsl(206_22%_7%_/_20%)_0px_10px_20px_-15px]',
        'data-[swipe=cancel]:translate-x-0 data-[swipe=move]:translate-x-[var(--radix-toast-swipe-move-x)] data-[state=closed]:animate-hide data-[state=open]:animate-slideIn data-[swipe=end]:animate-swipeOut data-[swipe=cancel]:transition-[transform_200ms_ease-out]'
      )}
      open={open}
      onOpenChange={setOpen}
      duration={duration}
    >
      {hasTitle ? (
        <div className="flex w-full flex-col gap-space-2 pb-space-1">
          <div className="flex w-full items-center">
            <div className="flex w-full items-center gap-space-2">
              {renderToastIcon(isError, type)}
              <ToastLib.Title
                className="cursor-text select-text font-medium text-primary"
                onPointerDown={stopSwipeOnTextSelect}
              >
                {title}
              </ToastLib.Title>
            </div>
            {trailingButtons}
          </div>
          {descriptionNode}
          {action && <div className="ml-space-6">{action}</div>}
        </div>
      ) : (
        <div className="flex w-full items-start gap-space-2">
          {renderToastIcon(isError, type)}
          <div className="flex flex-1 flex-col gap-space-2">
            {descriptionNode}
            {action}
          </div>
          {trailingButtons}
        </div>
      )}
    </ToastLib.Root>
  );
}
