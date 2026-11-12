import { useRef, useState, type KeyboardEvent } from 'react';
import { Badge } from '@langchain/macaw-components/Badge';
import { Banner } from '@langchain/macaw-components/Banner';
import { Button } from '@langchain/macaw-components/Button';
import { EmptyState } from '@langchain/macaw-components/EmptyState';
import { Input } from '@langchain/macaw-components/Input';
import { Kbd, KbdGroup } from '@langchain/macaw-components/Kbd';
import { RadioCard } from '@langchain/macaw-components/RadioCard';
import { RadioGroup } from '@langchain/macaw-components/RadioGroup';
import { Text } from '@langchain/macaw-components/Text';
import { Textarea } from '@langchain/macaw-components/Textarea';
import { cn } from '@langchain/macaw-components/utils/cn';
import { callApi, errorMessage } from './lib/api.ts';
import { toNumber } from './lib/format.ts';
import { isRecord } from './lib/path.ts';
import { MarkdownText } from './markdown.tsx';

type Option = { id: string; label: string; score: number; value?: string; description?: string };
type Item = {
  key: string;
  label: string;
  description?: string;
  required: boolean;
  /** `unavailable` when the item has no feedback config, so there is no scale to answer on. */
  type: 'categorical' | 'continuous' | 'freeform' | 'unavailable';
  /** The pinned feedback config's id, sent as `feedback_config_id`. */
  configId?: string;
  options: Option[];
  /** A continuous config's bounds. */
  min?: number;
  max?: number;
};
type Answer = { option?: string; text?: string };
type Submitted = Array<Record<string, unknown>>;

const humanize = (key: string) => {
  const words = key.replace(/[_-]+/g, ' ').trim();
  return words.charAt(0).toUpperCase() + words.slice(1);
};

/** A queue's `rubric_items`, typed by each item's pinned `feedback_config`; descriptions only label categories. */
function rubricItems(rubric: unknown): Item[] {
  const raw = isRecord(rubric) && Array.isArray(rubric.rubric_items) ? rubric.rubric_items : rubric;
  if (!Array.isArray(raw)) return [];
  return raw.filter(isRecord).flatMap((entry): Item[] => {
    if (typeof entry.feedback_key !== 'string' || entry.is_assertion === true) return [];
    const item: Item = {
      key: entry.feedback_key,
      label: humanize(entry.feedback_key),
      description:
        typeof entry.description === 'string' && entry.description ? entry.description : undefined,
      // `is_required` applies only while an item is shown, and `visible_if` isn't evaluated here.
      required: entry.is_required === true && entry.visible_if == null,
      type: 'unavailable',
      options: [],
    };
    const pinned = isRecord(entry.feedback_config) ? entry.feedback_config : null;
    const config = pinned && isRecord(pinned.feedback_config) ? pinned.feedback_config : null;
    if (!pinned || typeof pinned.id !== 'string' || !config) return [item];
    const configId = pinned.id;
    if (config.type === 'categorical') {
      const valueDescriptions = isRecord(entry.value_descriptions) ? entry.value_descriptions : {};
      const scoreDescriptions = isRecord(entry.score_descriptions) ? entry.score_descriptions : {};
      const categories = Array.isArray(config.categories) ? config.categories.filter(isRecord) : [];
      const options = categories.flatMap((category): Option[] => {
        const score = toNumber(category.value);
        if (score == null) return [];
        const label =
          typeof category.label === 'string' && category.label ? category.label : undefined;
        const description = label ? valueDescriptions[label] : scoreDescriptions[String(score)];
        return [
          {
            id: String(score),
            label: label ? humanize(label) : String(score),
            score,
            value: label,
            description: typeof description === 'string' && description ? description : undefined,
          },
        ];
      });
      return options.length > 0 ? [{ ...item, type: 'categorical', configId, options }] : [item];
    }
    if (config.type === 'continuous') {
      const min = toNumber(config.min) ?? undefined;
      const max = toNumber(config.max) ?? undefined;
      return [{ ...item, type: 'continuous', configId, min, max }];
    }
    if (config.type === 'freeform') return [{ ...item, type: 'freeform', configId }];
    return [item];
  });
}

