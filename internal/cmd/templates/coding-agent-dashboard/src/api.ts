// All LangSmith access goes through window.langsmith.call — the sandbox has
// no network of its own. See AGENTS.md for the operation format and endpoints.
import type { Project, Run, RunsQueryResponse } from './types';

export async function fetchProjects(limit = 100): Promise<Project[]> {
  return window.langsmith.call('GET /api/v1/sessions', {
    params: { limit: String(limit) },
  }) as Promise<Project[]>;
}

// Metadata equality is two paired clauses in the filter DSL, not
// eq(metadata.key, ...).
const CODING_FILTER = 'and(eq(metadata_key, "ls_agent_purpose"), eq(metadata_value, "coding"))';

// Root coding-agent runs for one project over a recent window; session scope
// and is_root keep the query bounded.
export async function fetchCodingRuns(
  sessionId: string,
  windowDays = 7,
  limit = 100
): Promise<Run[]> {
  const startTime = new Date(Date.now() - windowDays * 86400000).toISOString();
  const resp = (await window.langsmith.call('POST /api/v1/runs/query', {
    body: {
      session: [sessionId],
      is_root: true,
      filter: CODING_FILTER,
      start_time: startTime,
      limit,
    },
  })) as RunsQueryResponse;
  return resp?.runs ?? [];
}
