import { Input } from '@langchain/macaw-components/Input';
import { useEffect, useRef, useState } from 'react';
import {
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@langchain/macaw-components/Select';
import { patchFeedback, submitFeedback, deleteFeedback } from '../api';
import type { FeedbackConfig, FeedbackItem, QueueItemType, RubricItem } from '../types';

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

  if (isCategorical) {
    return (
      <div title={error ?? undefined}>
        <SelectRoot
          value={score == null ? '' : String(score)}
          disabled={saving}
          onValueChange={(value) => {
            if (value === 'clear') {
              handleDelete();
              return;
            }
            const next = Number(value);
            setScore(next);
            const category = config!.categories!.find((category) => category.value === next);
            save(next, category?.label ?? String(next));
          }}
        >
          <SelectTrigger
            aria-label={item.feedback_key}
            aria-invalid={!!error}
            data-row-index={rowIndex}
            data-col-index={colIndex}
            className="w-full"
          >
            <SelectValue placeholder="—" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="clear">Clear score</SelectItem>
            {config!.categories!.map((category) => (
              <SelectItem key={category.value} value={String(category.value)}>
                {category.label ?? String(category.value)}
              </SelectItem>
            ))}
          </SelectContent>
        </SelectRoot>
      </div>
    );
  }

  if (isContinuous) {
    return (
      <Input
        size="sm"
        debounceMs={0}
        aria-label={item.feedback_key}
        isError={!!error}
        type="number"
        step="any"
        min={config?.min ?? undefined}
        max={config?.max ?? undefined}
        value={score == null ? '' : String(score)}
        disabled={saving}
        onChange={(value) => setScore(value === '' ? null : Number(value))}
        onBlur={() => {
          if (score == null && existingFeedback) handleDelete();
          else if (score != null) save(score, null);
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
        }}
        placeholder="—"
        title={error ?? undefined}
        data-row-index={rowIndex}
        data-col-index={colIndex}
        className="min-w-0 flex-1"
      />
    );
  }

  // Freeform
  return (
    <Input
      size="sm"
      debounceMs={0}
      aria-label={item.feedback_key}
      isError={!!error}
      type="text"
      value={comment}
      disabled={saving}
      onChange={(value) => setComment(value)}
      onBlur={() => {
        if (comment.trim()) save(null, null, comment);
        else if (existingFeedback) handleDelete();
      }}
      onKeyDown={(e) => {
        if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
      }}
      placeholder="—"
      title={error ?? undefined}
      data-row-index={rowIndex}
      data-col-index={colIndex}
      className="min-w-0 flex-1"
    />
  );
}
