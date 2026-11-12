export type CallArgs = { params?: Record<string, string | string[]>; body?: unknown };

const RETRIES = 3;

export function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message || error.name;
  if (typeof error === 'string') return error;
  try {
    return JSON.stringify(error);
  } catch {
    return String(error);
  }
}

/** The host passes only the error's message, and LangSmith's 429 bodies say "rate limit exceeded". */
export function isRateLimited(error: unknown): boolean {
  const status = (error as { status?: unknown } | null)?.status;
  return status === 429 || /\b429\b|too many requests|rate limit/i.test(errorMessage(error));
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/** The bridge takes string query params: numbers and booleans are stringified, empty values dropped. */
export function toQueryParams(params: unknown): Record<string, string | string[]> | undefined {
  if (params == null || typeof params !== 'object') return undefined;
  const out: Record<string, string | string[]> = {};
  for (const [key, value] of Object.entries(params as Record<string, unknown>)) {
    if (value == null || value === '') continue;
    if (Array.isArray(value)) {
      const list = value
        .filter((v) => v != null && v !== '')
        .map((v) => (typeof v === 'object' ? JSON.stringify(v) : String(v)));
      if (list.length > 0) out[key] = list;
    } else {
      out[key] = typeof value === 'object' ? JSON.stringify(value) : String(value);
    }
  }
  return out;
}

export function buildArgs(params: unknown, body: unknown): CallArgs | undefined {
  const args: CallArgs = {};
  const query = toQueryParams(params);
  if (query && Object.keys(query).length > 0) args.params = query;
  if (body !== undefined && body !== null) args.body = body;
  return Object.keys(args).length > 0 ? args : undefined;
}

/** window.langsmith.call with exponential backoff and jitter on HTTP 429. */
export async function callApi<T = unknown>(operation: string, args?: CallArgs): Promise<T> {
  if (!window.langsmith?.call) throw new Error('window.langsmith is not available in this frame');
  for (let attempt = 0; ; attempt++) {
    try {
      return (await window.langsmith.call(operation, args)) as T;
    } catch (error) {
      if (attempt >= RETRIES || !isRateLimited(error)) throw error;
      await sleep(Math.min(500 * 2 ** attempt, 4000) + Math.random() * 400);
    }
  }
}
