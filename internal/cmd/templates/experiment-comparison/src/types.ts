export interface Dataset {
  id: string;
  name: string;
}

export interface Experiment {
  id: string;
  name: string;
}

// Per-run feedback_stats is an untyped nested map from the API
// ({ [key]: { avg, n, ... } }); read it through scoreFor in lib/delta.
export interface ExperimentRun {
  session_id: string;
  outputs: Record<string, unknown> | null;
  outputs_preview: string | null;
  error: string | null;
  start_time: string;
  end_time: string | null;
  total_tokens: number | null;
  total_cost: number | null;
  feedback_stats: Record<string, Record<string, unknown>> | null;
}

export interface ExampleWithRuns {
  id: string;
  name: string;
  inputs: Record<string, unknown> | null;
  outputs: Record<string, unknown> | null;
  runs: ExperimentRun[];
}

export interface FeedbackConfigSchema {
  feedback_key: string;
  is_lower_score_better: boolean | null;
}

// An ordered experiment with its display letter and series color (baseline first).
export interface ExperimentView {
  id: string;
  name: string;
  letter: string;
  color: string;
  isBaseline: boolean;
}

// Per-experiment aggregates derived client-side over the fetched examples.
export interface Aggregate {
  experimentId: string;
  runCount: number;
  avgLatencyMs: number | null;
  totalCost: number | null;
  avgTokens: number | null;
  avgScores: Record<string, number>;
}
