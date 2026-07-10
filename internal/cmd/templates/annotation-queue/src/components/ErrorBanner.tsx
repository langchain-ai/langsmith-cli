import { useState } from 'react';
import { AlertTriangleIcon, ChevronRightIcon } from '@langchain/untitled-ui-icons';
import { cn } from '../lib/utils';

/** Red alert banner shown above a run's outputs when the run errored, mirroring LangSmith's run error card. */
export function ErrorBanner({ error }: { error: string }) {
  const [expanded, setExpanded] = useState(false);
  const canExpand = error.includes('\n') || error.length > 100;

  return (
    <div role="alert" className="mb-3 w-full rounded-sm bg-error px-2.5 py-1">
      <div className="flex w-full items-center gap-2">
        <AlertTriangleIcon className="h-3.5 w-3.5 shrink-0 text-error-primary" />
        <div className="flex min-w-0 flex-1 gap-1">
          <span className="shrink-0 whitespace-nowrap text-xs font-medium text-error-secondary">
            Error
          </span>
          <span
            className={cn(
              'min-w-0 flex-1 text-xs text-error-secondary',
              !expanded && 'truncate whitespace-nowrap'
            )}
          >
            {error}
          </span>
        </div>
        {canExpand && (
          <button
            type="button"
            className="shrink-0 text-error-secondary"
            onClick={() => setExpanded((v) => !v)}
          >
            <ChevronRightIcon className={cn('h-3.5 w-3.5 transition-transform', expanded && 'rotate-90')} />
          </button>
        )}
      </div>
      {expanded && (
        <div className="mt-1 pl-6">
          <span className="whitespace-pre-wrap break-words text-xs text-error-secondary">{error}</span>
        </div>
      )}
    </div>
  );
}
