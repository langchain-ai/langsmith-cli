import { Fragment, useEffect, useMemo, useRef, useState } from 'react';
import { ChevronDownIcon, ChevronRightIcon } from '@langchain/untitled-ui-icons';
import { Badge } from '@/components/langsmith/design-system/components/Badge';
import { Banner } from '@/components/langsmith/design-system/components/Banner';
import { Button } from '@/components/langsmith/design-system/components/Button';
import { Checkbox, getCheckedState } from '@/components/langsmith/design-system/components/Checkbox';
import { Spinner } from '@/components/langsmith/design-system/components/Spinner';
import { Text } from '@/components/langsmith/design-system/components/Text';
import { GridCell } from './GridCell';
import { ThreadViewer } from './ThreadViewer';
import type {
  AnnotationQueue,
  FeedbackConfig,
  FeedbackItem,
  QueueItem,
  RubricItem,
} from '../types';
import { feedbackSubjectKey, itemLabel } from '../types';
import { cn } from '../lib/utils';

interface Props {
  queue: AnnotationQueue | null;
  columns: RubricItem[];
  configs: Record<string, FeedbackConfig>;
  rows: QueueItem[];
  /** Exact total for this section (see useItemSection) — shown in the header
   * bar instead of deriving a count from the loaded page. */
  total: number;
  rowsLoading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  onLoadMore: () => void;
  feedbackBySubject: Record<string, Record<string, FeedbackItem>>;
  activeRow: number;
  expandedItemId: string | null;
  expandLoading: boolean;
  completeError: string | null;
  selectedItemIds: Set<string>;
  onToggleExpand: (itemId: string) => void;
  onToggleRowSelected: (itemId: string) => void;
  onToggleSelectAll: () => void;
  onBulkComplete: () => void;
  onActivateRow: (index: number) => void;
  onCellSaved: (subjectKey: string, feedback: FeedbackItem) => void;
  onCellDeleted: (subjectKey: string, feedbackKey: string) => void;
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
  'cursor-pointer truncate px-space-3 py-1.5 align-middle hover:bg-surface-level-2';

const CHECKBOX_COL_WIDTH = 40;
const COMPLETE_COL_WIDTH = 140;
const NAME_COL_DEFAULT = 220;
const MIN_COL_WIDTH = 80;
const FALLBACK_CONTAINER_WIDTH = 1200;

function seedWidths(ids: string[], available: number): Record<string, number> {
  const out: Record<string, number> = {};
  if (ids.length === 0) return out;
  const dataIds = ids.slice(1);
  let nameW = NAME_COL_DEFAULT;
  if (nameW > available - dataIds.length * MIN_COL_WIDTH) {
    nameW = Math.max(MIN_COL_WIDTH, Math.floor(available / ids.length));
  }
  out[ids[0]] = nameW;
  const each = Math.max(MIN_COL_WIDTH, Math.floor((available - nameW) / Math.max(1, dataIds.length)));
  dataIds.forEach((id) => {
    out[id] = each;
  });
  const last = ids[ids.length - 1];
  const sum = ids.reduce((s, id) => s + out[id], 0);
  out[last] = Math.max(MIN_COL_WIDTH, out[last] + (available - sum));
  return out;
}

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
  feedbackBySubject,
  activeRow,
  expandedItemId,
  expandLoading,
  completeError,
  selectedItemIds,
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

  const [containerWidth, setContainerWidth] = useState(0);
  const [columnWidths, setColumnWidths] = useState<Record<string, number>>({});
  const resizeRef = useRef<{
    leftId: string;
    rightId: string;
    startX: number;
    startLeft: number;
    startRight: number;
  } | null>(null);

  const middleIds = useMemo(
    () => ['name', 'inputs', 'outputs', ...columns.map((c) => c.feedback_key)],
    [columns]
  );

  const available = Math.max(
    MIN_COL_WIDTH * middleIds.length,
    (containerWidth || FALLBACK_CONTAINER_WIDTH) - CHECKBOX_COL_WIDTH - COMPLETE_COL_WIDTH
  );

