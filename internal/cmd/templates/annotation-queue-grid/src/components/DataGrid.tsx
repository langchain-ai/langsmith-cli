import { Fragment, useEffect, useRef } from 'react';
import { ChevronDownIcon, ChevronRightIcon } from '@langchain/untitled-ui-icons';
import { GridCell } from './GridCell';
import { Spinner } from './Spinner';
import type {
  AnnotationQueue,
  AnnotationQueueRun,
  FeedbackConfig,
  FeedbackItem,
  RubricItem,
} from '../types';
import { cn } from '../lib/utils';

interface Props {
  queue: AnnotationQueue | null;
  columns: RubricItem[];
  configs: Record<string, FeedbackConfig>;
  rows: AnnotationQueueRun[];
  rowsLoading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  onLoadMore: () => void;
  feedbackByRun: Record<string, Record<string, FeedbackItem>>;
  activeRow: number;
  expandedRunId: string | null;
  completeError: string | null;
  onToggleExpand: (runId: string) => void;
  onActivateRow: (index: number) => void;
  onCellSaved: (runId: string, feedback: FeedbackItem) => void;
  onCellDeleted: (runId: string, feedbackKey: string) => void;
  onComplete: (index: number) => void;
}

function stringifyIO(value: Record<string, unknown> | null): string {
  if (!value) return '';
  try {
    return JSON.stringify(value);
  } catch {
    return '';
  }
}

