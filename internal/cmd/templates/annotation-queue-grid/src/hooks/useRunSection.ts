import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchQueueRuns, fetchQueueRunsSize } from '../api';
import type { AnnotationQueueRunSectionStatus } from '../api';
import type { AnnotationQueueRun } from '../types';

const PAGE_SIZE = 50;

export interface RunSection {
  runs: AnnotationQueueRun[];
  /** Exact count for this status, from fetchQueueRunsSize — not an estimate,
   * so callers can display it directly instead of deriving from runs.length. */
  total: number;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  loadMore: () => void;
  removeRun: (queueRunId: string) => void;
  restoreRun: (run: AnnotationQueueRun, atIndex: number) => void;
}

/**
 * Loads one status section of an annotation queue's runs, a page at a time.
 * Total count comes from a separate /size call (not a response header — see
 * fetchQueueRunsSize), and that total caps how far loadMore will page.
 */
export function useRunSection(
  queueId: string | undefined,
  status: AnnotationQueueRunSectionStatus
): RunSection {
  const [runs, setRuns] = useState<AnnotationQueueRun[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const loadingMoreRef = useRef(false);
  const runsRef = useRef<AnnotationQueueRun[]>([]);
  runsRef.current = runs;

  useEffect(() => {
    setRuns([]);
    setTotal(0);
    if (!queueId) return;
    let cancelled = false;
    setLoading(true);
    Promise.all([
      fetchQueueRuns(queueId, status, PAGE_SIZE, 0),
      fetchQueueRunsSize(queueId, status),
    ])
      .then(([firstPage, size]) => {
        if (cancelled) return;
        setRuns(firstPage);
        setTotal(size);
      })
      .catch((e) => console.error(`Failed to load ${status} runs`, e))
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [queueId, status]);

  const loadMore = useCallback(() => {
    if (!queueId || loadingMoreRef.current) return;
    if (runsRef.current.length >= total) return;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    fetchQueueRuns(queueId, status, PAGE_SIZE, runsRef.current.length)
      .then((nextPage) => {
        setRuns((prev) => {
          // Offsets can drift as runs complete elsewhere while this page
          // loads; dedupe defensively rather than assuming clean boundaries.
          const seen = new Set(prev.map((r) => r.queue_run_id));
          const fresh = nextPage.filter((r) => !seen.has(r.queue_run_id));
          return [...prev, ...fresh];
        });
      })
      .catch((e) => console.error(`Failed to load more ${status} runs`, e))
      .finally(() => {
        loadingMoreRef.current = false;
        setLoadingMore(false);
      });
  }, [queueId, status, total]);

  const removeRun = useCallback((queueRunId: string) => {
    setRuns((prev) => prev.filter((r) => r.queue_run_id !== queueRunId));
    setTotal((prev) => Math.max(0, prev - 1));
  }, []);

  const restoreRun = useCallback((run: AnnotationQueueRun, atIndex: number) => {
    setRuns((prev) => {
      if (prev.some((r) => r.queue_run_id === run.queue_run_id)) return prev;
      const next = [...prev];
      next.splice(Math.min(atIndex, next.length), 0, run);
      return next;
    });
    setTotal((prev) => prev + 1);
  }, []);

  return {
    runs,
    total,
    loading,
    loadingMore,
    hasMore: runs.length < total,
    loadMore,
    removeRun,
    restoreRun,
  };
}
