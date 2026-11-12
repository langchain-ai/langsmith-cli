import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { Banner } from '@langchain/macaw-components/Banner';
import { Button } from '@langchain/macaw-components/Button';
import { ChartCardSkeleton } from '@langchain/macaw-components/ChartCard';
import { EmptyState } from '@langchain/macaw-components/EmptyState';
import { LinearProgress } from '@langchain/macaw-components/LinearProgress';
import { Skeleton } from '@langchain/macaw-components/Skeleton';
import { cn } from '@langchain/macaw-components/utils/cn';
import { buildArgs, callApi, errorMessage, type CallArgs } from './lib/api.ts';
import { getPath, isRecord, stableKey } from './lib/path.ts';

/** window.langsmith.call with up to three retries on HTTP 429 (exponential backoff with jitter). Use for one-off reads and writes. */
export function callWithRetry<T = unknown>(
  operation: string,
  args?: { params?: Record<string, unknown>; body?: unknown }
): Promise<T> {
  return callApi<T>(operation, buildArgs(args?.params, args?.body) as CallArgs | undefined);
}

export type QueryOptions = {
  params?: Record<string, unknown>;
  body?: unknown;
  /** False (or a falsy dependency such as an unselected id) holds the request. */
  enabled?: boolean;
  /** Dot path to the list in the response; default `items`, or the response itself when it is an array. */
  itemsPath?: string;
  /** `cursor` passes `next_cursor` back as `cursor` (query param for GET, body field otherwise); `offset` pages by `offset`/`limit`. Default `cursor`. */
  paging?: 'cursor' | 'offset' | false;
  /** Page size for offset paging (sent as `limit`). Default 50. */
  pageSize?: number;
};

export type QueryResult<T = unknown, I = Record<string, unknown>> = {
  data: T | null;
  items: I[] | null;
  loading: boolean;
  loadingMore: boolean;
  error: string | null;
  hasMore: boolean;
  loadMore: () => void;
  refresh: () => void;
};

type State = {
  /** The request this state belongs to. */
  key: string | null;
  data: unknown;
  items: unknown[] | null;
  next: string | number | null;
  loading: boolean;
  loadingMore: boolean;
  error: string | null;
};
const IDLE: State = {
  key: null,
  data: null,
  items: null,
  next: null,
  loading: false,
  loadingMore: false,
  error: null,
};

function itemsOf(response: unknown, itemsPath?: string): unknown[] | null {
  if (itemsPath) {
    const value = getPath(response, itemsPath);
    return Array.isArray(value) ? value : null;
  }
  if (isRecord(response) && Array.isArray(response.items)) return response.items;
  return Array.isArray(response) ? response : null;
}

const isGet = (operation: string) => /^GET\s/i.test(operation);

/**
 * Loads `operation` through the bridge and re-fetches whenever the operation, params or body
 * change. Retries 429s, ignores stale responses, and pages one request at a time.
 */
export function useQuery<T = unknown, I = Record<string, unknown>>(
  operation: string | null | undefined,
  options: QueryOptions = {}
): QueryResult<T, I> {
  const { params, body, enabled = true, itemsPath, paging = 'cursor', pageSize = 50 } = options;
  const active = Boolean(operation) && enabled;
  const key = stableKey([operation ?? null, params ?? null, body ?? null, paging, pageSize]);
  const [state, setState] = useState<State>({ ...IDLE, loading: active });
  const latest = useRef({ key, operation, params, body, itemsPath, paging, pageSize });
  latest.current = { key, operation, params, body, itemsPath, paging, pageSize };
  const sequence = useRef(0);
  // The load whose next page is in flight, so an older request's page never blocks a newer one's.
  const pendingMore = useRef<number | null>(null);
  const stateRef = useRef(state);
  stateRef.current = state;

  const run = useCallback(async (mode: 'load' | 'more') => {
    const o = latest.current;
    if (!o.operation) return;
    const previous = stateRef.current;
    if (
      mode === 'more' &&
      (previous.key !== o.key ||
        previous.next == null ||
        previous.loadingMore ||
        previous.loading ||
        pendingMore.current === sequence.current)
    )
      return;
    // Two loadMore calls before a re-render would otherwise fetch the same page twice.
    if (mode === 'more') pendingMore.current = sequence.current;
    const id = mode === 'more' ? sequence.current : ++sequence.current;
    // A refresh keeps its data while it loads; a new request starts empty.
    setState((s) =>
      mode === 'more'
        ? { ...s, loadingMore: true }
        : s.key === o.key
          ? { ...s, loading: true, error: null }
          : { ...IDLE, key: o.key, loading: true }
    );
    let p: Record<string, unknown> | undefined = o.params;
    let b: unknown = o.body;
    if (o.paging === 'offset') {
      const offset = mode === 'more' ? Number(previous.next) : 0;
      const page = { limit: o.pageSize, offset };
      if (isGet(o.operation)) p = { ...(p ?? {}), ...page };
      else b = { ...(isRecord(b) ? b : {}), ...page };
    } else if (o.paging === 'cursor' && mode === 'more') {
      if (isGet(o.operation)) p = { ...(p ?? {}), cursor: previous.next };
      else b = { ...(isRecord(b) ? b : {}), cursor: previous.next };
    }
    try {
      const response = await callApi(o.operation, buildArgs(p, b));
      if (id !== sequence.current) return;
      const items = itemsOf(response, o.itemsPath);
      let next: string | number | null = null;
      if (o.paging === 'cursor') {
        const cursor = getPath(response, 'next_cursor');
        next = typeof cursor === 'string' && cursor ? cursor : null;
      } else if (o.paging === 'offset' && items) {
        const offset = mode === 'more' ? Number(previous.next) : 0;
        next = items.length >= o.pageSize ? offset + items.length : null;
      }
      setState((s) => ({
        key: o.key,
        data: response,
        items: mode === 'more' ? [...(s.items ?? []), ...(items ?? [])] : items,
        next,
        loading: false,
        loadingMore: false,
        error: null,
      }));
    } catch (error) {
      if (id !== sequence.current) return;
      console.error(`[blocks] ${o.operation} failed:`, errorMessage(error));
      setState((s) => ({ ...s, loading: false, loadingMore: false, error: errorMessage(error) }));
    } finally {
      if (mode === 'more' && pendingMore.current === id) pendingMore.current = null;
    }
  }, []);

  useEffect(() => {
    if (!active) {
      sequence.current++;
      setState(IDLE);
      return;
    }
    void run('load');
  }, [active, key, run]);

  const loadMore = useCallback(() => void run('more'), [run]);
  const refresh = useCallback(() => void run('load'), [run]);

  // Until the effect above starts this request, state may still hold the previous one's response.
  const view = state.key === key ? state : { ...IDLE, loading: active };
  return {
    data: view.data as T | null,
    items: view.items as I[] | null,
    loading: view.loading,
    loadingMore: view.loadingMore,
    error: view.error,
    hasMore: view.next != null,
    loadMore,
    refresh,
  };
}

