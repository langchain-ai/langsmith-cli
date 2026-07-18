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

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// window.langsmith.call collapses every failure (including a transient 429
// from firing several queries at once) into a plain Error with no status
// code attached — the sandbox bridge drops it before the app ever sees it.
// So retry blindly on any failure rather than trying to sniff "rate limit"
// out of message text that varies between local dev and production.
const RETRY_ATTEMPTS = 3;
const RETRY_BASE_DELAY_MS = 400;

async function callWithRetry<T>(operation: string, args: unknown): Promise<T> {
  for (let attempt = 1; ; attempt++) {
    try {
      return (await window.langsmith.call(operation, args)) as T;
    } catch (e) {
      if (attempt >= RETRY_ATTEMPTS) throw e;
      await sleep(RETRY_BASE_DELAY_MS * 2 ** (attempt - 1) + Math.random() * 200);
    }
  }
}

export async function fetchProjects(
  search = '',
  offset = 0,
  limit = 25
): Promise<Project[]> {
  const params: Record<string, string> = { limit: String(limit), offset: String(offset) };
  if (search) params.name_contains = search;
  return callWithRetry<Project[]>('GET /api/v1/sessions', { params });
}

interface Scope {
  isRoot?: boolean;
  runType?: string;
}

// staggerMs spaces out the four fetchProjectRuns queries below so they don't
// all land on the API in the same instant — reduces the odds of a burst rate
// limit tripping on any one of them.
async function queryRuns(sessionId: string, startTime: string, scope: Scope, staggerMs = 0): Promise<Run[]> {
  if (staggerMs) await sleep(staggerMs);
  const body: Record<string, unknown> = {
    session: [sessionId],
    filter: CODING_FILTER,
    start_time: startTime,
    limit: LIMIT,
    select: SELECT,
  };
  if (scope.isRoot) body.is_root = true;
  if (scope.runType) body.run_type = scope.runType;
  const resp = await callWithRetry<RunsQueryResponse>('POST /api/v1/runs/query', { body });
  return resp?.runs ?? [];
}

// Four scoped queries: roots (economics), llm (models), tool (tool usage),
// chain (subagents). run_type is honored server-side, so no per-trace fan-out.
export async function fetchProjectRuns(sessionId: string, windowDays = 7): Promise<ProjectRuns> {
  const startTime = windowStart(windowDays);
  const [roots, llm, tool, chains] = await Promise.all([
    queryRuns(sessionId, startTime, { isRoot: true }, 0),
    queryRuns(sessionId, startTime, { runType: 'llm' }, 150),
    queryRuns(sessionId, startTime, { runType: 'tool' }, 300),
    queryRuns(sessionId, startTime, { runType: 'chain' }, 450),
  ]);
  const subagents = chains.filter((r) => r.extra?.metadata?.ls_agent_type === 'subagent');
  return { roots, llm, tool, subagents };
}