function scoreHint(item: Item): string | undefined {
  if (item.min != null && item.max != null) return `A score from ${item.min} to ${item.max}`;
  if (item.min != null) return `A score of at least ${item.min}`;
  if (item.max != null) return `A score of at most ${item.max}`;
  return undefined;
}

/** Why a score breaks its config's range, or null when it fits; the server checks only enforced configs. */
function scoreError(item: Item, score: number): string | null {
  const { min, max } = item;
  if (min != null && max != null && (score < min || score > max))
    return `Score must be between ${min} and ${max}`;
  if (min != null && score < min) return `Score must be at least ${min}`;
  if (max != null && score > max) return `Score must be at most ${max}`;
  return null;
}

function answered(item: Item, answer: Answer | undefined): boolean {
  if (item.type === 'categorical') return answer?.option != null;
  if (item.type === 'continuous') return toNumber(answer?.text) != null;
  if (item.type === 'freeform') return Boolean(answer?.text?.trim());
  return false;
}

/** The answer's problem when it can't be saved as given. */
function answerError(item: Item, answer: Answer | undefined): string | null {
  if (item.type !== 'continuous') return null;
  const score = toNumber(answer?.text);
  return score == null ? null : scoreError(item, score);
}

function feedbackBody(item: Item, answer: Answer): Record<string, unknown> {
  const target = { key: item.key, feedback_config_id: item.configId };
  if (item.type === 'categorical') {
    const option = item.options.find((o) => o.id === answer.option);
    return { ...target, score: option?.score, ...(option?.value ? { value: option.value } : {}) };
  }
  if (item.type === 'continuous') return { ...target, score: toNumber(answer.text) };
  return { ...target, comment: answer.text?.trim() };
}

type FormProps = {
  rubric: unknown;
  title?: string;
  instructions?: string;
  submitLabel?: string;
};

