// All LangSmith access goes through window.langsmith.call — the sandbox has
// no network of its own. See AGENTS.md for the operation format and endpoints.
import type { Project, ProjectRuns, Run, RunsQueryResponse, ScopeKey } from './types';

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
// out of message text that varies between local dev and production. A
// workspace that's already saturated (busy shared project, several other
// dev sessions hitting the same key) can stay over its limit for seconds at
// a time, so this budget is generous: 5 attempts, backoff capped at 6s.
const RETRY_ATTEMPTS = 5;
const RETRY_BASE_DELAY_MS = 500;
const RETRY_MAX_DELAY_MS = 6000;

type CallArgs = Parameters<Window['langsmith']['call']>[1];

async function callWithRetry<T>(operation: string, args: CallArgs): Promise<T> {
  for (let attempt = 1; ; attempt++) {
    try {
      return (await window.langsmith.call(operation, args)) as T;
    } catch (e) {
      if (attempt >= RETRY_ATTEMPTS) throw e;
      const delay = Math.min(RETRY_BASE_DELAY_MS * 2 ** (attempt - 1), RETRY_MAX_DELAY_MS);
      await sleep(delay + Math.random() * 300);
    }
  }
}

export async function fetchProjects(
  search = '',
  offset = 0,
  limit = 25
): Promise<Project[]> {
  // reference_free=true keeps real tracing projects and drops eval experiments.
  const params: Record<string, string> = {
    limit: String(limit),
    offset: String(offset),
    reference_free: 'true',
  };
  if (search) params.name_contains = search;
  return callWithRetry<Project[]>('GET /api/v1/sessions', { params });
}

interface Scope {
  isRoot?: boolean;
  runType?: string;
}

const SCOPES: { key: ScopeKey; scope: Scope }[] = [
  { key: 'roots', scope: { isRoot: true } },
  { key: 'llm', scope: { runType: 'llm' } },
  { key: 'tool', scope: { runType: 'tool' } },
  { key: 'chain', scope: { runType: 'chain' } },
];

// Gap enforced between queries even when one resolves instantly (e.g. a
// cached response) — on top of whatever the retries above already spent
// waiting, this keeps the four queries from ever landing back-to-back.
const INTER_QUERY_GAP_MS = 250;

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
  const resp = await callWithRetry<RunsQueryResponse>('POST /api/v1/runs/query', { body });
  return resp?.runs ?? [];
}

// The four scoped queries (roots for economics, llm for models, tool for
// tool usage, chain for subagents) run one at a time, not in parallel — a
// single client firing several requests at once was enough to trip a
// workspace's rate limit on its own. Each query gets its own retry budget
// (callWithRetry), so a scope that's still failing after that budget is
// reported in failedScopes with an empty array instead of failing the
// other three scopes' data too.
export async function fetchProjectRuns(sessionId: string, windowDays = 7): Promise<ProjectRuns> {
  const startTime = windowStart(windowDays);
  const byKey: Record<ScopeKey, Run[]> = { roots: [], llm: [], tool: [], chain: [] };
  const failedScopes: ScopeKey[] = [];

  for (const { key, scope } of SCOPES) {
    try {
      byKey[key] = await queryRuns(sessionId, startTime, scope);
    } catch (e) {
      console.error(`Failed to load "${key}" runs after retries`, e);
      failedScopes.push(key);
    }
    await sleep(INTER_QUERY_GAP_MS);
  }

  const subagents = byKey.chain.filter((r) => r.extra?.metadata?.ls_agent_type === 'subagent');
  return { roots: byKey.roots, llm: byKey.llm, tool: byKey.tool, subagents, failedScopes };
}
