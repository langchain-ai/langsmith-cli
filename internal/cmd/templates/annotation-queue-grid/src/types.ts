export type IOMode = 'collapsed' | 'expanded' | 'raw';

export type QueueItemType = 'RUN' | 'THREAD';

/** Membership stub from GET .../items (metadata only — hydrate payloads separately). */
export interface QueueItem {
  id: string;
  item_type: QueueItemType;
  queue_id?: string;
  run_id?: string;
  thread_id?: string;
  project_id?: string;
  start_time?: string;
  added_at: string;
  effective_added_at?: string;
  last_reviewed_time: string | null;
  reserved_by: string[];
  completed_by: string[];
  // Hydrated (not on list):
  name?: string | null;
  inputs?: Record<string, unknown> | null;
  outputs?: Record<string, unknown> | null;
  error?: string | null;
  trace_id?: string;
  /** THREAD hydrate: chat turns from POST /v1/trajectory (format: messages) */
  messages?: StandardMessage[];
}

/** Normalized chat message from POST /v1/trajectory. */
export interface StandardMessage {
  role: string;
  content: string | Array<string | Record<string, unknown>>;
  id?: string;
  name?: string;
  tool_call_id?: string;
}

// Mirrors AnnotationQueueRubricItemSchema (smith-backend/app/schemas.py) — the
// rubric item itself carries no type/min/max/options; that comes from a
// separate per-key FeedbackConfig lookup (see fetchFeedbackConfigs in api.ts).
export interface RubricItem {
  feedback_key: string;
  description: string | null;
  value_descriptions: Record<string, string> | null;
  score_descriptions: Record<string, string> | null;
  is_required?: boolean | null;
  is_assertion?: boolean | null;
}

export interface AnnotationQueue {
  id: string;
  name: string;
  queue_type: 'single' | 'pairwise';
  rubric_items: RubricItem[];
  rubric_instructions: string | null;
  num_reviewers_per_item?: number | null;
}

export interface FeedbackCategory {
  value: number;
  label: string | null;
}

export type FeedbackConfigType = 'continuous' | 'categorical' | 'freeform';

// Mirrors FeedbackConfig (smith-backend/app/schemas.py) — determines how a
// feedback key should be rendered/entered (number input, category buttons, or
// free text), fetched separately per key from GET /feedback-configs.
export interface FeedbackConfig {
  type: FeedbackConfigType;
  min: number | null;
  max: number | null;
  categories: FeedbackCategory[] | null;
}

export interface FeedbackConfigSchema {
  feedback_key: string;
  feedback_config: FeedbackConfig;
  tenant_id: string;
  modified_at: string;
  is_lower_score_better: boolean | null;
}

export interface FeedbackSubmission {
  key: string;
  /** Required for RUN items; omit when using feedback_thread_id. */
  run_id?: string;
  /** Required for THREAD items; omit when using run_id. */
  feedback_thread_id?: string;
  score?: number | null;
  value?: string | null;
  comment?: string | null;
  // Only reviewer notes pass this today — creates the "note" key's config as
  // freeform the first time it's used, mirroring the real UI's RunNotesCrud.
  feedback_config?: { type: FeedbackConfigType };
  // Without these, the backend has to look the run up by run_id alone —
  // which can miss (falls into an async/best-effort path that may never
  // land) instead of writing the feedback inline. Always pass them when
  // known, same as the real annotation queue UI does.
  trace_id?: string;
  session_id?: string;
  start_time?: string;
}

export interface FeedbackItem {
  id: string;
  run_id?: string | null;
  feedback_thread_id?: string | null;
  key: string;
  score: number | null;
  value: string | null;
  comment: string | null;
  created_at: string;
}

/** Display label for a list/stub item before or after hydrate. */
export function itemLabel(item: QueueItem): string {
  if (item.name) return item.name;
  if (item.item_type === 'THREAD') {
    return item.thread_id ? item.thread_id.slice(0, 12) : item.id.slice(0, 8);
  }
  return item.run_id ? item.run_id.slice(0, 8) : item.id.slice(0, 8);
}

/** Key used to load/store feedback for this item (run id or thread id). */
export function feedbackSubjectKey(item: QueueItem): string | undefined {
  if (item.item_type === 'THREAD') return item.thread_id;
  return item.run_id;
}