function Form({
  props,
  subject,
  onSubmitted,
}: {
  props: FormProps;
  subject: Record<string, string> | null;
  onSubmitted?: (feedback: Submitted) => void | Promise<void>;
}) {
  const items = rubricItems(props.rubric);
  const [answers, setAnswers] = useState<Record<string, Answer>>({});
  // Keys already saved for this subject: a retry skips them, so feedback is never written twice.
  const [saved, setSaved] = useState<Record<string, true>>({});
  // One feedback id per key, so a retry after a lost response updates the same row.
  const feedbackIds = useRef<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [finished, setFinished] = useState(false);
  const [error, setError] = useState<{ title: string; message: string } | null>(null);
  const [showMissing, setShowMissing] = useState(false);
  const open = items.filter((item) => item.type !== 'unavailable' && !saved[item.key]);
  const missing = open.filter((item) => item.required && !answered(item, answers[item.key]));
  const outOfRange = open.filter((item) => answerError(item, answers[item.key]));

  const update = (key: string, patch: Answer) => {
    setAnswers((prev) => ({ ...prev, [key]: { ...prev[key], ...patch } }));
    setFinished(false);
  };

  const submit = async () => {
    if (saving || finished) return;
    setError(null);
    const fail = (message: string) => setError({ title: 'Feedback not saved', message });
    if (!subject) return fail('Select a run or thread before submitting feedback.');
    if (missing.length > 0) {
      setShowMissing(true);
      return fail(
        `Answer the required ${missing.length === 1 ? 'item' : 'items'}: ${missing.map((m) => m.label).join(', ')}.`
      );
    }
    if (outOfRange.length > 0) {
      return fail(
        outOfRange
          .map((item) => `${item.label}: ${answerError(item, answers[item.key])}.`)
          .join(' ')
      );
    }
    const payloads: Submitted = items
      .filter((item) => answered(item, answers[item.key]))
      .map((item) => ({
        id: (feedbackIds.current[item.key] ??= crypto.randomUUID()),
        ...feedbackBody(item, answers[item.key]),
      }));
    if (payloads.length === 0) return fail('Answer at least one item.');
    setSaving(true);
    const done = { ...saved };
    try {
      for (const payload of payloads) {
        const key = String(payload.key);
        if (done[key]) continue;
        await callApi('POST /api/v1/feedback', {
          body: {
            ...payload,
            ...subject,
            feedback_source: { type: 'app' },
            extend_trace_retention: false,
          },
        });
        done[key] = true;
        setSaved({ ...done });
      }
    } catch (e) {
      setSaving(false);
      fail(errorMessage(e));
      console.error('[blocks] FeedbackForm submit failed:', errorMessage(e));
      return;
    }
    try {
      await onSubmitted?.(payloads);
      setFinished(true);
    } catch (e) {
      setError({ title: 'Feedback saved, but the next step failed', message: errorMessage(e) });
      console.error('[blocks] FeedbackForm onSubmitted failed:', errorMessage(e));
    } finally {
      setSaving(false);
    }
  };

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      void submit();
    }
  };

  return (
    <div className="flex min-w-0 flex-col gap-space-5" onKeyDown={onKeyDown}>
      {(props.title || props.instructions) && (
        <div className="flex flex-col gap-space-2">
          {props.title && (
            <Text as="h3" variant="h3">
              {props.title}
            </Text>
          )}
          {props.instructions && (
            <MarkdownText content={props.instructions} className="text-secondary" />
          )}
        </div>
      )}
      {items.length === 0 && (
        <EmptyState
          size="sm"
          title="No rubric items"
          description="This queue has no feedback rubric yet."
        />
      )}
      {items.map((item) => {
        const answer = answers[item.key];
        const locked = Boolean(saved[item.key]) || saving;
        const rangeError = saved[item.key] ? null : answerError(item, answer);
        const invalid =
          Boolean(rangeError) ||
          (showMissing && item.required && !saved[item.key] && !answered(item, answer));
        return (
          <fieldset key={item.key} className="m-0 flex min-w-0 flex-col gap-space-2 border-0 p-0">
            <div className="flex flex-col gap-space-1">
              <legend className="flex items-center gap-space-2 p-0">
                <Text as="span" variant="md" weight="medium">
                  {item.label}
                </Text>
                {saved[item.key] ? (
                  <Badge color="success" size="xs">
                    Saved
                  </Badge>
                ) : item.type === 'unavailable' ? (
                  <Badge color="warning" size="xs">
                    Unavailable
                  </Badge>
                ) : (
                  item.required && (
                    <Badge color={invalid ? 'error' : 'secondary'} size="xs">
                      Required
                    </Badge>
                  )
                )}
              </legend>
              {item.description && (
                <Text variant="sm" color="tertiary">
                  {item.description}
                </Text>
              )}
            </div>
            {item.type === 'unavailable' ? (
              <Text variant="sm" color="tertiary">
                This item has no feedback config, so it can’t be answered here.
              </Text>
            ) : item.type === 'categorical' ? (
              <RadioGroup
                value={answer?.option ?? ''}
                onValueChange={(option) => update(item.key, { option })}
                disabled={locked}
                className="flex flex-col gap-space-2"
                aria-label={item.label}
              >
                {item.options.map((option) => (
                  <RadioCard
                    key={option.id}
                    value={option.id}
                    disabled={locked}
                    className={cn('items-start p-space-3', invalid && 'border-error')}
                  >
                    <div className="flex min-w-0 flex-col gap-0.5">
                      <Text as="span" variant="md" weight="medium">
                        {option.label}
                      </Text>
                      {option.description && (
                        <Text as="span" variant="sm" color="tertiary" className="whitespace-normal">
                          {option.description}
                        </Text>
                      )}
                    </div>
                  </RadioCard>
                ))}
              </RadioGroup>
            ) : item.type === 'continuous' ? (
              <Input
                size="md"
                type="number"
                debounceMs={0}
                value={answer?.text ?? ''}
                isError={invalid}
                disabled={locked}
                hintText={rangeError ?? scoreHint(item)}
                placeholder="Score"
                aria-label={item.label}
                onChange={(text) => update(item.key, { text })}
              />
            ) : (
              <Textarea
                size="md"
                rows={3}
                value={answer?.text ?? ''}
                isError={invalid}
                disabled={locked}
                placeholder="Write feedback"
                aria-label={item.label}
                onChange={(text) => update(item.key, { text })}
              />
            )}
          </fieldset>
        );
      })}
      {error && (
        <Banner intent="error" title={error.title}>
          {error.message}
        </Banner>
      )}
      {finished && <Banner intent="success" title="Feedback saved" />}
      <div className="flex items-center justify-between gap-space-3">
        <KbdGroup className="text-tertiary">
          <Kbd>⌘</Kbd>
          <Kbd>Enter</Kbd>
        </KbdGroup>
        <Button
          type="button"
          color="primary"
          variant="normal"
          size="md"
          loading={saving}
          disabled={!subject || finished}
          onClick={() => void submit()}
        >
          {props.submitLabel || 'Submit'}
        </Button>
      </div>
    </div>
  );
}

