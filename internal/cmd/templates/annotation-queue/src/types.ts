export type IOMode = 'collapsed' | 'expanded' | 'raw';

export interface AnnotationQueueRun {
  id: string;
  queue_run_id: string;
  session_id: string;
  name: string | null;
  inputs: Record<string, unknown> | null;
  outputs: Record<string, unknown> | null;
  error: string | null;
  last_reviewed_time: string | null;
  added_at: string | null;
  completed_by: string[];
  reserved_by: string[];
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
  run_id: string;
  score?: number | null;
  value?: string | null;
  comment?: string | null;
}

export interface FeedbackItem {
  id: string;
  run_id: string;
  key: string;
  score: number | null;
  value: string | null;
  comment: string | null;
  created_at: string;
}
