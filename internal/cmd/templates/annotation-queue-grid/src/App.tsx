import { useEffect, useMemo, useRef, useState } from 'react';
import { DataGrid } from './components/DataGrid';
import { QueueBar } from './components/QueueBar';
import { fetchFeedbackConfigs, fetchFeedbacks, fetchQueue, markRunComplete } from './api';
import { useRunSection } from './hooks/useRunSection';
import type { AnnotationQueue, FeedbackConfig, FeedbackItem } from './types';

interface Props {
  /** Optional starting queue. Apps are uniform now and normally receive {}, so
   * this is usually empty and the user picks a queue via the QueueBar. */
  queueId?: string;
  /** Host render metadata; `metadata.mode` is "dark"|"light", applied by the sandbox — no branching needed. */
  metadata?: RenderMetadata;
}

// feedbackByRun[runId][feedbackKey] → the latest saved feedback for that cell.
export type FeedbackByRun = Record<string, Record<string, FeedbackItem>>;

export function App({ queueId: initialQueueId }: Props) {
  const [queueId, setQueueId] = useState(initialQueueId ?? '');
  const [queue, setQueue] = useState<AnnotationQueue | null>(null);
  const [configs, setConfigs] = useState<Record<string, FeedbackConfig>>({});
  const [feedbackByRun, setFeedbackByRun] = useState<FeedbackByRun>({});
  const [activeRow, setActiveRow] = useState(0);
  const [expandedRunId, setExpandedRunId] = useState<string | null>(null);
  const [completeError, setCompleteError] = useState<string | null>(null);

  const section = useRunSection(queueId || undefined, 'needs_my_review');
  const rows = section.runs;

  // Refs so the window keydown listener always calls the latest handlers
  // (same trick as the 3-pane App.tsx).
  const completeRef = useRef<(index: number) => void>(() => {});
  const moveRef = useRef<(direction: 'up' | 'down') => void>(() => {});

  // Columns are the rubric's feedback keys.
  const columns = useMemo(() => queue?.rubric_items ?? [], [queue?.rubric_items]);

  // Reset per-queue UI state when the queue changes (run loading itself is
  // owned by useRunSection, keyed on the same queueId).
  useEffect(() => {
    setQueue(null);
    setActiveRow(0);
    setExpandedRunId(null);
    setCompleteError(null);
    if (!queueId) return;
    fetchQueue(queueId)
      .then(setQueue)
      .catch((e) => console.error('Failed to load queue', e));
  }, [queueId]);

  // Fetch each key's type/min/max/categories config — the rubric item carries none (see RubricItem).
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

  // Load existing feedback for every loaded row so already-scored cells are
  // prefilled — re-runs as loadMore brings in new rows, only fetching the
  // ones we don't already have.
  useEffect(() => {
    const idsToFetch = rows.map((r) => r.id).filter((id) => !(id in feedbackByRun));
    if (idsToFetch.length === 0) return;
    let cancelled = false;
    Promise.all(
      idsToFetch.map((id) =>
        fetchFeedbacks(id)
          .then((items) => [id, items] as const)
          .catch((e) => {
            console.error('Failed to load feedback for run', id, e);
            return [id, [] as FeedbackItem[]] as const;
          })
      )
    ).then((entries) => {
      if (cancelled) return;
      setFeedbackByRun((prev) => {
        const next = { ...prev };
        for (const [runId, items] of entries) {
          const byKey: Record<string, FeedbackItem> = {};
          for (const item of items) byKey[item.key] = item;
          next[runId] = byKey;
        }
        return next;
      });
    });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows.map((r) => r.id).join(',')]);

  function handleCellSaved(runId: string, feedback: FeedbackItem) {
    setFeedbackByRun((prev) => ({
      ...prev,
      [runId]: { ...(prev[runId] ?? {}), [feedback.key]: feedback },
    }));
  }

  function handleCellDeleted(runId: string, feedbackKey: string) {
    setFeedbackByRun((prev) => {
      const row = { ...(prev[runId] ?? {}) };
      delete row[feedbackKey];
      return { ...prev, [runId]: row };
    });
  }

  // Optimistically remove a completed row and keep the active row within
  // bounds; restore it (and surface the error) if the server rejects it.
  function handleComplete(index: number) {
    const run = rows[index];
    if (!run) return;
    const queueRunId = run.queue_run_id;
    setCompleteError(null);
    section.removeRun(queueRunId);
    setActiveRow((prev) => Math.max(0, Math.min(prev, rows.length - 2)));
    markRunComplete(queueRunId).catch((e) => {
      console.error('Failed to mark complete — restoring row', e);
      section.restoreRun(run, index);
      setCompleteError(e instanceof Error ? e.message : String(e));
    });
  }
  useEffect(() => {
    completeRef.current = handleComplete;
  });

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

  // ArrowUp/ArrowDown move between rows (ignored mid-edit); Escape blurs the cell.
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      const el = document.activeElement as HTMLElement | null;
      const tag = el?.tagName;
      const inInput = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el?.isContentEditable;

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
          <span className="text-sm text-tertiary">Select an annotation queue to start reviewing.</span>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-surface-level-1">
      <QueueBar selectedQueueId={queueId} onSelect={setQueueId} />
      <div className="flex min-h-0 flex-1 flex-col p-space-4">
        <DataGrid
          queue={queue}
          columns={columns}
          configs={configs}
          rows={rows}
          rowsLoading={section.loading}
          loadingMore={section.loadingMore}
          hasMore={section.hasMore}
          onLoadMore={section.loadMore}
          feedbackByRun={feedbackByRun}
          activeRow={activeRow}
          expandedRunId={expandedRunId}
          onToggleExpand={(runId) => setExpandedRunId((prev) => (prev === runId ? null : runId))}
          completeError={completeError}
          onActivateRow={setActiveRow}
          onCellSaved={handleCellSaved}
          onCellDeleted={handleCellDeleted}
          onComplete={(index) => completeRef.current(index)}
        />
      </div>
    </div>
  );
}