  const widthsReady =
    middleIds.every((id) => columnWidths[id] != null) &&
    Object.keys(columnWidths).length === middleIds.length;
  const resolvedWidths = widthsReady ? columnWidths : seedWidths(middleIds, available);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const update = () => setContainerWidth(el.clientWidth);
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    if (containerWidth <= 0) return;
    setColumnWidths((prev) => {
      const hasAll =
        middleIds.every((id) => prev[id] != null) && Object.keys(prev).length === middleIds.length;
      if (!hasAll) return seedWidths(middleIds, available);
      const sum = middleIds.reduce((s, id) => s + prev[id], 0);
      if (sum <= 0 || Math.abs(sum - available) < 1) return prev;
      const scale = available / sum;
      const scaled: Record<string, number> = {};
      middleIds.forEach((id) => {
        scaled[id] = Math.max(MIN_COL_WIDTH, Math.round(prev[id] * scale));
      });
      const last = middleIds[middleIds.length - 1];
      const scaledSum = middleIds.reduce((s, id) => s + scaled[id], 0);
      scaled[last] = Math.max(MIN_COL_WIDTH, scaled[last] + (available - scaledSum));
      return scaled;
    });
  }, [containerWidth, middleIds, available]);

  useEffect(() => {
    function onMove(e: PointerEvent) {
      const drag = resizeRef.current;
      if (!drag) return;
      const total = drag.startLeft + drag.startRight;
      let newLeft = drag.startLeft + (e.clientX - drag.startX);
      newLeft = Math.max(MIN_COL_WIDTH, Math.min(newLeft, total - MIN_COL_WIDTH));
      setColumnWidths((prev) => ({ ...prev, [drag.leftId]: newLeft, [drag.rightId]: total - newLeft }));
    }
    function onUp() {
      if (!resizeRef.current) return;
      resizeRef.current = null;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    }
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
    return () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
    };
  }, []);

  function startResize(e: React.PointerEvent, leftId: string, rightId: string) {
    e.preventDefault();
    e.stopPropagation();
    resizeRef.current = {
      leftId,
      rightId,
      startX: e.clientX,
      startLeft: resolvedWidths[leftId],
      startRight: resolvedWidths[rightId],
    };
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }

  function resizeHandle(leftId: string, rightId: string | null) {
    if (!rightId) return null;
    return (
      <span
        onPointerDown={(e) => startResize(e, leftId, rightId)}
        onClick={(e) => e.stopPropagation()}
        className="absolute right-0 top-0 z-20 h-full w-1.5 cursor-col-resize hover:bg-brand"
      />
    );
  }

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

  if (!queue) {
    return (
      <div className="flex flex-1 items-center justify-center rounded-lg border border-default">
        <Text variant="md" color="tertiary">
          Loading queue…
        </Text>
      </div>
    );
  }

  // A row's required columns must all have feedback before it can be marked
  // complete — mirrors the 3-pane FeedbackPanel's allRequiredFilled gate.
  function requiredFilled(subjectKey: string | undefined): boolean {
    if (!subjectKey) return false;
    const rowFeedback = feedbackBySubject[subjectKey] ?? {};
    return columns.filter((c) => c.is_required).every((c) => rowFeedback[c.feedback_key] != null);
  }

  const colSpan = 1 + 3 + columns.length + 1;

  // Sync the active-row highlight to whichever cell holds focus.
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
    // focus fires handleGridFocus, which sets activeRow.
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-default">
      {/* Header bar */}
      <div className="flex items-center justify-between gap-space-4 border-b border-default px-space-4 py-space-3">
        <div className="flex items-baseline gap-space-2">
          <Text variant="h3">{queue.name}</Text>
          <Text variant="md" color="tertiary">
            {`${Math.max(total, rows.length)} to review · ${
              /* ←/→ only matter with multiple rubric columns. */
              columns.length > 1 ? '↑/↓/←/→' : '↑/↓'
            } to move between cells`}
          </Text>
        </div>
        {selectedItemIds.size > 0 && (
          <Button size="sm" onClick={onBulkComplete}>
            {`Mark ${selectedItemIds.size} Completed`}
          </Button>
        )}
      </div>

      {completeError && (
        <Banner intent="error" flush title="Failed to mark complete">
          {completeError}
        </Banner>
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
            <Text variant="md" color="tertiary">
              This queue has no rubric items to score.
            </Text>
          </div>
        ) : rows.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            {/* hasMore here means the auto-fetch effect above is already
                loading the next page — never flash "nothing left" when
                we know there's more coming. */}
            <Text variant="md" color="tertiary">
              {rowsLoading || loadingMore || hasMore ? 'Loading items…' : 'Nothing left to review 🎉'}
            </Text>
          </div>
        ) : (
          <table className="w-full table-fixed border-collapse text-left">
            <colgroup>
              <col style={{ width: CHECKBOX_COL_WIDTH }} />
              {middleIds.map((id) => (
                <col key={id} style={{ width: resolvedWidths[id] }} />
              ))}
              <col style={{ width: COMPLETE_COL_WIDTH }} />
            </colgroup>
            <thead className="sticky top-0 z-10 bg-surface-level-2">
              <tr>
                <th className="border-b border-default px-space-3 py-space-2">
                  <Checkbox
                    checked={getCheckedState(
                      rows.length > 0 && selectedItemIds.size === rows.length,
                      selectedItemIds.size > 0
                    )}
                    onCheckedChange={onToggleSelectAll}
                    aria-label="Select all rows"
                  />
                </th>
                <th className="relative border-b border-default px-space-3 py-space-2 text-xs font-medium text-tertiary">
                  Item
                  {resizeHandle('name', 'inputs')}
                </th>
                <th className="relative border-b border-l border-default px-space-3 py-space-2 text-xs font-medium text-tertiary">
                  Inputs
                  {resizeHandle('inputs', 'outputs')}
                </th>
                <th className="relative border-b border-l border-default px-space-3 py-space-2 text-xs font-medium text-tertiary">
                  Outputs
                  {resizeHandle('outputs', columns[0]?.feedback_key ?? null)}
                </th>
                {columns.map((col, colIndex) => (
                  <th
                    key={col.feedback_key}
                    className="relative border-b border-l border-default px-space-3 py-space-2 text-xs font-medium text-tertiary"
                    title={col.description ?? undefined}
                  >
                    <span className="flex items-center gap-space-1">
                      {col.feedback_key}
                      {col.is_required && <span className="text-error-primary">*</span>}
                    </span>
                    {resizeHandle(col.feedback_key, columns[colIndex + 1]?.feedback_key ?? null)}
                  </th>
                ))}
                <th className="border-b border-l border-default px-space-3 py-space-2 text-xs font-medium text-tertiary" />
              </tr>
            </thead>
            <tbody>
              {rows.map((item, index) => {
                const isActive = index === activeRow;
                const isExpanded = expandedItemId === item.id;
                const isSelected = selectedItemIds.has(item.id);
                const subjectKey = feedbackSubjectKey(item);
                const rowFeedback = subjectKey ? (feedbackBySubject[subjectKey] ?? {}) : {};
                const isThread = item.item_type === 'THREAD';
                const inputsPreview = isThread
                  ? 'Open to view messages'
                  : stringifyIO(item.inputs ?? null);
                const outputsPreview = isThread ? '—' : stringifyIO(item.outputs ?? null);
                const label = itemLabel(item);
                return (
                  <Fragment key={item.id}>
                    <tr
                      onClick={() => onActivateRow(index)}
                      className={cn(
                        'cursor-pointer',
                        !isExpanded && 'border-b border-default',
                        isActive
                          ? 'bg-selected'
                          : index % 2 === 1
                            ? 'bg-surface-level-2/30 hover:bg-surface-level-1-hover'
                            : 'hover:bg-surface-level-1-hover'
                      )}
                    >
                      <td className="px-space-3 py-1.5 align-middle" onClick={(e) => e.stopPropagation()}>
                        <Checkbox
                          checked={isSelected}
                          onCheckedChange={() => onToggleRowSelected(item.id)}
                          aria-label={`Select ${label}`}
                        />
                      </td>
                      <td
                        className={cn(expandableCellClass, 'text-sm text-secondary')}
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggleExpand(item.id);
                        }}
                      >
                        <div className="flex min-w-0 items-center gap-1.5">
                          {isExpanded ? (
                            <ChevronDownIcon className="h-3.5 w-3.5 shrink-0 text-tertiary" />
                          ) : (
                            <ChevronRightIcon className="h-3.5 w-3.5 shrink-0 text-tertiary" />
                          )}
                          <Badge
                            size="xxs"
                            rounded="xs"
                            color={isThread ? 'primary' : 'secondary'}
                            className="shrink-0 uppercase"
                          >
                            {isThread ? 'Thread' : 'Run'}
                          </Badge>
                          <span className="min-w-0 truncate" title={label}>
                            {label}
                          </span>
                        </div>
                      </td>
                      <td
                        className={cn(expandableCellClass, 'border-l border-default font-mono text-xs text-tertiary')}
                        title={inputsPreview}
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggleExpand(item.id);
                        }}
                      >
                        {inputsPreview || '—'}
                      </td>
                      <td
                        className={cn(expandableCellClass, 'border-l border-default font-mono text-xs text-tertiary')}
                        title={outputsPreview}
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggleExpand(item.id);
                        }}
                      >
                        {outputsPreview || '—'}
                      </td>
                      {columns.map((col, colIndex) => (
                        <td
                          key={col.feedback_key}
                          className="border-l border-default p-space-1 align-middle"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <GridCell
                            item={col}
                            config={configs[col.feedback_key]}
                            itemType={item.item_type}
                            runId={item.item_type === 'RUN' ? item.run_id : undefined}
                            feedbackThreadId={
                              item.item_type === 'THREAD' ? item.thread_id : undefined
                            }
                            traceId={item.trace_id}
                            sessionId={item.project_id}
                            startTime={item.start_time}
                            existingFeedback={rowFeedback[col.feedback_key]}
                            rowIndex={index}
                            colIndex={colIndex}
                            onSaved={(fb) => subjectKey && onCellSaved(subjectKey, fb)}
                            onDeleted={(key) => subjectKey && onCellDeleted(subjectKey, key)}
                          />
                        </td>
                      ))}
                      <td className="border-l border-default px-space-2 py-space-1 text-center align-middle">
                        {selectedItemIds.size === 0 && (
                          <Button
                            size="xs"
                            onClick={(e) => {
                              e.stopPropagation();
                              onComplete(index);
                            }}
                            disabled={!requiredFilled(subjectKey)}
                            title={
                              !requiredFilled(subjectKey)
                                ? 'Fill all required (*) columns first'
                                : undefined
                            }
                          >
                            Mark Completed
                          </Button>
                        )}
                      </td>
                    </tr>
                    {isExpanded && (
                      <tr className="border-b border-default bg-surface-level-1">
                        <td colSpan={colSpan} className="p-space-3">
                          {isThread ? (
                            <div className="max-h-[360px] overflow-auto">
                              <ThreadViewer
                                messages={item.messages}
                                threadId={item.thread_id}
                                loading={expandLoading}
                              />
                            </div>
                          ) : (
                            <div className="grid grid-cols-2 gap-space-4">
                              <div className="flex flex-col gap-space-1">
                                <Text variant="sm" weight="medium" color="tertiary">
                                  Inputs
                                </Text>
                                <pre className="max-h-[300px] overflow-auto whitespace-pre-wrap break-words rounded-md bg-surface-level-2 p-space-2 font-mono text-xs text-secondary">
                                  {expandLoading && !item.inputs
                                    ? 'Loading…'
                                    : prettyIO(item.inputs ?? null)}
                                </pre>
                              </div>
                              <div className="flex flex-col gap-space-1">
                                <Text variant="sm" weight="medium" color="tertiary">
                                  Outputs
                                </Text>
                                <pre className="max-h-[300px] overflow-auto whitespace-pre-wrap break-words rounded-md bg-surface-level-2 p-space-2 font-mono text-xs text-secondary">
                                  {expandLoading && !item.outputs
                                    ? 'Loading…'
                                    : prettyIO(item.outputs ?? null)}
                                </pre>
                              </div>
                            </div>
                          )}
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
              {hasMore && (
                <tr>
                  <td colSpan={colSpan} className="p-space-3">
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
