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
  /** Exact total for this section (see useRunSection) — shown in the header
   * bar instead of deriving a count from the loaded page. */
  total: number;
  rowsLoading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  onLoadMore: () => void;
  feedbackByRun: Record<string, Record<string, FeedbackItem>>;
  activeRow: number;
  expandedRunId: string | null;
  completeError: string | null;
  selectedRunIds: Set<string>;
  onToggleExpand: (runId: string) => void;
  onToggleRowSelected: (queueRunId: string) => void;
  onToggleSelectAll: () => void;
  onBulkComplete: () => void;
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

// Clickable, always-visibly-interactive treatment shared by the Run
// Name/Inputs/Outputs cells — the whole column, not just its text, opens
// the expanded row.
const expandableCellClass =
  'max-w-[220px] cursor-pointer truncate px-3 py-1.5 align-middle hover:bg-surface-level-2';

export function DataGrid({
  queue,
  columns,
  configs,
  rows,
  total,
  rowsLoading,
  loadingMore,
  hasMore,
  onLoadMore,
  feedbackByRun,
  activeRow,
  expandedRunId,
  completeError,
  selectedRunIds,
  onToggleExpand,
  onToggleRowSelected,
  onToggleSelectAll,
  onBulkComplete,
  onActivateRow,
  onCellSaved,
  onCellDeleted,
  onComplete,
}: Props) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const sentinelRef = useRef<HTMLDivElement | null>(null);
  const selectAllRef = useRef<HTMLInputElement | null>(null);

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

  // The sentinel above only helps once it's actually on screen — but when
  // every loaded row gets marked complete (e.g. a bulk action clears the
  // whole visible page), the table itself unmounts in favor of the empty
  // state below, taking the sentinel with it. Nothing would ever trigger
  // the next page again even though more rows exist server-side, so fetch
  // proactively whenever the list runs dry while more is still available.
  useEffect(() => {
    if (rows.length === 0 && hasMore && !rowsLoading && !loadingMore) {
      onLoadMore();
    }
  }, [rows.length, hasMore, rowsLoading, loadingMore, onLoadMore]);

  // Reflect partial selection as the native indeterminate visual — plain
  // HTML has no attribute for this, it's set imperatively on the node.
  useEffect(() => {
    if (!selectAllRef.current) return;
    selectAllRef.current.indeterminate = selectedRunIds.size > 0 && selectedRunIds.size < rows.length;
  }, [selectedRunIds.size, rows.length]);

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

  const colSpan = 1 + 3 + columns.length + 1;

  // Keep the active-row highlight in lockstep with whatever cell actually
  // holds focus — mouse click into a cell, Tab, or arrow-key navigation all
  // land here because React's onFocus bubbles (focusin semantics). This is
  // the single source of truth for activeRow; without it the highlight goes
  // stale (clicking a cell can't rely on the row onClick — the cell's <td>
  // stops propagation — and Tab uses native focus the arrow handler never
  // sees), then "jumps" on the next arrow press.
  function handleGridFocus(e: React.FocusEvent<HTMLDivElement>) {
    const rowAttr = (e.target as HTMLElement).getAttribute('data-row-index');
    if (rowAttr == null) return;
    onActivateRow(Number(rowAttr));
  }

  // Arrow keys move focus between feedback cells (identified by the
  // data-row-index/data-col-index GridCell stamps onto its input/select) —
  // this is what makes the grid navigable like an actual spreadsheet
  // instead of just editable one field at a time. Deliberately unconditional
  // (not just at text boundaries): values here are short scores/categories/
  // comments, so trading away in-text left/right cursor movement for fast
  // cell-to-cell movement is the right default for a review workflow.
  // Moving focus fires handleGridFocus above, which is what updates activeRow.
  function handleGridKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    if (!['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(e.key)) return;
    const target = e.target as HTMLElement;
    const rowAttr = target.getAttribute('data-row-index');
    const colAttr = target.getAttribute('data-col-index');
    if (rowAttr == null || colAttr == null) return;
    const row = Number(rowAttr);
    const col = Number(colAttr);

    let nextRow = row;
    let nextCol = col;
    if (e.key === 'ArrowUp') nextRow -= 1;
    else if (e.key === 'ArrowDown') nextRow += 1;
    else if (e.key === 'ArrowLeft') nextCol -= 1;
    else nextCol += 1;

    if (nextRow < 0 || nextRow >= rows.length || nextCol < 0 || nextCol >= columns.length) return;

    e.preventDefault();
    const el = scrollRef.current?.querySelector<HTMLElement>(
      `[data-row-index="${nextRow}"][data-col-index="${nextCol}"]`
    );
    if (!el) return;
    el.focus();
    if (el instanceof HTMLInputElement) el.select();
    // activeRow is updated by handleGridFocus when el.focus() fires above.
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-secondary">
      {/* Header bar */}
      <div className="flex items-center justify-between border-b border-secondary px-4 py-3">
        <div className="flex items-baseline gap-2">
          <span className="text-base font-medium text-primary">{queue.name}</span>
          <span className="text-sm text-tertiary">
            {Math.max(total, rows.length)} to review ·{' '}
            {/* ←/→ only move between rubric columns, so only advertise them
                when there's more than one to move between. */}
            {columns.length > 1 ? '↑/↓/←/→' : '↑/↓'} to move between cells
          </span>
        </div>
        {selectedRunIds.size > 0 && (
          <button
            type="button"
            onClick={onBulkComplete}
            className="rounded-md bg-brand px-3 py-1.5 text-xs font-medium text-brand-on-fill transition-colors hover:bg-brand-hover"
          >
            Mark {selectedRunIds.size} Completed
          </button>
        )}
      </div>

      {completeError && (
        <div role="alert" className="border-b border-secondary bg-error px-4 py-2 text-xs text-error-secondary">
          Failed to mark complete: {completeError}
        </div>
      )}

      {/* Grid */}
      <div
        ref={scrollRef}
        onKeyDown={handleGridKeyDown}
        onFocus={handleGridFocus}
        className="min-h-0 flex-1 overflow-auto"
      >
        {columns.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <span className="text-sm text-tertiary">This queue has no rubric items to score.</span>
          </div>
        ) : rows.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <span className="text-sm text-tertiary">
              {/* hasMore here means the auto-fetch effect above is already
                  loading the next page — never flash "nothing left" when
                  we know there's more coming. */}
              {rowsLoading || loadingMore || hasMore ? 'Loading runs…' : 'Nothing left to review 🎉'}
            </span>
          </div>
        ) : (
          <table className="w-full border-collapse text-left">
            <thead className="sticky top-0 z-10 bg-surface-level-2">
              <tr>
                <th className="w-10 border-b border-secondary px-3 py-2">
                  <input
                    ref={selectAllRef}
                    type="checkbox"
                    checked={rows.length > 0 && selectedRunIds.size === rows.length}
                    onChange={onToggleSelectAll}
                    aria-label="Select all rows"
                    className="h-4 w-4 cursor-pointer accent-[var(--bg-brand)]"
                  />
                </th>
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
                const isSelected = selectedRunIds.has(run.queue_run_id);
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
                      <td className="px-3 py-1.5 align-middle" onClick={(e) => e.stopPropagation()}>
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => onToggleRowSelected(run.queue_run_id)}
                          aria-label={`Select ${run.name ?? run.id}`}
                          className="h-4 w-4 cursor-pointer accent-[var(--bg-brand)]"
                        />
                      </td>
                      <td
                        className={cn(expandableCellClass, 'text-sm text-secondary')}
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggleExpand(run.id);
                        }}
                      >
                        <div className="flex min-w-0 items-center gap-1.5">
                          {isExpanded ? (
                            <ChevronDownIcon className="h-3.5 w-3.5 shrink-0 text-tertiary" />
                          ) : (
                            <ChevronRightIcon className="h-3.5 w-3.5 shrink-0 text-tertiary" />
                          )}
                          <span className="min-w-0 truncate" title={run.name ?? run.id}>
                            {run.name ?? run.id.slice(0, 8)}
                          </span>
                        </div>
                      </td>
                      <td
                        className={cn(expandableCellClass, 'border-l border-secondary font-mono text-xs text-tertiary')}
                        title={inputsPreview}
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggleExpand(run.id);
                        }}
                      >
                        {inputsPreview || '—'}
                      </td>
                      <td
                        className={cn(expandableCellClass, 'border-l border-secondary font-mono text-xs text-tertiary')}
                        title={outputsPreview}
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggleExpand(run.id);
                        }}
                      >
                        {outputsPreview || '—'}
                      </td>
                      {columns.map((col, colIndex) => (
                        <td
                          key={col.feedback_key}
                          className="border-l border-secondary px-1 py-1 align-middle"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <GridCell
                            item={col}
                            config={configs[col.feedback_key]}
                            runId={run.id}
                            traceId={run.trace_id}
                            sessionId={run.session_id}
                            startTime={run.start_time}
                            existingFeedback={rowFeedback[col.feedback_key]}
                            rowIndex={index}
                            colIndex={colIndex}
                            onSaved={(fb) => onCellSaved(run.id, fb)}
                            onDeleted={(key) => onCellDeleted(run.id, key)}
                          />
                        </td>
                      ))}
                      <td className="border-l border-secondary px-2 py-1 text-center align-middle">
                        {/* One or more rows selected: only the bulk action
                            in the header bar applies — hide the per-row
                            button instead of leaving a redundant, easy-to-
                            misclick affordance next to the checkboxes. */}
                        {selectedRunIds.size === 0 && (
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
                        )}
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
