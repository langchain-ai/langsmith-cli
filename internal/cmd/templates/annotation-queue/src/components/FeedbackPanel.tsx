import { IconButton } from '@langchain/macaw-components/IconButton';
import { Card } from '@langchain/macaw-components/Card';
import { Badge } from '@langchain/macaw-components/Badge';
import { Button } from '@langchain/macaw-components/Button';
import { Input } from '@langchain/macaw-components/Input';
import { Textarea } from '@langchain/macaw-components/Textarea';
import { EmptyState } from '@langchain/macaw-components/EmptyState';
import {
  CheckCircleFillIcon,
  CaretDownIcon,
  CaretRightIcon,
  PencilSimpleLineFillIcon,
  InfoRegularIcon,
  PlusIcon,
  XIcon,
} from '@langchain/macaw-components/icons';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ErrorBanner } from './ErrorBanner';
import { FeedbackChip } from './FeedbackChip';
import { ReviewerNotes } from './ReviewerNotes';
import { Spinner } from '@langchain/macaw-components/Spinner';
import type {
  AnnotationQueue,
  FeedbackConfig,
  FeedbackItem,
  QueueItemType,
  RubricItem,
} from '../types';
import {
  deleteFeedback,
  fetchFeedbackConfigs,
  fetchFeedbacksForRun,
  fetchFeedbacksForThread,
  patchFeedback,
  submitFeedback,
} from '../api';
import { cn } from '@langchain/macaw-components/utils/cn';

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

interface Props {
  queue: AnnotationQueue | null;
  itemType: QueueItemType | undefined;
  /** RUN subject id */
  runId: string | undefined;
  /** THREAD subject id (feedback_thread_id) */
  feedbackThreadId: string | undefined;
  traceId: string | undefined;
  sessionId: string | undefined;
  startTime: string | undefined;
  itemId: string | undefined;
  onComplete: () => void;
  completing: boolean;
  completeError: string | null;
  totalNeedsReview: number;
}

// ── Rubric item card ─────────────────────────────────────────────────────────

interface RubricCardProps {
  item: RubricItem;
  config: FeedbackConfig | undefined;
  existingFeedback: FeedbackItem | undefined;
  itemType: QueueItemType;
  runId: string | undefined;
  feedbackThreadId: string | undefined;
  traceId: string | undefined;
  sessionId: string | undefined;
  startTime: string | undefined;
  expanded: boolean;
  onToggleExpand: () => void;
  onFeedbackSaved: (feedback: FeedbackItem) => void;
  onFeedbackDeleted: (feedbackKey: string) => void;
}

