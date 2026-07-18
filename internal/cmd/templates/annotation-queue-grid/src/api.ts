// LangSmith API access, entirely through window.langsmith.call — see
// global.d.ts for the bridge's type, and AGENTS.md for the operation format
// and available endpoints. No fetch()/XMLHttpRequest here: the sandbox this
// app runs in has no network access of its own.
import type {
  AnnotationQueue,
  AnnotationQueueRun,
  FeedbackConfigSchema,
  FeedbackItem,
  FeedbackSubmission,
} from './types';

export async function fetchQueues(
  search = '',
  offset = 0,
  limit = 25
): Promise<AnnotationQueue[]> {
  const params: Record<string, string> = { limit: String(limit), offset: String(offset) };
  if (search) params.name_contains = search;
  return window.langsmith.call('GET /api/v1/annotation-queues', {
    params,
  }) as Promise<AnnotationQueue[]>;
}

export async function fetchQueue(queueId: string): Promise<AnnotationQueue> {
  return window.langsmith.call(
    `GET /api/v1/annotation-queues/${queueId}`
  ) as Promise<AnnotationQueue>;
}

export type AnnotationQueueRunSectionStatus =
  | 'needs_my_review'
  | 'needs_others_review'
  | 'completed';

export async function fetchQueueRuns(
  queueId: string,
  status: AnnotationQueueRunSectionStatus | null = null,
  limit = 50,
  offset = 0
): Promise<AnnotationQueueRun[]> {
  const params: Record<string, string> = { limit: String(limit), offset: String(offset) };
  if (status) params.status = status;
  return window.langsmith.call(`GET /api/v1/annotation-queues/${queueId}/runs`, {
    params,
  }) as Promise<AnnotationQueueRun[]>;
}

// The /runs endpoint's total-count header isn't reachable through the
// window.langsmith.call bridge (it only returns the parsed body), so we ask
// this endpoint separately to learn how many runs exist for a status.
export async function fetchQueueRunsSize(
  queueId: string,
  status: AnnotationQueueRunSectionStatus
): Promise<number> {
  const result = (await window.langsmith.call(
    `GET /api/v1/annotation-queues/${queueId}/size`,
    { params: { status } }
  )) as { size: number };
  return result.size;
}

export async function fetchFeedbackConfigs(keys: string[]): Promise<FeedbackConfigSchema[]> {
  if (keys.length === 0) return [];
  return window.langsmith.call('GET /api/v1/feedback-configs', {
    params: { key: keys },
  }) as Promise<FeedbackConfigSchema[]>;
}

export async function submitFeedback(feedback: FeedbackSubmission): Promise<FeedbackItem> {
  return window.langsmith.call('POST /api/v1/feedback', {
    body: {
      ...feedback,
      feedback_source: { type: 'app' },
      // The /feedback schema defaults this to true; explicitly opt out like
      // the real annotation queue UI does — a review action shouldn't have
      // the side effect of extending the underlying trace's retention.
      extend_trace_retention: false,
    },
  }) as Promise<FeedbackItem>;
}

export async function fetchFeedbacks(runId: string): Promise<FeedbackItem[]> {
  return window.langsmith.call('GET /api/v1/feedback', {
    params: { run: runId },
  }) as Promise<FeedbackItem[]>;
}

export async function patchFeedback(
  feedbackId: string,
  patch: Partial<Pick<FeedbackItem, 'score' | 'value' | 'comment'>>
): Promise<FeedbackItem> {
  return window.langsmith.call(`PATCH /api/v1/feedback/${feedbackId}`, {
    body: patch,
  }) as Promise<FeedbackItem>;
}

export async function deleteFeedback(feedbackId: string): Promise<void> {
  await window.langsmith.call(`DELETE /api/v1/feedback/${feedbackId}`);
}

export async function markRunComplete(queueRunId: string): Promise<void> {
  await window.langsmith.call(`POST /api/v1/annotation-queues/status/${queueRunId}`, {
    body: { status: 'completed' },
  });
}
