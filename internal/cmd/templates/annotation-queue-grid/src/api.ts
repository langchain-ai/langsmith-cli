// LangSmith API access, entirely through window.langsmith.call — see
// global.d.ts for the bridge's type, and AGENTS.md for the operation format
// and available endpoints. No fetch()/XMLHttpRequest here: the sandbox this
// app runs in has no network access of its own.
//
// Queue membership uses GET .../items (metadata only). Hydrate RUN via
// GET /v2/runs/{id} and THREAD via POST /v1/trajectory (format: messages).
// Do not use GET /v2/threads/{id}/messages — it is SSE-only and the JSON
// bridge cannot stream it.
import type {
  AnnotationQueue,
  FeedbackConfigSchema,
  FeedbackItem,
  FeedbackSubmission,
  QueueItem,
  StandardMessage,
} from './types';

const TRAJECTORY_MAX_PAGES = 10;

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

/** UI section keys — "completed" maps to API status "archived". */
export type QueueItemSectionStatus =
  | 'needs_my_review'
  | 'needs_others_review'
  | 'completed';

export type QueueItemApiStatus =
  | 'needs_my_review'
  | 'needs_others_review'
  | 'archived';

export function toItemApiStatus(status: QueueItemSectionStatus): QueueItemApiStatus {
  return status === 'completed' ? 'archived' : status;
}

export interface ListQueueItemsResponse {
  items: QueueItem[];
  next_cursor: string | null;
  previous_cursor?: string | null;
}

export async function fetchQueueItems(
  queueId: string,
  status: QueueItemSectionStatus,
  pageSize = 50,
  cursor?: string | null
): Promise<ListQueueItemsResponse> {
  const params: Record<string, string> = {
    status: toItemApiStatus(status),
    page_size: String(pageSize),
  };
  if (cursor) params.cursor = cursor;
  return window.langsmith.call(
    `GET /api/v1/platform/annotation-queues/${queueId}/items`,
    {
      params,
    }
  ) as Promise<ListQueueItemsResponse>;
}

export async function fetchQueueItemsCount(
  queueId: string,
  status: QueueItemSectionStatus
): Promise<number> {
  const result = (await window.langsmith.call(
    `GET /api/v1/platform/annotation-queues/${queueId}/items/count`,
    { params: { status: toItemApiStatus(status) } }
  )) as { count: number };
  return result.count;
}

export async function fetchRun(
  runId: string,
  projectId: string,
  startTime?: string
): Promise<{
  id: string;
  name?: string | null;
  inputs?: Record<string, unknown> | null;
  outputs?: Record<string, unknown> | null;
  error?: string | null;
  trace_id?: string;
  project_id?: string;
  start_time?: string;
}> {
  const params: Record<string, string | string[]> = {
    project_id: projectId,
    // PROJECT_ID is the HTTP select token (wire key project_id); SESSION_ID is rejected.
    selects: ['ID', 'NAME', 'INPUTS', 'OUTPUTS', 'ERROR', 'TRACE_ID', 'PROJECT_ID', 'START_TIME'],
  };
  if (startTime) params.start_time = startTime;
  return window.langsmith.call(`GET /v2/runs/${runId}`, { params }) as Promise<{
    id: string;
    name?: string | null;
    inputs?: Record<string, unknown> | null;
    outputs?: Record<string, unknown> | null;
    error?: string | null;
    trace_id?: string;
    project_id?: string;
    start_time?: string;
  }>;
}

/** Chronological human/AI messages for a thread (JSON; not SSE /messages). */
export async function fetchThreadMessages(
  threadId: string,
  projectId: string
): Promise<StandardMessage[]> {
  const messages: StandardMessage[] = [];
  let cursor: string | undefined;
  for (let page = 0; page < TRAJECTORY_MAX_PAGES; page++) {
    const body: Record<string, string> = {
      project_id: projectId,
      thread_id: threadId,
      format: 'messages',
    };
    if (cursor) body.cursor = cursor;
    const resp = (await window.langsmith.call('POST /v1/trajectory', {
      body,
    })) as {
      messages?: StandardMessage[];
      next_cursor?: string | null;
    };
    messages.push(...(resp.messages ?? []));
    if (!resp.next_cursor) break;
    cursor = resp.next_cursor;
  }
  return messages;
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

export async function fetchFeedbacksForRun(runId: string): Promise<FeedbackItem[]> {
  return window.langsmith.call('GET /api/v1/feedback', {
    params: { run: runId },
  }) as Promise<FeedbackItem[]>;
}

export async function fetchFeedbacksForThread(
  feedbackThreadId: string,
  sessionId?: string
): Promise<FeedbackItem[]> {
  const params: Record<string, string> = { feedback_thread_id: feedbackThreadId };
  if (sessionId) params.session = sessionId;
  return window.langsmith.call('GET /api/v1/feedback', {
    params,
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

export async function markItemComplete(itemId: string): Promise<void> {
  await window.langsmith.call(
    `POST /api/v1/platform/annotation-queues/items/${itemId}/status`,
    {
      body: { status: 'completed' },
    }
  );
}
