// All access goes through window.langsmith.call (the sandbox has no network).
// See AGENTS.md for the operations and shapes.
import type { Dataset, ExampleWithRuns, Experiment, FeedbackConfigSchema } from './types';

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
  return window.langsmith.call(`POST /api/v1/datasets/${datasetId}/runs`, {
    body: { session_ids: sessionIds, limit, offset: 0 },
  }) as Promise<ExampleWithRuns[]>;
}

// is_lower_score_better per feedback key, so score deltas honor direction.
export async function fetchFeedbackConfigs(keys: string[]): Promise<FeedbackConfigSchema[]> {
  if (keys.length === 0) return [];
  return window.langsmith.call('GET /api/v1/feedback-configs', {
    params: { key: keys },
  }) as Promise<FeedbackConfigSchema[]>;
}