type SkeletonKind = 'list' | 'table' | 'chart' | 'metrics' | 'none';

function LoadingSkeleton({ kind = 'list' }: { kind?: SkeletonKind }) {
  if (kind === 'none') return null;
  if (kind === 'chart') return <ChartCardSkeleton variant="bar" className="h-[240px]" />;
  if (kind === 'metrics') {
    return (
      <div
        className="grid grid-cols-2 gap-space-4 md:grid-cols-4"
        aria-busy="true"
        aria-label="Loading"
      >
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-[88px] rounded-lg" />
        ))}
      </div>
    );
  }
  const rows = kind === 'table' ? 6 : 3;
  return (
    <div className="flex flex-col gap-space-3" aria-busy="true" aria-label="Loading">
      {kind === 'table' && <Skeleton className="h-8 w-full rounded-md" />}
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton
          key={i}
          className={cn(
            'h-4 rounded-sm',
            ['w-full', 'w-11/12', 'w-4/5', 'w-full', 'w-3/4', 'w-5/6'][i]
          )}
        />
      ))}
    </div>
  );
}

function QueryError({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <Banner
      intent="error"
      title="Couldn’t load data"
      action={
        onRetry && (
          <Button size="sm" color="secondary" variant="outlined" onClick={onRetry}>
            Retry
          </Button>
        )
      }
    >
      <span className="font-mono text-xs [overflow-wrap:anywhere]">{message}</span>
    </Banner>
  );
}

/** A "Load more" control for a paged useQuery result; renders nothing when there is no next page. */
export function LoadMore({
  result,
  label = 'Load more',
}: {
  result: Pick<QueryResult, 'hasMore' | 'loadingMore' | 'loadMore'>;
  label?: string;
}) {
  if (!result.hasMore) return null;
  return (
    <div className="flex justify-center py-space-2">
      <Button
        size="sm"
        color="secondary"
        variant="outlined"
        loading={result.loadingMore}
        onClick={result.loadMore}
      >
        {label}
      </Button>
    </div>
  );
}

export type QueryProps<T, I> = QueryOptions & {
  operation: string | null | undefined;
  /** Shown when the list is empty (arrays only). */
  emptyText?: string;
  emptyDescription?: string;
  skeleton?: SkeletonKind;
  /** Adds a Load more button under the children while there is a next page. Default true. */
  showLoadMore?: boolean;
  children: (result: QueryResult<T, I>) => ReactNode;
};

/** useQuery plus its states: skeleton while first loading, error with Retry, empty state, then children. */
export function Query<T = unknown, I = Record<string, unknown>>({
  operation,
  emptyText = 'Nothing here yet',
  emptyDescription,
  skeleton = 'list',
  showLoadMore = true,
  children,
  ...options
}: QueryProps<T, I>) {
  const result = useQuery<T, I>(operation, options);
  if (result.error && result.data == null)
    return <QueryError message={result.error} onRetry={result.refresh} />;
  if (result.data == null) return result.loading ? <LoadingSkeleton kind={skeleton} /> : null;
  const empty = Array.isArray(result.items)
    ? result.items.length === 0
    : Array.isArray(result.data) && result.data.length === 0;
  return (
    <div className="relative flex min-w-0 flex-col gap-space-3">
      {result.loading && (
        <LinearProgress className="absolute inset-x-0 top-0" aria-label="Refreshing" />
      )}
      {result.error && <QueryError message={result.error} onRetry={result.refresh} />}
      {empty ? (
        <EmptyState size="sm" title={emptyText} description={emptyDescription} />
      ) : (
        children(result)
      )}
      {showLoadMore && !empty && <LoadMore result={result} />}
    </div>
  );
}
