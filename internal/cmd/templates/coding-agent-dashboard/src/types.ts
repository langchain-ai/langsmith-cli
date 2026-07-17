export interface Project {
  id: string;
  name: string;
}

// A root run from POST /api/v1/runs/query. Custom metadata lives under
// extra.metadata (see LangSmith run schema).
export interface Run {
  id: string;
  name: string | null;
  error: string | null;
  extra: { metadata?: Record<string, unknown> } | null;
}

export interface RunsQueryResponse {
  runs: Run[];
}

// Aggregated counts for one ls_integration value.
export interface IntegrationStat {
  integration: string;
  count: number;
  errors: number;
  models: { model: string; count: number }[];
}
