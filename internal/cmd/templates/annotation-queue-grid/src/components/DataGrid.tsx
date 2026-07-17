import { GridCell } from './GridCell';
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
  feedbackByRun: Record<string, Record<string, FeedbackItem>>;
  activeRow: number;
  /** Count of assertion-flagged rubric items excluded from the grid columns. */
  assertionCount: number;
  onActivateRow: (index: number) => void;
  onCellSaved: (runId: string, feedback: FeedbackItem) => void;
  onCellDeleted: (runId: string, feedbackKey: string) => void;
  onComplete: (index: number) => void;
}

// A short preview of a run's inputs so each row is identifiable without a
// dedicated detail pane.
function runPreview(run: AnnotationQueueRun): string {
  if (run.name) return run.name;
  if (run.inputs) {
    try {
      const s = JSON.stringify(run.inputs);
      return s.length > 80 ? s.slice(0, 80) + '…' : s;
    } catch {
      /* fall through */
    }
  }
  return run.id.slice(0, 8);
}

export function DataGrid({
  queue,
  columns,
  configs,
  rows,
  rowsLoading,
  feedbackByRun,
  activeRow,
  assertionCount,
  onActivateRow,
  onCellSaved,
  onCellDeleted,
  onComplete,
}: Props) {
  if (!queue) {
    return (
      <div className="flex flex-1 items-center justify-center rounded-lg border border-secondary">
        <span className="text-sm text-tertiary">Loading queue…</span>
      </div>
    );
  }

  // A row's required (non-assertion) columns must all have feedback before it
  // can be marked Done — mirrors FeedbackPanel's allRequiredFilled gate.
  function requiredFilled(runId: string): boolean {
    const rowFeedback = feedbackByRun[runId] ?? {};
    return columns.filter((c) => c.is_required).every((c) => rowFeedback[c.feedback_key] != null);
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-secondary">
      {/* Header bar */}
      <div className="flex items-center justify-between border-b border-secondary px-4 py-3">
        <div className="flex items-baseline gap-2">
          <span className="text-base font-medium text-primary">{queue.name}</span>
          <span className="text-sm text-tertiary">
            {rows.length} to review · ↑/↓ to move between rows
          </span>
        </div>
        {assertionCount > 0 && (
          <span className="text-xs text-quaternary">
            {assertionCount} assertion rubric item{assertionCount === 1 ? '' : 's'} not shown (managed separately)
          </span>
        )}
      </div>

      {/* Grid */}
      <div className="min-h-0 flex-1 overflow-auto">
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
                <th className="w-[280px] min-w-[280px] border-b border-secondary px-3 py-2 text-xs font-medium text-tertiary">
                  Run
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
                <th className="w-[100px] border-b border-l border-secondary px-3 py-2 text-xs font-medium text-tertiary">
                  {/* Done column */}
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((run, index) => {
                const isActive = index === activeRow;
                const rowFeedback = feedbackByRun[run.id] ?? {};
                return (
                  <tr
                    key={run.queue_run_id}
                    onClick={() => onActivateRow(index)}
                    className={cn(
                      'border-b border-secondary',
                      isActive ? 'bg-selected' : 'hover:bg-surface-level-1-hover'
                    )}
                  >
                    <td className="max-w-[280px] truncate px-3 py-1.5 align-middle text-sm text-secondary" title={runPreview(run)}>
                      {runPreview(run)}
                    </td>
                    {columns.map((col) => (
                      <td key={col.feedback_key} className="border-l border-secondary px-1 py-1 align-middle">
                        <GridCell
                          item={col}
                          config={configs[col.feedback_key]}
                          runId={run.id}
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
                        Done
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
