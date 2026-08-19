import {
  CheckCircleBrokenIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  Edit03Icon,
  InfoCircleIcon,
  PlusIcon,
  XIcon,
} from '@langchain/untitled-ui-icons';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Badge } from '@/components/langsmith/design-system/components/Badge';
import { Banner } from '@/components/langsmith/design-system/components/Banner';
import { Button } from '@/components/langsmith/design-system/components/Button';
import { Icon } from '@/components/langsmith/design-system/components/Icon';
import { IconButton } from '@/components/langsmith/design-system/components/IconButton';
import { Input } from '@/components/langsmith/design-system/components/Input';
import { Kbd, KbdGroup } from '@/components/langsmith/design-system/components/Kbd';
import { Spinner } from '@/components/langsmith/design-system/components/Spinner';
import { Text } from '@/components/langsmith/design-system/components/Text';
import { Textarea } from '@/components/langsmith/design-system/components/Textarea';
import { FeedbackChip } from './FeedbackChip';
import { ReviewerNotes } from './ReviewerNotes';
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
import { cn } from '../lib/utils';

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

// ── Empty state ──────────────────────────────────────────────────────────────

// Hand-composed from <Icon> + <Text> rather than the design system's
// <EmptyState>: that component's optional call-to-action is a <Link>, which
// imports react-router-dom — a router this single-view sandboxed app has no
// other use for.
function PanelEmptyState({
  icon,
  title,
  body,
}: {
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>;
  title: string;
  body: string;
}) {
  return (
    <div className="flex flex-col items-center gap-space-3 rounded-xl bg-surface-level-2 px-space-5 py-space-7">
      <Icon icon={icon} size="md" color="brand" rounded />
      <div className="flex flex-col gap-space-1 text-center">
        <Text variant="md" weight="semibold" color="secondary">
          {title}
        </Text>
        <Text variant="md" color="tertiary">
          {body}
        </Text>
      </div>
    </div>
  );
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
    <div className="flex flex-col gap-space-2 rounded-lg border border-default p-space-3">
      {/* Header row */}
      <button
        type="button"
        className="flex w-full items-center gap-space-2 text-left"
        onClick={onToggleExpand}
      >
        <div className="flex min-w-0 flex-1 items-center gap-space-2">
          <FeedbackChip
            feedbackKey={item.feedback_key}
            score={feedbackScore}
            value={feedbackValue}
            isLoading={saving}
          />
          {isFilled && !saving && (
            <CheckCircleBrokenIcon className="size-4 shrink-0 text-icon-success" />
          )}
          {item.is_required && (
            <Badge color="warning" size="xs" rounded="xs">
              Required
            </Badge>
          )}
        </div>
        {expanded ? (
          <ChevronDownIcon className="size-4 shrink-0 text-icon-tertiary" />
        ) : (
          <ChevronRightIcon className="size-4 shrink-0 text-icon-tertiary" />
        )}
      </button>

      {expanded && (
        <div className="flex flex-col gap-space-2">
          {item.description && (
            <Text variant="md" color="quaternary">
              {item.description}
            </Text>
          )}

          {error && (
            <Banner intent="error" title="Couldn't save">
              {error}
            </Banner>
          )}

          {/* Category options. Deliberately buttons rather than <RadioCard>:
              clicking the selected option again clears the feedback, which a
              radio group has no way to express. */}
          {isCategorical && (
            <div className="flex flex-col gap-space-2">
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
                  <button
                    key={cat.value}
                    type="button"
                    className={cn(
                      'flex w-full cursor-pointer flex-col gap-space-1 rounded-md border p-space-3 text-left transition-colors duration-fast',
                      isSelected
                        ? 'border-brand bg-brand-muted'
                        : 'border-default hover:bg-surface-level-2'
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
                    <Text
                      as="span"
                      variant="md"
                      weight="medium"
                      className={cn(saving && 'opacity-0')}
                    >
                      {label}
                    </Text>
                    {description && (
                      <Text
                        as="span"
                        variant="md"
                        color="tertiary"
                        className="line-clamp-2 whitespace-normal break-words"
                      >
                        {description}
                      </Text>
                    )}
                  </button>
                );
              })}
            </div>
          )}

          {/* Numeric input */}
          {isContinuous && (
            <div className="flex flex-col gap-space-2">
              {(config?.min != null || config?.max != null) && (
                <Text variant="sm" color="quaternary">
                  {config?.min != null && config?.max != null
                    ? `Min: ${config.min}, Max: ${config.max}`
                    : config?.min != null
                      ? `Min: ${config.min}`
                      : `Max: ${config?.max}`}
                </Text>
              )}
              <div className="flex gap-space-2">
                <Input
                  type="number"
                  step="any"
                  min={config?.min ?? undefined}
                  max={config?.max ?? undefined}
                  className="min-w-0 flex-1"
                  value={score == null ? '' : String(score)}
                  debounceMs={0}
                  onChange={(value) => setScore(value === '' ? null : Number(value))}
                  onBlur={() => save(score, null)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
                  }}
                  placeholder="Enter score"
                />
                {existingFeedback && (
                  <ClearButton onClear={handleDelete} label="Clear score" />
                )}
              </div>
            </div>
          )}

          {/* Freeform text */}
          {isFreeform && (
            <div className="flex gap-space-2">
              <Textarea
                className="flex-1"
                value={comment}
                debounceMs={0}
                onChange={setComment}
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
                resize="none"
              />
              {existingFeedback && <ClearButton onClear={handleDelete} label="Clear note" />}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// onMouseDown preventDefault keeps the click from blurring the field first,
// which would fire the field's own save on the way out.
function ClearButton({ onClear, label }: { onClear: () => void; label: string }) {
  return (
    <IconButton
      size="xs"
      color="secondary"
      variant="plain"
      icon={XIcon}
      label={label}
      className="shrink-0 self-start"
      onMouseDown={(e) => e.preventDefault()}
      onClick={onClear}
    />
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
      <div className="flex-1 overflow-auto px-space-4 py-space-4">
        <div className="flex flex-col gap-space-7">
          {/* Instructions section */}
          <div className="flex flex-col gap-space-2">
            <Text variant="h3">Instructions</Text>
            {queue?.rubric_instructions ? (
              <Text variant="md" color="secondary">
                {queue.rubric_instructions}
              </Text>
            ) : (
              <PanelEmptyState
                icon={InfoCircleIcon}
                title="No instructions yet"
                body="Contact your administrator to create a clear annotation rubric."
              />
            )}
          </div>

          {/* Feedback section */}
          <div className="flex flex-col gap-space-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-space-2">
                <Text variant="h3">Feedback</Text>
                {feedbackLoading && <Spinner size="sm" className="text-icon-tertiary" />}
              </div>
              {queue && (
                <Button
                  size="xs"
                  color="secondary"
                  variant="outlined"
                  leftDecorator={PlusIcon}
                  onClick={() => setAddingKey(true)}
                >
                  Add
                </Button>
              )}
            </div>

            {addingKey && (
              <Input
                autoFocus
                value={newKeyName}
                debounceMs={0}
                onChange={setNewKeyName}
                onBlur={handleAddKey}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
                  if (e.key === 'Escape') {
                    setNewKeyName('');
                    setAddingKey(false);
                  }
                }}
                placeholder="Feedback key"
              />
            )}

            {!queue ? (
              <div className="flex items-center justify-center py-space-6">
                <Text variant="md" color="tertiary">
                  Loading rubric…
                </Text>
              </div>
            ) : !hasRubricItems ? (
              <PanelEmptyState
                icon={Edit03Icon}
                title="No feedback rubrics yet"
                body="Add an existing rubric or set up one for future use."
              />
            ) : (
              <div className="flex flex-col gap-space-3">
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
        <div className="sticky bottom-0 flex flex-col gap-space-2 border-t border-default bg-surface-level-1 px-space-4 pb-20 pt-space-4">
          {completeError && (
            <Banner intent="error" title="Couldn't mark this item complete">
              {completeError}
            </Banner>
          )}
          <Button
            size="md"
            className="w-full"
            onClick={onComplete}
            loading={completing}
            disabled={completing || !allRequiredFilled}
            title={!allRequiredFilled ? 'Fill in all required rubric items' : undefined}
          >
            {completing ? (
              'Saving…'
            ) : (
              <span className="inline-flex items-center gap-space-2">
                {totalNeedsReview > 1 ? 'Next' : 'Done'}
                <KbdGroup>
                  <Kbd variant="inherit">⌘</Kbd>
                  <Kbd variant="inherit">↵</Kbd>
                </KbdGroup>
              </span>
            )}
          </Button>
        </div>
      )}
    </div>
  );
}
