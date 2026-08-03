import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchQueueItems, fetchQueueItemsCount } from '../api';
import type { QueueItemSectionStatus } from '../api';
import type { QueueItem } from '../types';

const PAGE_SIZE = 50;

export interface ItemSection {
  items: QueueItem[];
  /** Exact count for this status from fetchQueueItemsCount. */
  total: number;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  loadMore: () => void;
  removeItem: (itemId: string) => void;
  prependItem: (item: QueueItem) => void;
  restoreItem: (item: QueueItem, atIndex: number) => void;
}

/**
 * Loads one status section of an annotation queue's items with cursor
 * pagination. UI status "completed" is requested as API "archived".
 */
export function useItemSection(
  queueId: string | undefined,
  status: QueueItemSectionStatus,
  enabled = true
): ItemSection {
  const [items, setItems] = useState<QueueItem[]>([]);
  const [total, setTotal] = useState(0);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const loadingMoreRef = useRef(false);
  const nextCursorRef = useRef<string | null>(null);
  nextCursorRef.current = nextCursor;

  useEffect(() => {
    setItems([]);
    setTotal(0);
    setNextCursor(null);
    if (!queueId || !enabled) return;
    let cancelled = false;
    setLoading(true);
    Promise.all([
      fetchQueueItems(queueId, status, PAGE_SIZE),
      fetchQueueItemsCount(queueId, status),
    ])
      .then(([page, size]) => {
        if (cancelled) return;
        setItems(page.items ?? []);
        setNextCursor(page.next_cursor ?? null);
        setTotal(size);
      })
      .catch((e) => console.error(`Failed to load ${status} items`, e))
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [queueId, status, enabled]);

  const loadMore = useCallback(() => {
    if (!queueId || loadingMoreRef.current) return;
    const cursor = nextCursorRef.current;
    if (!cursor) return;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    fetchQueueItems(queueId, status, PAGE_SIZE, cursor)
      .then((page) => {
        setItems((prev) => {
          const seen = new Set(prev.map((i) => i.id));
          const fresh = (page.items ?? []).filter((i) => !seen.has(i.id));
          return [...prev, ...fresh];
        });
        setNextCursor(page.next_cursor ?? null);
      })
      .catch((e) => console.error(`Failed to load more ${status} items`, e))
      .finally(() => {
        loadingMoreRef.current = false;
        setLoadingMore(false);
      });
  }, [queueId, status]);

  const removeItem = useCallback((itemId: string) => {
    setItems((prev) => prev.filter((i) => i.id !== itemId));
    setTotal((prev) => Math.max(0, prev - 1));
  }, []);

  const prependItem = useCallback((item: QueueItem) => {
    setItems((prev) => [item, ...prev]);
    setTotal((prev) => prev + 1);
  }, []);

  const restoreItem = useCallback((item: QueueItem, atIndex: number) => {
    setItems((prev) => {
      if (prev.some((i) => i.id === item.id)) return prev;
      const next = [...prev];
      next.splice(Math.min(atIndex, next.length), 0, item);
      return next;
    });
    setTotal((prev) => prev + 1);
  }, []);

  return {
    items,
    total,
    loading,
    loadingMore,
    hasMore: nextCursor != null,
    loadMore,
    removeItem,
    prependItem,
    restoreItem,
  };
}
