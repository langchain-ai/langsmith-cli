import { useEffect, useRef, useState } from 'react';
import { ChevronDownIcon } from '@langchain/untitled-ui-icons';
// The container/element split the design system's <Input> is built from. A
// spreadsheet cell can't be an <Input>: DataGrid moves focus between cells by
// querying for the focusable element that carries the row/col data attributes,
// and every cell has to stay a plain focusable control. Borrowing the styling
// helpers keeps these cells pixel-identical to a real Input anyway.
import {
  getInputContainerClasses,
  getInputElementClasses,
} from '@/components/langsmith/design-system/components/Input/inputStyles';
import { patchFeedback, submitFeedback, deleteFeedback } from '../api';
import type { FeedbackConfig, FeedbackItem, QueueItemType, RubricItem } from '../types';
import { cn } from '../lib/utils';

interface Props {
  item: RubricItem;
  config: FeedbackConfig | undefined;
  itemType: QueueItemType;
  runId: string | undefined;
  feedbackThreadId: string | undefined;
  traceId: string | undefined;
  sessionId: string | undefined;
  startTime: string | undefined;
  existingFeedback: FeedbackItem | undefined;
  /** Grid coordinates, stamped onto the focusable element as data
   * attributes so DataGrid's arrow-key handler can find neighboring cells
   * via the DOM without every cell needing to know about its siblings. */
  rowIndex: number;
  colIndex: number;
  onSaved: (feedback: FeedbackItem) => void;
  onDeleted: (feedbackKey: string) => void;
}

// One editable cell. Saves as-you-go via the same submit/patch/delete calls the 3-pane FeedbackPanel uses.
export function GridCell({
  item,
  config,
  itemType,
  runId,
  feedbackThreadId,
  traceId,
  sessionId,
  startTime,
  existingFeedback,
  rowIndex,
  colIndex,
  onSaved,
  onDeleted,
}: Props) {
  const [score, setScore] = useState<number | null>(existingFeedback?.score ?? null);
  const [comment, setComment] = useState<string>(existingFeedback?.comment ?? '');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Resync when the underlying feedback identity changes (row reload / edit
  // elsewhere), mirroring RubricCard's prevFeedbackId guard.
  const prevFeedbackId = useRef<string | undefined>(existingFeedback?.id);
  useEffect(() => {
    if (existingFeedback?.id !== prevFeedbackId.current) {
      prevFeedbackId.current = existingFeedback?.id;
      setScore(existingFeedback?.score ?? null);
      setComment(existingFeedback?.comment ?? '');
      setError(null);
    }
  });

  // Editor type comes from the feedback config (freeform when unconfigured,
  // like LangSmith). Non-categorical, non-continuous keys fall through to freeform.
  const configType = config?.type ?? 'freeform';
  const isCategorical = configType === 'categorical' && !!config?.categories?.length;
  const isContinuous = configType === 'continuous';

  async function save(newScore: number | null, newValue: string | null, newComment?: string) {
    setSaving(true);
    setError(null);
    try {
      const commentVal = newComment !== undefined ? newComment : comment;
      let saved: FeedbackItem;
      if (existingFeedback) {
        saved = await patchFeedback(existingFeedback.id, {
          score: newScore,
          value: newValue,
          comment: commentVal || null,
        });
      } else if (itemType === 'THREAD' && feedbackThreadId) {
        saved = await submitFeedback({
          key: item.feedback_key,
          feedback_thread_id: feedbackThreadId,
          score: newScore,
          value: newValue ?? undefined,
          comment: commentVal || undefined,
          session_id: sessionId,
        });
      } else {
        saved = await submitFeedback({
          key: item.feedback_key,
          run_id: runId,
          score: newScore,
          value: newValue ?? undefined,
          comment: commentVal || undefined,
          trace_id: traceId,
          session_id: sessionId,
          start_time: startTime,
        });
      }
      onSaved(saved);
    } catch (e) {
      console.error('Failed to save feedback', e);
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!existingFeedback) return;
    setSaving(true);
    setError(null);
    try {
      await deleteFeedback(existingFeedback.id);
      setScore(null);
      setComment('');
      onDeleted(item.feedback_key);
    } catch (e) {
      console.error('Failed to delete feedback', e);
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  // Deliberately spreadsheet-like: a persistent visible box (not a
  // hover-only/transparent border) so every fillable cell reads as
  // "editable" at rest, with the design system's focus ring standing in for a
  // selected spreadsheet cell.
  const containerClass = cn(
    getInputContainerClasses({ size: 'sm', variant: 'outlined', isError: !!error, disabled: saving }),
    'bg-surface-level-1',
    saving && 'opacity-50'
  );
  const elementClass = getInputElementClasses({ size: 'sm', disabled: saving, className: 'text-primary' });

  if (isCategorical) {
    return (
      <div className={cn(containerClass, 'relative')} title={error ?? undefined}>
        <select
          value={score ?? ''}
          disabled={saving}
          onChange={(e) => {
            if (e.target.value === '') {
              setScore(null);
              handleDelete();
              return;
            }
            const val = Number(e.target.value);
            const cat = config!.categories!.find((c) => c.value === val);
            setScore(val);
            save(val, cat?.label ?? String(val));
          }}
          data-row-index={rowIndex}
          data-col-index={colIndex}
          // bg-none drops the arrow @tailwindcss/forms paints onto every
          // <select>; appearance-none alone doesn't remove a background image,
          // and it would sit next to the chevron below.
          className={cn(elementClass, 'cursor-pointer appearance-none bg-none pr-space-4')}
        >
          <option value="">—</option>
          {config!.categories!.map((cat) => (
            <option key={cat.value} value={cat.value}>
              {cat.label ?? String(cat.value)}
            </option>
          ))}
        </select>
        <ChevronDownIcon className="pointer-events-none absolute right-1.5 top-1/2 size-3.5 -translate-y-1/2 text-icon-tertiary" />
      </div>
    );
  }

  if (isContinuous) {
    return (
      <div className={containerClass} title={error ?? undefined}>
        <input
          type="number"
          step="any"
          min={config?.min ?? undefined}
          max={config?.max ?? undefined}
          value={score ?? ''}
          disabled={saving}
          onChange={(e) => setScore(e.target.value === '' ? null : Number(e.target.value))}
          onBlur={() => {
            if (score == null && existingFeedback) handleDelete();
            else if (score != null) save(score, null);
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
          }}
          placeholder="—"
          data-row-index={rowIndex}
          data-col-index={colIndex}
          className={elementClass}
        />
      </div>
    );
  }

  // Freeform
  return (
    <div className={containerClass} title={error ?? undefined}>
      <input
        type="text"
        value={comment}
        disabled={saving}
        onChange={(e) => setComment(e.target.value)}
        onBlur={() => {
          if (comment.trim()) save(null, null, comment);
          else if (existingFeedback) handleDelete();
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
        }}
        placeholder="—"
        data-row-index={rowIndex}
        data-col-index={colIndex}
        className={elementClass}
      />
    </div>
  );
}
