// All LangSmith access goes through window.langsmith.call — the sandbox has
// no network of its own. See AGENTS.md for the operation format and endpoints.
import type { Project, ProjectStats, RunGroupStats, RunsQueryResponse, RunStats, StatsScope } from './types';

// Metadata equality is two paired clauses in the filter DSL, not
// eq(metadata.key, ...).
const CODING_FILTER = 'and(eq(metadata_key, "ls_agent_purpose"), eq(metadata_value, "coding"))';

// The recent-runs table is intentionally bounded — it's a sample for
// browsing, not a data source for any headline number. Every stat above it
// comes from the stats endpoints instead, which aggregate server-side over
// the whole time window with no row limit.
const RECENT_RUNS_LIMIT = 100;

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

const STATS_SELECT = [
  'run_count',
  'error_rate',
  'latency_p50',
  'latency_p99',
  'latency_avg',
  'total_tokens',
  'prompt_tokens',
  'completion_tokens',
  'total_cost',
  'prompt_cost',
  'completion_cost',
];

// Gap enforced between queries even when one resolves instantly (e.g. a
// cached response) — on top of whatever the retries above already spent
// waiting, this keeps the two stats queries and the recent-runs query from
// ever landing back-to-back (see fetchProjectRuns's history in git blame for
// why: a single client firing several requests at once was enough to trip a
// workspace's rate limit on its own).
const INTER_QUERY_GAP_MS = 250;

// Exact, whole-window aggregates — no row limit, no sampling. Two
// independent calls (turns, threads) so one being rate-limited doesn't blank
// the other's numbers; see ProjectStats.failedScopes.
export async function fetchProjectStats(sessionId: string, windowDays: number): Promise<ProjectStats> {
  const startTime = windowStart(windowDays);
  const failedScopes: StatsScope[] = [];

  let turns: RunStats | null = null;
  try {
    turns = await callWithRetry<RunStats>('POST /api/v1/runs/stats', {
      body: { session: [sessionId], filter: CODING_FILTER, start_time: startTime, is_root: true, select: STATS_SELECT },
    });
  } catch (e) {
    console.error('Failed to load turn stats after retries', e);
    failedScopes.push('turns');
  }

  await sleep(INTER_QUERY_GAP_MS);

  let threads: RunGroupStats | null = null;
  try {
    threads = await callWithRetry<RunGroupStats>('POST /api/v1/runs/group/stats', {
      body: { session_id: sessionId, group_by: 'conversation', filter: CODING_FILTER, start_time: startTime },
    });
  } catch (e) {
    console.error('Failed to load thread stats after retries', e);
    failedScopes.push('threads');
  }

  return { turns, threads, failedScopes };
}

// A bounded sample for the recent-runs table only — never used to compute a
// headline stat, so its 100-row cap doesn't need to be (and isn't) accurate
// for anything but "what happened most recently."
export async function fetchRecentRuns(sessionId: string, windowDays: number) {
  await sleep(INTER_QUERY_GAP_MS);
  const resp = await callWithRetry<RunsQueryResponse>('POST /api/v1/runs/query', {
    body: {
      session: [sessionId],
      filter: CODING_FILTER,
      start_time: windowStart(windowDays),
      is_root: true,
      limit: RECENT_RUNS_LIMIT,
      select: ['id', 'name', 'error', 'start_time', 'end_time', 'total_tokens', 'total_cost', 'extra'],
    },
  });
  return resp?.runs ?? [];
}