function prettyIO(value: Record<string, unknown> | null): string {
  if (!value) return '(empty)';
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function DataGrid({
  queue,
  columns,
  configs,
  rows,
  rowsLoading,
  loadingMore,
  hasMore,
  onLoadMore,
  feedbackByRun,
  activeRow,
  expandedRunId,
  completeError,
  onToggleExpand,
  onActivateRow,
  onCellSaved,
  onCellDeleted,
  onComplete,
}: Props) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  // Infinite scroll: fetch the next page once the sentinel at the bottom of
  // the scroll container comes into view.
  useEffect(() => {
    if (!hasMore) return;
    const root = scrollRef.current;
    const target = sentinelRef.current;
    if (!root || !target) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) onLoadMore();
      },
      { root, rootMargin: '200px' }
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, [hasMore, onLoadMore]);

  if (!queue) {
    return (
      <div className="flex flex-1 items-center justify-center rounded-lg border border-secondary">
        <span className="text-sm text-tertiary">Loading queue…</span>
      </div>
    );
  }

  // A row's required columns must all have feedback before it can be marked
  // complete — mirrors the 3-pane FeedbackPanel's allRequiredFilled gate.
  function requiredFilled(runId: string): boolean {
    const rowFeedback = feedbackByRun[runId] ?? {};
    return columns.filter((c) => c.is_required).every((c) => rowFeedback[c.feedback_key] != null);
  }

  const colSpan = 3 + columns.length + 1;

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-secondary">
      {/* Header bar */}
      <div className="flex items-center justify-between border-b border-secondary px-4 py-3">
        <div className="flex items-baseline gap-2">
          <span className="text-base font-medium text-primary">{queue.name}</span>
          <span className="text-sm text-tertiary">
            {rows.length}
            {hasMore ? '+' : ''} to review · ↑/↓ to move between rows
          </span>
        </div>
      </div>

      {completeError && (
        <div role="alert" className="border-b border-secondary bg-error px-4 py-2 text-xs text-error-secondary">
          Failed to mark complete: {completeError}
        </div>
      )}

      {/* Grid */}
      <div ref={scrollRef} className="min-h-0 flex-1 overflow-auto">
        {columns.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <span className="text-sm text-tertiary">This queue has no rubric items to score.</span>
          </div>
        ) : rows.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <span className="text-sm text-tertiary">
              {rowsLoading ? 'Loading runs…' : 'Nothing left to review 🎉'}
            </span>
          </div>
        ) : (
          <table className="w-full border-collapse text-left">
            <thead className="sticky top-0 z-10 bg-surface-level-2">
              <tr>
                <th className="w-[220px] min-w-[220px] border-b border-secondary px-3 py-2 text-xs font-medium text-tertiary">
                  Run Name
                </th>
                <th className="w-[220px] min-w-[220px] border-b border-l border-secondary px-3 py-2 text-xs font-medium text-tertiary">
                  Inputs
                </th>
                <th className="w-[220px] min-w-[220px] border-b border-l border-secondary px-3 py-2 text-xs font-medium text-tertiary">
                  Outputs
                </th>
                {columns.map((col) => (
                  <th
                    key={col.feedback_key}
                    className="min-w-[140px] border-b border-l border-secondary px-3 py-2 text-xs font-medium text-tertiary"
                    title={col.description ?? undefined}
                  >
                    <span className="flex items-center gap-1">
                      {col.feedback_key}
                      {col.is_required && <span className="text-error-primary">*</span>}
                    </span>
                  </th>
                ))}
                <th className="w-[140px] border-b border-l border-secondary px-3 py-2 text-xs font-medium text-tertiary">
                  {/* Mark Completed column */}
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((run, index) => {
                const isActive = index === activeRow;
                const isExpanded = expandedRunId === run.id;
                const rowFeedback = feedbackByRun[run.id] ?? {};
                const inputsPreview = stringifyIO(run.inputs);
                const outputsPreview = stringifyIO(run.outputs);
                return (
                  <Fragment key={run.queue_run_id}>
                    <tr
                      onClick={() => onActivateRow(index)}
                      className={cn(
                        'cursor-pointer',
                        !isExpanded && 'border-b border-secondary',
                        isActive
                          ? 'bg-selected'
                          : index % 2 === 1
                            ? 'bg-secondary/30 hover:bg-surface-level-1-hover'
                            : 'hover:bg-surface-level-1-hover'
                      )}
                    >
                      <td className="max-w-[220px] px-3 py-1.5 align-middle text-sm text-secondary">
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            onToggleExpand(run.id);
                          }}
                          className="flex w-full min-w-0 items-center gap-1.5 rounded text-left hover:text-primary"
                        >
                          {isExpanded ? (
                            <ChevronDownIcon className="h-3.5 w-3.5 shrink-0 text-tertiary" />
                          ) : (
                            <ChevronRightIcon className="h-3.5 w-3.5 shrink-0 text-tertiary" />
                          )}
                          <span className="min-w-0 truncate" title={run.name ?? run.id}>
                            {run.name ?? run.id.slice(0, 8)}
                          </span>
                        </button>
                      </td>
                      <td
                        className="max-w-[220px] cursor-pointer truncate border-l border-secondary px-3 py-1.5 align-middle font-mono text-xs text-tertiary hover:text-secondary"
                        title={inputsPreview}
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggleExpand(run.id);
                        }}
                      >
                        {inputsPreview || '—'}
                      </td>
                      <td
                        className="max-w-[220px] cursor-pointer truncate border-l border-secondary px-3 py-1.5 align-middle font-mono text-xs text-tertiary hover:text-secondary"
                        title={outputsPreview}
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggleExpand(run.id);
                        }}
                      >
                        {outputsPreview || '—'}
                      </td>
                      {columns.map((col) => (
                        <td key={col.feedback_key} className="border-l border-secondary px-1 py-1 align-middle">
                          <GridCell
                            item={col}
                            config={configs[col.feedback_key]}
                            runId={run.id}
                            traceId={run.trace_id}
                            sessionId={run.session_id}
                            startTime={run.start_time}
                            existingFeedback={rowFeedback[col.feedback_key]}
                            onSaved={(fb) => onCellSaved(run.id, fb)}
                            onDeleted={(key) => onCellDeleted(run.id, key)}
                          />
                        </td>
                      ))}
                      <td className="border-l border-secondary px-2 py-1 text-center align-middle">
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            onComplete(index);
                          }}
                          disabled={!requiredFilled(run.id)}
                          title={!requiredFilled(run.id) ? 'Fill all required (*) columns first' : undefined}
                          className="rounded-md bg-brand px-3 py-1 text-xs font-medium text-brand-on-fill transition-colors hover:bg-brand-hover disabled:opacity-50"
                        >
                          Mark Completed
                        </button>
                      </td>
                    </tr>
                    {isExpanded && (
                      <tr className="border-b border-secondary bg-surface-level-1">
                        <td colSpan={colSpan} className="px-3 py-3">
                          <div className="grid grid-cols-2 gap-4">
                            <div className="flex flex-col gap-1">
                              <span className="text-xs font-medium text-tertiary">Inputs</span>
                              <pre className="max-h-[300px] overflow-auto whitespace-pre-wrap break-words rounded-md bg-surface-level-2 p-2 font-mono text-xs text-secondary">
                                {prettyIO(run.inputs)}
                              </pre>
                            </div>
                            <div className="flex flex-col gap-1">
                              <span className="text-xs font-medium text-tertiary">Outputs</span>
                              <pre className="max-h-[300px] overflow-auto whitespace-pre-wrap break-words rounded-md bg-surface-level-2 p-2 font-mono text-xs text-secondary">
                                {prettyIO(run.outputs)}
                              </pre>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
              {hasMore && (
                <tr>
                  <td colSpan={colSpan} className="px-3 py-3">
                    <div ref={sentinelRef} className="flex items-center justify-center">
                      {loadingMore && <Spinner size="sm" />}
                    </div>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
