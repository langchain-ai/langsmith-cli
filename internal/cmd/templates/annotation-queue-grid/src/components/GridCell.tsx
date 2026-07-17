import { useEffect, useRef, useState } from 'react';
import { patchFeedback, submitFeedback, deleteFeedback } from '../api';
import type { FeedbackConfig, FeedbackItem, RubricItem } from '../types';
import { cn } from '../lib/utils';

interface Props {
  item: RubricItem;
  config: FeedbackConfig | undefined;
  runId: string;
  existingFeedback: FeedbackItem | undefined;
  onSaved: (feedback: FeedbackItem) => void;
  onDeleted: (feedbackKey: string) => void;
}

// One editable cell. Saves as-you-go via the same submit/patch/delete calls the 3-pane FeedbackPanel uses.
export function GridCell({ item, config, runId, existingFeedback, onSaved, onDeleted }: Props) {
  const [score, setScore] = useState<number | null>(existingFeedback?.score ?? null);
  const [comment, setComment] = useState<string>(existingFeedback?.comment ?? '');
  const [saving, setSaving] = useState(false);

  // Resync when the underlying feedback identity changes (row reload / edit
  // elsewhere), mirroring RubricCard's prevFeedbackId guard.
  const prevFeedbackId = useRef<string | undefined>(existingFeedback?.id);
  useEffect(() => {
    if (existingFeedback?.id !== prevFeedbackId.current) {
      prevFeedbackId.current = existingFeedback?.id;
      setScore(existingFeedback?.score ?? null);
      setComment(existingFeedback?.comment ?? '');
    }
  });

  // Editor type comes from the feedback config (freeform when unconfigured,
  // like LangSmith). Non-categorical, non-continuous keys fall through to freeform.
  const configType = config?.type ?? 'freeform';
  const isCategorical = configType === 'categorical' && !!config?.categories?.length;
  const isContinuous = configType === 'continuous';

  async function save(newScore: number | null, newValue: string | null, newComment?: string) {
    setSaving(true);
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
        });
      }
      onSaved(saved);
    } catch (e) {
      console.error('Failed to save feedback', e);
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!existingFeedback) return;
    setSaving(true);
    try {
      await deleteFeedback(existingFeedback.id);
      setScore(null);
      setComment('');
      onDeleted(item.feedback_key);
    } catch (e) {
      console.error('Failed to delete feedback', e);
    } finally {
      setSaving(false);
    }
  }

  const cellInput =
    'w-full rounded border border-transparent bg-transparent px-2 py-1 text-sm text-primary hover:border-secondary focus:border-brand focus:bg-primary focus:outline-none';

  if (isCategorical) {
    return (
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
        className={cn(cellInput, 'cursor-pointer', saving && 'opacity-50')}
      >
        <option value="">—</option>
        {config!.categories!.map((cat) => (
          <option key={cat.value} value={cat.value}>
            {cat.label ?? String(cat.value)}
          </option>
        ))}
      </select>
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
      className={cn(cellInput, saving && 'opacity-50')}
    />
  );
}