function RubricCard({
  item,
  config,
  existingFeedback,
  itemType,
  runId,
  feedbackThreadId,
  traceId,
  sessionId,
  startTime,
  expanded,
  onToggleExpand,
  onFeedbackSaved,
  onFeedbackDeleted,
}: RubricCardProps) {
  const [score, setScore] = useState<number | null>(existingFeedback?.score ?? null);
  const [comment, setComment] = useState<string>(existingFeedback?.comment ?? '');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Sync when run changes (existingFeedback.id changes)
  const prevFeedbackId = useRef<string | undefined>(existingFeedback?.id);
  useEffect(() => {
    if (existingFeedback?.id !== prevFeedbackId.current) {
      prevFeedbackId.current = existingFeedback?.id;
      setScore(existingFeedback?.score ?? null);
      setComment(existingFeedback?.comment ?? '');
      setError(null);
    }
  });

  // Type comes from the feedback config, not the rubric item — a key with no
  // config yet (e.g. a brand-new ad-hoc key) defaults to freeform, same as
  // LangSmith's own behavior for unconfigured keys.
  const configType = config?.type ?? 'freeform';
  const isCategorical = configType === 'categorical' && !!config?.categories?.length;
  const isContinuous = configType === 'continuous';
  const isFreeform = !isCategorical && !isContinuous;

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
      onFeedbackSaved(saved);
    } catch (e) {
      console.error('Failed to save feedback', e);
      setError(errorMessage(e));
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
      onFeedbackDeleted(item.feedback_key);
    } catch (e) {
      console.error('Failed to delete feedback', e);
      setError(errorMessage(e));
    } finally {
      setSaving(false);
    }
  }

  const isFilled = existingFeedback != null;
  const feedbackScore = existingFeedback?.score;
  const feedbackValue = existingFeedback?.value;

  return (
    <Card className="flex flex-col gap-space-2">
      {/* Header row */}
      <Button
        color="secondary"
        variant="plain"
        size="sm"
        aria-label={item.feedback_key}
        aria-expanded={expanded}
        type="button"
        className="flex w-full items-center gap-2 border-0 px-0 text-left"
        onClick={onToggleExpand}
      >
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <FeedbackChip
            feedbackKey={item.feedback_key}
            score={feedbackScore}
            value={feedbackValue}
            isLoading={saving}
          />
          {isFilled && !saving && (
            <CheckCircleFillIcon className="h-4 w-4 shrink-0 text-success-primary" />
          )}
          {item.is_required && <Badge color="warning">Required</Badge>}
        </div>
        {expanded ? (
          <CaretDownIcon className="h-4 w-4 shrink-0 text-tertiary" />
        ) : (
          <CaretRightIcon className="h-4 w-4 shrink-0 text-tertiary" />
        )}
      </Button>

      {expanded && (
        <div className="flex flex-col gap-2">
          {item.description && <p className="text-sm text-quaternary">{item.description}</p>}

          {error && <ErrorBanner error={error} />}

          {/* Category options */}
          {isCategorical && (
            <div className="flex flex-col gap-2">
              {config!.categories!.map((cat) => {
                const label = cat.label ?? String(cat.value);
                const isSelected = score === cat.value;
                // value_descriptions is keyed by label when categories have a
                // string label; score_descriptions is keyed by the stringified
                // numeric value otherwise (mirrors AnnotationQueueRubricForm).
                const description = cat.label
                  ? (item.value_descriptions?.[cat.label] ?? undefined)
                  : (item.score_descriptions?.[String(cat.value)] ?? undefined);
                return (
                  <Button
                    color="secondary"
                    variant="outlined"
                    size="sm"
                    key={cat.value}
                    aria-label={label}
                    aria-pressed={isSelected}
                    type="button"
                    className={cn(
                      'h-auto w-full flex-col items-start gap-space-1 whitespace-normal py-space-3 text-left',
                      isSelected && 'bg-selected'
                    )}
                    onClick={() => {
                      if (isSelected) {
                        setScore(null);
                        handleDelete();
                      } else {
                        setScore(cat.value);
                        save(cat.value, label);
                      }
                    }}
                    disabled={saving}
                  >
                    <span className={cn('text-sm font-medium', saving && 'opacity-0')}>
                      {label}
                    </span>
                    {description && (
                      <span className="line-clamp-2 whitespace-normal break-words text-sm text-tertiary">
                        {description}
                      </span>
                    )}
                  </Button>
                );
              })}
            </div>
          )}

          {/* Numeric input */}
          {isContinuous && (
            <div className="flex flex-col gap-2">
              {(config?.min != null || config?.max != null) && (
                <div className="text-xs text-quaternary">
                  {config?.min != null && config?.max != null
                    ? `Min: ${config.min}, Max: ${config.max}`
                    : config?.min != null
                      ? `Min: ${config.min}`
                      : `Max: ${config?.max}`}
                </div>
              )}
              <div className="flex gap-2">
                <Input
                  size="sm"
                  debounceMs={0}
                  aria-label={item.feedback_key}
                  type="number"
                  step="any"
                  min={config?.min ?? undefined}
                  max={config?.max ?? undefined}
                  value={score == null ? '' : String(score)}
                  onChange={(value) => {
                    const val = value === '' ? null : Number(value);
                    setScore(val);
                  }}
                  onBlur={() => save(score, null)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
                  }}
                  placeholder="Enter score"
                  className="min-w-0 flex-1"
                />
                {existingFeedback && (
                  <IconButton
                    color="secondary"
                    variant="plain"
                    icon={XIcon}
                    label="Clear feedback"
                    className="self-start"
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={handleDelete}
                  />
                )}
              </div>
            </div>
          )}

          {/* Freeform text */}
          {isFreeform && (
            <div className="flex gap-2">
              <Textarea
                size="sm"
                debounceMs={0}
                aria-label={item.feedback_key}
                value={comment}
                onChange={(value) => setComment(value)}
                onBlur={() => {
                  if (comment.trim()) save(null, null, comment);
                  else if (existingFeedback) handleDelete();
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                    (e.target as HTMLTextAreaElement).blur();
                  }
                }}
                placeholder="Enter text"
                rows={3}
                className="min-w-0 flex-1"
              />
              {existingFeedback && (
                <IconButton
                  color="secondary"
                  variant="plain"
                  icon={XIcon}
                  label="Clear feedback"
                  className="self-start"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={handleDelete}
                />
              )}
            </div>
          )}
        </div>
      )}
    </Card>
  );
}

// ── FeedbackPanel ─────────────────────────────────────────────────────────────

