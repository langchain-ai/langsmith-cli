export interface Project {
  id: string;
  name: string;
}

// Fields selected from POST /api/v1/runs/query. Custom metadata lives at
// extra.metadata; token/cost totals roll up onto root runs.
export interface Run {
  id: string;
  name: string | null;
  run_type: string | null;
  parent_run_id: string | null;
  trace_id: string | null;
  error: string | null;
  start_time: string | null;
  end_time: string | null;
  total_tokens: number | null;
  prompt_tokens: number | null;
  completion_tokens: number | null;
  total_cost: number | null;
  prompt_token_details: { cache_read?: number; cache_creation?: number } | null;
  extra: { metadata?: Record<string, unknown>; runtime?: Record<string, unknown> } | null;
}

export interface RunsQueryResponse {
  runs: Run[];
  cursor?: string | null;
}

// The four scoped result sets one project view is built from.
export interface ProjectRuns {
  roots: Run[];
  llm: Run[];
  tool: Run[];
  subagents: Run[];
}

// Per-integration model counts for the model×integration breakdown.
export interface IntegrationStat {
  integration: string;
  count: number;
  errors: number;
  models: { model: string; count: number }[];
}
