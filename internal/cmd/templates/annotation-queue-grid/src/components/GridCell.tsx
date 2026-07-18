import { useEffect, useRef, useState } from 'react';
import { ChevronDownIcon } from '@langchain/untitled-ui-icons';
import { patchFeedback, submitFeedback, deleteFeedback } from '../api';
import type { FeedbackConfig, FeedbackItem, RubricItem } from '../types';
import { cn } from '../lib/utils';

interface Props {
  item: RubricItem;
  config: FeedbackConfig | undefined;
  runId: string;
  traceId: string | undefined;
  sessionId: string | undefined;
  startTime: string | undefined;
  existingFeedback: FeedbackItem | undefined;
  onSaved: (feedback: FeedbackItem) => void;
  onDeleted: (feedbackKey: string) => void;
}

// One editable cell. Saves as-you-go via the same submit/patch/delete calls the 3-pane FeedbackPanel uses.
export function GridCell({
  item,
  config,
  runId,
  traceId,
  sessionId,
  startTime,
  existingFeedback,
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

  const cellInput = cn(
    'w-full rounded border border-transparent bg-transparent px-2 py-1 text-sm text-primary hover:border-secondary focus:border-brand focus:bg-primary focus:outline-none',
    error && 'border-error-strong'
  );

  if (isCategorical) {
    return (
      <div className="relative">
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
          title={error ?? undefined}
          className={cn(cellInput, 'appearance-none pr-6', 'cursor-pointer', saving && 'opacity-50')}
        >
          <option value="">—</option>
          {config!.categories!.map((cat) => (
            <option key={cat.value} value={cat.value}>
              {cat.label ?? String(cat.value)}
            </option>
          ))}
        </select>
        <ChevronDownIcon className="pointer-events-none absolute right-1.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-tertiary" />
      </div>
    );
  }

  if (isContinuous) {
    return (
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
        title={error ?? undefined}
        className={cn(cellInput, saving && 'opacity-50')}
      />
    );
  }

  // Freeform
  return (
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
      title={error ?? undefined}
      className={cn(cellInput, saving && 'opacity-50')}
    />
  );
}