export function FeedbackPanel({
  queue,
  itemType,
  runId,
  feedbackThreadId,
  traceId,
  sessionId,
  startTime,
  itemId,
  onComplete,
  completing,
  completeError,
  totalNeedsReview,
}: Props) {
  const [feedbackMap, setFeedbackMap] = useState<Record<string, FeedbackItem>>({});
  const [feedbackLoading, setFeedbackLoading] = useState(false);
  const [feedbackConfigs, setFeedbackConfigs] = useState<Record<string, FeedbackConfig>>({});
  const [expandedItems, setExpandedItems] = useState<boolean[]>([]);
  // Ad-hoc feedback keys added via the "+ Add" button, on top of the queue's rubric
  const [adhocKeys, setAdhocKeys] = useState<string[]>([]);
  const [addingKey, setAddingKey] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');

  const allRubricItems: RubricItem[] = [
    ...(queue?.rubric_items ?? []),
    ...adhocKeys.map(
      (key): RubricItem => ({
        feedback_key: key,
        description: null,
        value_descriptions: null,
        score_descriptions: null,
      })
    ),
  ];
  // Assertion-flagged rubric items belong to dataset-curation mode, not the
  // Feedback list — mirrors AnnotationQueueRubricForm's filteredQueue.
  const visibleRubricItems = allRubricItems.filter((item) => !item.is_assertion);

  // Initialize expanded state when queue loads. Sized to the *visible*
  // (non-assertion) items, since that's what expandedItems is indexed against
  // everywhere else.
  useEffect(() => {
    if (queue?.id) {
      const visibleCount = (queue.rubric_items ?? []).filter((item) => !item.is_assertion).length;
      setExpandedItems(Array(visibleCount).fill(true));
      setAdhocKeys([]);
    }
  }, [queue?.id, queue?.rubric_items]);

  // Fetch the type/min/max/categories config for each visible key — the
  // rubric item itself carries no type info (see RubricItem in types.ts).
  const rubricKeys = useMemo(
    () => visibleRubricItems.map((item) => item.feedback_key),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [visibleRubricItems.map((item) => item.feedback_key).join(',')]
  );
  useEffect(() => {
    if (rubricKeys.length === 0) {
      setFeedbackConfigs({});
      return;
    }
    fetchFeedbackConfigs(rubricKeys)
      .then((configs) => {
        const map: Record<string, FeedbackConfig> = {};
        for (const c of configs) map[c.feedback_key] = c.feedback_config;
        setFeedbackConfigs(map);
      })
      .catch((e) => console.error('Failed to load feedback configs', e));
  }, [rubricKeys]);

  function handleAddKey() {
    const key = newKeyName.trim();
    if (key && !allRubricItems.some((item) => item.feedback_key === key)) {
      setAdhocKeys((prev) => [...prev, key]);
      setExpandedItems((prev) => [...prev, true]);
    }
    setNewKeyName('');
    setAddingKey(false);
  }

  // Reload feedbacks when the selected item's subject changes.
  useEffect(() => {
    const isThread = itemType === 'THREAD';
    const subject = isThread ? feedbackThreadId : runId;
    if (!subject) {
      setFeedbackMap({});
      return;
    }
    setFeedbackLoading(true);
    const load = isThread
      ? fetchFeedbacksForThread(subject, sessionId)
      : fetchFeedbacksForRun(subject);
    load
      .then((items) => {
        const map: Record<string, FeedbackItem> = {};
        for (const item of items) map[item.key] = item;
        setFeedbackMap(map);
      })
      .catch((e) => console.error('Failed to load feedbacks', e))
      .finally(() => setFeedbackLoading(false));
  }, [itemType, runId, feedbackThreadId, sessionId]);

  const handleFeedbackSaved = useCallback(
    (feedback: FeedbackItem, index: number) => {
      setFeedbackMap((prev) => ({ ...prev, [feedback.key]: feedback }));
      // Auto-advance: open next unfilled item
      setExpandedItems((prev) => {
        const next = [...prev];
        for (let i = index + 1; i < visibleRubricItems.length; i++) {
          if (
            !feedbackMap[visibleRubricItems[i].feedback_key] &&
            feedback.key !== visibleRubricItems[i].feedback_key
          ) {
            next[i] = true;
            break;
          }
        }
        return next;
      });
    },
    [visibleRubricItems, feedbackMap]
  );

  const handleFeedbackDeleted = useCallback((feedbackKey: string) => {
    setFeedbackMap((prev) => {
      const next = { ...prev };
      delete next[feedbackKey];
      return next;
    });
  }, []);

  const hasRubricItems = visibleRubricItems.length > 0;
  const allRequiredFilled = visibleRubricItems
    .filter((item) => item.is_required)
    .every((item) => feedbackMap[item.feedback_key] != null);

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {/* Scrollable content */}
      <div className="flex-1 overflow-auto px-4 py-4">
        <div className="flex flex-col gap-10">
          {/* Instructions section */}
          <div className="flex flex-col gap-2">
            <div className="text-base font-medium text-primary">Instructions</div>
            {queue?.rubric_instructions ? (
              <div className="text-sm text-secondary">{queue.rubric_instructions}</div>
            ) : (
              <EmptyState
                size="sm"
                icon={InfoRegularIcon}
                title="No instructions yet"
                description="Contact your administrator to create a clear annotation rubric."
              />
            )}
          </div>

          {/* Feedback section */}
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div className="text-base font-medium text-primary">Feedback</div>
                {feedbackLoading && <Spinner size="md" className="opacity-50" />}
              </div>
              {queue && (
                <Button
                  color="secondary"
                  variant="outlined"
                  size="sm"
                  aria-label="Add feedback key"
                  type="button"
                  onClick={() => setAddingKey(true)}
                  className="items-center gap-1"
                >
                  <PlusIcon className="h-3.5 w-3.5" />
                  Add
                </Button>
              )}
            </div>

            {addingKey && (
              <Input
                size="sm"
                debounceMs={0}
                aria-label="Feedback key"
                autoFocus
                value={newKeyName}
                onChange={(value) => setNewKeyName(value)}
                onBlur={handleAddKey}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
                  if (e.key === 'Escape') {
                    setNewKeyName('');
                    setAddingKey(false);
                  }
                }}
                placeholder="Feedback key"
                className="min-w-0 flex-1"
              />
            )}

            {!queue ? (
              <div
                role="status"
                className="flex items-center justify-center gap-space-2 py-space-8"
              >
                <Spinner size="sm" />
                <span className="text-sm text-tertiary">Loading rubric…</span>
              </div>
            ) : !hasRubricItems ? (
              <EmptyState
                size="sm"
                icon={PencilSimpleLineFillIcon}
                title="No feedback rubrics yet"
                description="Add an existing rubric or set up one for future use."
              />
            ) : (
              <div className="flex flex-col gap-3">
                {visibleRubricItems.map((item, idx) => (
                  <RubricCard
                    key={item.feedback_key}
                    item={item}
                    config={feedbackConfigs[item.feedback_key]}
                    existingFeedback={feedbackMap[item.feedback_key]}
                    itemType={itemType ?? 'RUN'}
                    runId={runId}
                    feedbackThreadId={feedbackThreadId}
                    traceId={traceId}
                    sessionId={sessionId}
                    startTime={startTime}
                    expanded={expandedItems[idx] ?? true}
                    onToggleExpand={() =>
                      setExpandedItems((prev) => {
                        const next = [...prev];
                        next[idx] = !next[idx];
                        return next;
                      })
                    }
                    onFeedbackSaved={(fb) => handleFeedbackSaved(fb, idx)}
                    onFeedbackDeleted={handleFeedbackDeleted}
                  />
                ))}
              </div>
            )}
          </div>

          {/* Reviewer notes — RUN only; product AQ hides notes for THREAD. */}
          {itemType !== 'THREAD' && (
            <ReviewerNotes
              runId={runId}
              traceId={traceId}
              sessionId={sessionId}
              startTime={startTime}
            />
          )}
        </div>
      </div>

      {/* Sticky footer */}
      {itemId && (itemType === 'THREAD' ? feedbackThreadId : runId) && (
        <div className="sticky bottom-0 border-t border-secondary bg-primary px-4 pb-20 pt-4">
          {completeError && <ErrorBanner error={completeError} />}
          <Button
            color="primary"
            variant="normal"
            size="md"
            aria-label={completing ? 'Saving review' : totalNeedsReview > 1 ? 'Next' : 'Done'}
            type="button"
            className="flex w-full items-center justify-center gap-2"
            onClick={onComplete}
            disabled={completing || !allRequiredFilled}
            title={!allRequiredFilled ? 'Fill in all required rubric items' : undefined}
          >
            {completing ? (
              'Saving…'
            ) : (
              <>
                {totalNeedsReview > 1 ? 'Next' : 'Done'}
                <span className="flex items-center gap-0.5 text-xs opacity-70">
                  <span>⌘</span>
                  <span>↵</span>
                </span>
              </>
            )}
          </Button>
        </div>
      )}
    </div>
  );
}
