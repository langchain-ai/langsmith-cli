import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchQueueRuns, fetchQueueRunsSize } from '../api';
import type { AnnotationQueueRunSectionStatus } from '../api';
import type { AnnotationQueueRun } from '../types';

const PAGE_SIZE = 50;

export interface RunSection {
  runs: AnnotationQueueRun[];
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  loadMore: () => void;
  removeRun: (queueRunId: string) => void;
  prependRun: (run: AnnotationQueueRun) => void;
}

/**
 * Loads one status section (needs_my_review / needs_others_review /
 * completed) of an annotation queue's runs, a page at a time. Mirrors the
 * production AQ UI's approach: total count comes from a separate /size
 * call (not a response header — see fetchQueueRunsSize), and that total
 * caps how far loadMore will page.
 */
export function useRunSection(
  queueId: string | undefined,
  status: AnnotationQueueRunSectionStatus,
  enabled = true
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
    if (!queueId || !enabled) return;
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
  }, [queueId, status, enabled]);

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

  const prependRun = useCallback((run: AnnotationQueueRun) => {
    setRuns((prev) => [run, ...prev]);
    setTotal((prev) => prev + 1);
  }, []);

  return {
    runs,
    loading,
    loadingMore,
    hasMore: runs.length < total,
    loadMore,
    removeRun,
    prependRun,
  };
}
