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

export async function fetchQueue(queueId: string): Promise<AnnotationQueue> {
  return window.langsmith.call(
    `GET /api/v1/annotation-queues/${queueId}`
  ) as Promise<AnnotationQueue>;
}

export async function fetchQueueRuns(
  queueId: string,
  status: 'needs_my_review' | 'needs_others_review' | 'completed' | null = null,
  limit = 50
): Promise<AnnotationQueueRun[]> {
  const params: Record<string, string> = { limit: String(limit), offset: '0' };
  if (status) params.status = status;
  return window.langsmith.call(`GET /api/v1/annotation-queues/${queueId}/runs`, {
    params,
  }) as Promise<AnnotationQueueRun[]>;
}

export async function fetchFeedbackConfigs(keys: string[]): Promise<FeedbackConfigSchema[]> {
  if (keys.length === 0) return [];
  return window.langsmith.call('GET /api/v1/feedback-configs', {
    params: { key: keys },
  }) as Promise<FeedbackConfigSchema[]>;
}

export async function submitFeedback(feedback: FeedbackSubmission): Promise<FeedbackItem> {
  return window.langsmith.call('POST /api/v1/feedback', {
    body: feedback,
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
