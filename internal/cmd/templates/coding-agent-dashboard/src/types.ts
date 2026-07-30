export interface Project {
  id: string;
  name: string;
}

// Fields selected from POST /v2/runs/query for the recent-runs table.
// Custom metadata lives at extra.metadata; token/cost totals roll up onto
// root runs.
export interface Run {
  id: string;
  name: string | null;
  error: string | null;
  start_time: string | null;
  end_time: string | null;
  total_tokens: number | null;
  total_cost: number | null;
  extra: { metadata?: Record<string, unknown> } | null;
}

export interface RunsQueryResponse {
  items: Run[];
  next_cursor?: string | null;
}

// Field names mirror `schemas.RunStats` in smith-backend/app/schemas.py.
// Every field is optional/nullable there too, so a stat this app didn't ask
// for (or one the backend didn't compute) is simply absent, not zero.
export interface RunStats {
  run_count?: number | null;
  error_rate?: number | null;
  latency_p50?: number | null;
  latency_p99?: number | null;
  total_tokens?: number | null;
  prompt_tokens?: number | null;
  completion_tokens?: number | null;
  total_cost?: number | null;
  prompt_cost?: number | null;
  completion_cost?: number | null;
}

// `schemas.RunGroupStats` = RunStats + group_count (distinct thread count),
// returned by POST /api/v1/runs/group/stats with group_by="conversation".
export interface RunGroupStats extends RunStats {
  group_count?: number | null;
}

// Share of the most recent root runs (up to 100) that are coding-agent
// traces — metadata ls_agent_purpose="coding". Computed client-side over an
// unfiltered recent-runs sample, so it's "share of the last N", not a
// whole-window rate. See fetchCodingShare.
export interface CodingShare {
  coding: number;
  total: number;
}

export type StatsScope = 'turns' | 'threads';

// The two stats calls this dashboard makes, each independently retryable —
// one scope failing (e.g. still rate-limited) shouldn't blank the other's
// numbers. See fetchProjectStats.
export interface ProjectStats {
  turns: RunStats | null;
  threads: RunGroupStats | null;
  failedScopes: StatsScope[];
}
