// All LangSmith access goes through window.langsmith.call — the sandbox has
// no network of its own. See AGENTS.md for the operation format and endpoints.
import type { Project, ProjectRuns, Run, RunsQueryResponse } from './types';

// Metadata equality is two paired clauses in the filter DSL, not
// eq(metadata.key, ...).
const CODING_FILTER = 'and(eq(metadata_key, "ls_agent_purpose"), eq(metadata_value, "coding"))';

// Each query returns at most the 100 most-recent matches; cursors are ignored.
const LIMIT = 100;

// extra carries the metadata every panel reads; the rest roll up on roots.
const SELECT = [
  'id', 'name', 'run_type', 'parent_run_id', 'trace_id', 'error',
  'start_time', 'end_time', 'total_tokens', 'prompt_tokens',
  'completion_tokens', 'total_cost', 'prompt_token_details', 'extra',
];

function windowStart(days: number): string {
  return new Date(Date.now() - days * 86400000).toISOString();
}

export async function fetchProjects(
  search = '',
  offset = 0,
  limit = 25
): Promise<Project[]> {
  const params: Record<string, string> = { limit: String(limit), offset: String(offset) };
  if (search) params.name_contains = search;
  return window.langsmith.call('GET /api/v1/sessions', {
    params,
  }) as Promise<Project[]>;
}

interface Scope {
  isRoot?: boolean;
  runType?: string;
}

async function queryRuns(sessionId: string, startTime: string, scope: Scope): Promise<Run[]> {
  const body: Record<string, unknown> = {
    session: [sessionId],
    filter: CODING_FILTER,
    start_time: startTime,
    limit: LIMIT,
    select: SELECT,
  };
  if (scope.isRoot) body.is_root = true;
  if (scope.runType) body.run_type = scope.runType;
  const resp = (await window.langsmith.call('POST /api/v1/runs/query', { body })) as RunsQueryResponse;
  return resp?.runs ?? [];
}

// Four scoped queries: roots (economics), llm (models), tool (tool usage),
// chain (subagents). run_type is honored server-side, so no per-trace fan-out.
export async function fetchProjectRuns(sessionId: string, windowDays = 7): Promise<ProjectRuns> {
  const startTime = windowStart(windowDays);
  const [roots, llm, tool, chains] = await Promise.all([
    queryRuns(sessionId, startTime, { isRoot: true }),
    queryRuns(sessionId, startTime, { runType: 'llm' }),
    queryRuns(sessionId, startTime, { runType: 'tool' }),
    queryRuns(sessionId, startTime, { runType: 'chain' }),
  ]);
  const subagents = chains.filter((r) => r.extra?.metadata?.ls_agent_type === 'subagent');
  return { roots, llm, tool, subagents };
}
