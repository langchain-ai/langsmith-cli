import { useEffect, useMemo, useRef, useState } from 'react';
import { DataGrid } from './components/DataGrid';
import {
  fetchFeedbackConfigs,
  fetchFeedbacks,
  fetchQueue,
  fetchQueueRuns,
  markRunComplete,
} from './api';
import type {
  AnnotationQueue,
  AnnotationQueueRun,
  FeedbackConfig,
  FeedbackItem,
  RubricItem,
} from './types';

interface Props {
  /** The only context this app receives — everything else is fetched via window.langsmith.call. */
  queueId: string;
  /** Host render metadata; `metadata.mode` is "dark"|"light", applied by the sandbox — no branching needed. */
  metadata?: RenderMetadata;
}

// feedbackByRun[runId][feedbackKey] → the latest saved feedback for that cell.
export type FeedbackByRun = Record<string, Record<string, FeedbackItem>>;

export function App({ queueId }: Props) {
  const [queue, setQueue] = useState<AnnotationQueue | null>(null);
  const [rows, setRows] = useState<AnnotationQueueRun[]>([]);
  const [rowsLoading, setRowsLoading] = useState(false);
  const [configs, setConfigs] = useState<Record<string, FeedbackConfig>>({});
  const [feedbackByRun, setFeedbackByRun] = useState<FeedbackByRun>({});
  const [activeRow, setActiveRow] = useState(0);

  // Refs so the window keydown listener always calls the latest handlers
  // (same trick as the 3-pane App.tsx).
  const completeRef = useRef<(index: number) => void>(() => {});
  const moveRef = useRef<(direction: 'up' | 'down') => void>(() => {});

  // Columns are the rubric's feedback keys; assertion-flagged items aren't
  // scored, so they're excluded (assertionCount lets the header note them).
  const columns: RubricItem[] = useMemo(
    () => (queue?.rubric_items ?? []).filter((item) => !item.is_assertion),
    [queue?.rubric_items]
  );
  const assertionCount = useMemo(
    () => (queue?.rubric_items ?? []).filter((item) => item.is_assertion).length,
    [queue?.rubric_items]
  );

  // The runs needing this reviewer — the editable rows. A run leaves once marked Done.
  useEffect(() => {
    if (!queueId) return;
    fetchQueue(queueId)
      .then(setQueue)
      .catch((e) => console.error('Failed to load queue', e));
    setRowsLoading(true);
    fetchQueueRuns(queueId, 'needs_my_review')
      .then(setRows)
      .catch((e) => console.error('Failed to load runs', e))
      .finally(() => setRowsLoading(false));
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

  // Load existing feedback for every row so already-scored cells are prefilled.
  useEffect(() => {
    if (rows.length === 0) {
      setFeedbackByRun({});
      return;
    }
    let cancelled = false;
    Promise.all(
      rows.map((r) =>
        fetchFeedbacks(r.id)
          .then((items) => [r.id, items] as const)
          .catch((e) => {
            console.error('Failed to load feedback for run', r.id, e);
            return [r.id, [] as FeedbackItem[]] as const;
          })
      )
    ).then((entries) => {
      if (cancelled) return;
      const next: FeedbackByRun = {};
      for (const [runId, items] of entries) {
        const byKey: Record<string, FeedbackItem> = {};
        for (const item of items) byKey[item.key] = item;
        next[runId] = byKey;
      }
      setFeedbackByRun(next);
    });
    return () => {
      cancelled = true;
    };
    // Re-run only when the set of row ids changes, not on every cell edit.
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

  // Optimistically remove a completed row (mirrors markRunComplete usage in the
  // 3-pane App.tsx) and keep the active row within bounds.
  function handleComplete(index: number) {
    const run = rows[index];
    if (!run) return;
    const queueRunId = run.queue_run_id;
    setRows((prev) => prev.filter((r) => r.queue_run_id !== queueRunId));
    setActiveRow((prev) => Math.max(0, Math.min(prev, rows.length - 2)));
    markRunComplete(queueRunId).catch((e) => {
      console.error('Failed to mark complete — restoring row', e);
      // Put it back if the server rejected the completion.
      setRows((prev) => {
        if (prev.some((r) => r.queue_run_id === queueRunId)) return prev;
        const restored = [...prev];
        restored.splice(Math.min(index, restored.length), 0, run);
        return restored;
      });
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
      <div className="flex h-screen flex-col items-center justify-center bg-surface-level-1">
        <span className="text-sm text-tertiary">
          No queueId in context — this app must be set as an annotation queue's active layout, or run with --queue-id locally.
        </span>
      </div>
    );
  }

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-surface-level-1 p-space-4">
      <DataGrid
        queue={queue}
        columns={columns}
        configs={configs}
        rows={rows}
        rowsLoading={rowsLoading}
        feedbackByRun={feedbackByRun}
        activeRow={activeRow}
        assertionCount={assertionCount}
        onActivateRow={setActiveRow}
        onCellSaved={handleCellSaved}
        onCellDeleted={handleCellDeleted}
        onComplete={(index) => completeRef.current(index)}
      />
    </div>
  );
}
