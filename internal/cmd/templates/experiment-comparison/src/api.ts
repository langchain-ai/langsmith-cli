// All access goes through window.langsmith.call (the sandbox has no network).
// See AGENTS.md for the operations and shapes.
import type { Dataset, ExampleWithRuns, Experiment, ExperimentRun, FeedbackConfigSchema } from './types';

const COMPARISON_SELECTS = [
  'PROJECT_ID',
  'OUTPUTS',
  'ERROR',
  'START_TIME',
  'END_TIME',
  'TOTAL_TOKENS',
  'TOTAL_COST',
  'FEEDBACK_STATS',
];

interface V2RunResponse {
  project_id?: string;
  outputs?: Record<string, unknown> | null;
  error?: string | null;
  start_time?: string;
  end_time?: string | null;
  total_tokens?: number | null;
  total_cost?: number | null;
  feedback_stats?: Record<string, Record<string, unknown>> | null;
}

interface V2ExampleWithRuns {
  id: string;
  name?: string | null;
  inputs: Record<string, unknown> | null;
  outputs?: Record<string, unknown> | null;
  runs: V2RunResponse[];
}

interface V2ComparisonResponse {
  items: V2ExampleWithRuns[];
  next_cursor: string | null;
}

function toExperimentRun(run: V2RunResponse): ExperimentRun {
  return {
    session_id: run.project_id ?? '',
    outputs: run.outputs ?? null,
    outputs_preview: null,
    error: run.error ?? null,
    start_time: run.start_time ?? '',
    end_time: run.end_time ?? null,
    total_tokens: run.total_tokens ?? null,
    total_cost: run.total_cost ?? null,
    feedback_stats: run.feedback_stats ?? null,
  };
}

export async function fetchDatasets(
  search = '',
  offset = 0,
  limit = 100
): Promise<Dataset[]> {
  const params: Record<string, string> = { limit: String(limit), offset: String(offset) };
  if (search) params.name_contains = search;
  return window.langsmith.call('GET /api/v1/datasets', {
    params,
  }) as Promise<Dataset[]>;
}

// Experiments are the sessions whose reference_dataset is this dataset.
export async function fetchExperiments(
  datasetId: string,
  search = '',
  offset = 0,
  limit = 100
): Promise<Experiment[]> {
  const params: Record<string, string | string[]> = {
    reference_dataset: [datasetId],
    reference_free: 'false',
    limit: String(limit),
    offset: String(offset),
  };
  if (search) params.name_contains = search;
  return window.langsmith.call('GET /api/v1/sessions', {
    params,
  }) as Promise<Experiment[]>;
}

// Per-example rows joined server-side across the given experiments. Each
// example carries one run per session_id (see ExampleWithRuns).
export async function fetchComparison(
  datasetId: string,
  sessionIds: string[],
  limit = 25
): Promise<ExampleWithRuns[]> {
  const resp = (await window.langsmith.call(`POST /api/v2/datasets/${datasetId}/experiment-runs`, {
    body: { experiment_ids: sessionIds, page_size: limit, selects: COMPARISON_SELECTS },
  })) as V2ComparisonResponse;
  return (resp?.items ?? []).map((ex) => ({
    id: ex.id,
    name: ex.name ?? '',
    inputs: ex.inputs ?? null,
    outputs: ex.outputs ?? null,
    runs: (ex.runs ?? []).map(toExperimentRun),
  }));
}

// is_lower_score_better per feedback key, so score deltas honor direction.
export async function fetchFeedbackConfigs(keys: string[]): Promise<FeedbackConfigSchema[]> {
  if (keys.length === 0) return [];
  return window.langsmith.call('GET /api/v1/feedback-configs', {
    params: { key: keys },
  }) as Promise<FeedbackConfigSchema[]>;
}