export type FeedbackFormProps = {
  /** The reviewed run (RUN items) or thread (THREAD items). */
  runId?: string | null;
  threadId?: string | null;
  /** The item's project_id. Required with threadId: thread feedback is rejected without its project. */
  projectId?: string | null;
  /**
   * A RUN item's `trace_id` and `start_time` (from `useRun`). LangSmith saves run feedback against
   * its trace, and without them feedback on a run it no longer has in its ingest cache is never saved.
   */
  traceId?: string | null;
  startTime?: string | null;
  /**
   * The `rubric_items` of a queue read by id (or that queue object). Each item's pinned
   * `feedback_config` sets its type: categories, a continuous score checked against the config's
   * bounds, or free text. An item without one shows as unavailable. Multi-select categories,
   * `visible_if`/`required_if` rules and `regex_validator` are not supported: an item with
   * `visible_if` always shows and is optional.
   */
  rubric: unknown;
  title?: string;
  instructions?: string;
  submitLabel?: string;
  /**
   * Called after every answer saved, with the payloads sent. If it throws, the form reports that apart
   * from save errors, and Submit retries it without saving anything twice.
   */
  onSubmitted?: (feedback: Submitted) => void | Promise<void>;
};

/** A rubric form that posts one POST /api/v1/feedback per answered item, then calls onSubmitted. */
export function FeedbackForm({
  runId,
  threadId,
  projectId,
  traceId,
  startTime,
  rubric,
  title,
  instructions,
  submitLabel,
  onSubmitted,
}: FeedbackFormProps) {
  const session: Record<string, string> = projectId ? { session_id: projectId } : {};
  const trace: Record<string, string> = {
    ...(traceId ? { trace_id: traceId } : {}),
    ...(startTime ? { start_time: startTime } : {}),
  };
  const subject: Record<string, string> | null = runId
    ? { run_id: runId, ...session, ...trace }
    : threadId && projectId
      ? { feedback_thread_id: threadId, ...session }
      : null;
  return (
    <Form
      key={`${projectId ?? ''}:${runId ?? threadId ?? ''}`}
      props={{ rubric, title, instructions, submitLabel }}
      subject={subject}
      onSubmitted={onSubmitted}
    />
  );
}

/**
 * Records this reviewer's completion of a queue item. The item moves to Completed (API status
 * `archived`) once the queue's review requirements are met, such as its reviewers per item.
 */
export function completeQueueItem(itemId: string) {
  return callApi(`POST /api/v1/platform/annotation-queues/items/${itemId}/status`, {
    body: { status: 'completed' },
  });
}
