import { useEffect, useMemo, useRef, useState } from 'react';
import { DataGrid } from './components/DataGrid';
import { QueueBar } from './components/QueueBar';
import {
  fetchFeedbackConfigs,
  fetchFeedbacksForRun,
  fetchFeedbacksForThread,
  fetchQueue,
  markItemComplete,
} from './api';
import { useHydratedItem } from './hooks/useHydratedItem';
import { useItemSection } from './hooks/useItemSection';
import type { AnnotationQueue, FeedbackConfig, FeedbackItem, QueueItem } from './types';
import { feedbackSubjectKey } from './types';

interface Props {
  /** Optional starting queue. Apps are uniform now and normally receive {}, so
   * this is usually empty and the user picks a queue via the QueueBar. */
  queueId?: string;
  /** Host render metadata; `metadata.mode` is "dark"|"light", applied by the sandbox — no branching needed. */
  metadata?: RenderMetadata;
}

// feedbackBySubject[runId|threadId][feedbackKey] → latest saved feedback.
export type FeedbackBySubject = Record<string, Record<string, FeedbackItem>>;

export function App({ queueId: initialQueueId }: Props) {
  const [queueId, setQueueId] = useState(initialQueueId ?? '');
  const [queue, setQueue] = useState<AnnotationQueue | null>(null);
  const [configs, setConfigs] = useState<Record<string, FeedbackConfig>>({});
  const [feedbackBySubject, setFeedbackBySubject] = useState<FeedbackBySubject>({});
  const [activeRow, setActiveRow] = useState(0);
  const [expandedItemId, setExpandedItemId] = useState<string | null>(null);
  const [completeError, setCompleteError] = useState<string | null>(null);
  const [selectedItemIds, setSelectedItemIds] = useState<Set<string>>(new Set());

  const section = useItemSection(queueId || undefined, 'needs_my_review');
  const rows = section.items;

  const expandedListItem =
    rows.find((r) => r.id === expandedItemId) ?? null;
  const { item: hydratedExpanded, loading: expanding } = useHydratedItem(expandedListItem);

  const displayRows: QueueItem[] = useMemo(() => {
    if (!hydratedExpanded) return rows;
    return rows.map((r) => (r.id === hydratedExpanded.id ? hydratedExpanded : r));
  }, [rows, hydratedExpanded]);

  const completeRef = useRef<(index: number) => void>(() => {});
  const moveRef = useRef<(direction: 'up' | 'down') => void>(() => {});

  const columns = useMemo(
    () => (queue?.rubric_items ?? []).filter((item) => !item.is_assertion),
    [queue?.rubric_items]
  );

  useEffect(() => {
    setQueue(null);
    setActiveRow(0);
    setExpandedItemId(null);
    setCompleteError(null);
    setSelectedItemIds(new Set());
    if (!queueId) return;
    fetchQueue(queueId)
      .then(setQueue)
      .catch((e) => console.error('Failed to load queue', e));
  }, [queueId]);

  const columnKeys = useMemo(() => columns.map((c) => c.feedback_key), [columns]);
  useEffect(() => {
    if (columnKeys.length === 0) {
      setConfigs({});
      return;
    }
    fetchFeedbackConfigs(columnKeys)
      .then((cfgs) => {
        const map: Record<string, FeedbackConfig> = {};
        for (const c of cfgs) map[c.feedback_key] = c.feedback_config;
        setConfigs(map);
      })
      .catch((e) => console.error('Failed to load feedback configs', e));
  }, [columnKeys.join(',')]);

  useEffect(() => {
    const toFetch = rows
      .map((r) => ({ item: r, key: feedbackSubjectKey(r) }))
      .filter(({ key }) => key && !(key in feedbackBySubject)) as {
      item: QueueItem;
      key: string;
    }[];
    if (toFetch.length === 0) return;
    let cancelled = false;
    Promise.all(
      toFetch.map(({ item, key }) => {
        const load =
          item.item_type === 'THREAD'
            ? fetchFeedbacksForThread(key, item.project_id)
            : fetchFeedbacksForRun(key);
        return load
          .then((items) => [key, items] as const)
          .catch((e) => {
            console.error('Failed to load feedback for', key, e);
            return [key, [] as FeedbackItem[]] as const;
          });
      })
    ).then((entries) => {
      if (cancelled) return;
      setFeedbackBySubject((prev) => {
        const next = { ...prev };
        for (const [subject, items] of entries) {
          const byKey: Record<string, FeedbackItem> = {};
          for (const item of items) byKey[item.key] = item;
          next[subject] = byKey;
        }
        return next;
      });
    });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows.map((r) => `${r.id}:${r.item_type}`).join(',')]);

  function handleCellSaved(subjectKey: string, feedback: FeedbackItem) {
    setFeedbackBySubject((prev) => ({
      ...prev,
      [subjectKey]: { ...(prev[subjectKey] ?? {}), [feedback.key]: feedback },
    }));
  }

  function handleCellDeleted(subjectKey: string, feedbackKey: string) {
    setFeedbackBySubject((prev) => {
      const row = { ...(prev[subjectKey] ?? {}) };
      delete row[feedbackKey];
      return { ...prev, [subjectKey]: row };
    });
  }

  function handleComplete(index: number) {
    const item = rows[index];
    if (!item) return;
    const itemId = item.id;
    setCompleteError(null);
    section.removeItem(itemId);
    setActiveRow((prev) => Math.max(0, Math.min(prev, rows.length - 2)));
    markItemComplete(itemId).catch((e) => {
      console.error('Failed to mark complete — restoring row', e);
      section.restoreItem(item, index);
      setCompleteError(e instanceof Error ? e.message : String(e));
    });
  }
  useEffect(() => {
    completeRef.current = handleComplete;
  });

  function toggleRowSelected(itemId: string) {
    setSelectedItemIds((prev) => {
      const next = new Set(prev);
      if (next.has(itemId)) next.delete(itemId);
      else next.add(itemId);
      return next;
    });
  }

  function toggleSelectAll() {
    setSelectedItemIds((prev) =>
      prev.size === rows.length ? new Set() : new Set(rows.map((r) => r.id))
    );
  }

  async function handleBulkComplete() {
    const targets = rows
      .map((item, index) => ({ item, index }))
      .filter(({ item }) => selectedItemIds.has(item.id));
    if (targets.length === 0) return;

    setCompleteError(null);
    setSelectedItemIds(new Set());
    for (const { item } of targets) section.removeItem(item.id);

    const results = await Promise.allSettled(
      targets.map(({ item }) => markItemComplete(item.id))
    );
    const failures = targets.filter((_, i) => results[i].status === 'rejected');
    if (failures.length > 0) {
      failures.forEach(({ item, index }) => section.restoreItem(item, index));
      setCompleteError(
        `Failed to mark ${failures.length} of ${targets.length} item${targets.length === 1 ? '' : 's'} complete`
      );
    }
  }

  function moveActiveRow(direction: 'up' | 'down') {
    setActiveRow((prev) => {
      if (rows.length === 0) return 0;
      const next = direction === 'up' ? prev - 1 : prev + 1;
      if (next < 0 || next >= rows.length) return prev;
      return next;
    });
  }
  useEffect(() => {
    moveRef.current = moveActiveRow;
  });

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      const el = document.activeElement as HTMLElement | null;
      const tag = el?.tagName;
      const inInput =
        tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el?.isContentEditable;

      if (e.key === 'Escape') {
        el?.blur();
        return;
      }
      if (inInput) return;
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        moveRef.current('up');
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        moveRef.current('down');
      }
    }
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  if (!queueId) {
    return (
      <div className="flex h-screen flex-col bg-surface-level-1">
        <QueueBar selectedQueueId={queueId} onSelect={setQueueId} />
        <div className="flex flex-1 items-center justify-center">
          <span className="text-sm text-tertiary">
            Select an annotation queue to start reviewing.
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-surface-level-1">
      <QueueBar selectedQueueId={queueId} onSelect={setQueueId} />
      <div className="flex min-h-0 flex-1 flex-col p-4">
        <DataGrid
          queue={queue}
          columns={columns}
          configs={configs}
          rows={displayRows}
          total={section.total}
          rowsLoading={section.loading}
          loadingMore={section.loadingMore}
          hasMore={section.hasMore}
          onLoadMore={section.loadMore}
          feedbackBySubject={feedbackBySubject}
          activeRow={activeRow}
          expandedItemId={expandedItemId}
          expandLoading={expanding}
          onToggleExpand={(itemId) =>
            setExpandedItemId((prev) => (prev === itemId ? null : itemId))
          }
          completeError={completeError}
          selectedItemIds={selectedItemIds}
          onToggleRowSelected={toggleRowSelected}
          onToggleSelectAll={toggleSelectAll}
          onBulkComplete={handleBulkComplete}
          onActivateRow={setActiveRow}
          onCellSaved={handleCellSaved}
          onCellDeleted={handleCellDeleted}
          onComplete={(index) => completeRef.current(index)}
        />
      </div>
    </div>
  );
}
