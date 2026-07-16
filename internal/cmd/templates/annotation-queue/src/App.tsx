import { useEffect, useRef, useState } from 'react';
import { FeedbackPanel } from './components/FeedbackPanel';
import { LinearProgress } from './components/LinearProgress';
import { RunList } from './components/RunList';
import { RunViewer } from './components/RunViewer';
import { fetchQueue, fetchQueueRuns, markRunComplete } from './api';
import type { AnnotationQueue, AnnotationQueueRun } from './types';

interface Props {
  /** The only context this app receives — everything else (run list, run
   * detail, feedback, ...) is fetched itself via window.langsmith.call. */
  queueId: string;
  /** Host render metadata; `metadata.mode` is "dark"|"light". The sandbox sets
   * html.dark from it, so this UI needs no branching. */
  metadata?: RenderMetadata;
}

export function App({ queueId }: Props) {
  const [queue, setQueue] = useState<AnnotationQueue | null>(null);
  const [needsReviewRuns, setNeedsReviewRuns] = useState<AnnotationQueueRun[]>([]);
  const [needsOthersReviewRuns, setNeedsOthersReviewRuns] = useState<AnnotationQueueRun[]>([]);
  const [completedRuns, setCompletedRuns] = useState<AnnotationQueueRun[]>([]);
  const [runsLoading, setRunsLoading] = useState(false);
  const [selectedQueueRunId, setSelectedQueueRunId] = useState<string | undefined>(undefined);

  // Keep a ref to the complete handler so hotkey always calls the latest version
  const completeRef = useRef<(() => Promise<void>) | null>(null);
  // Same trick for h/l navigation, which needs the latest run list + selection
  const goToAdjacentRunRef = useRef<(direction: 'prev' | 'next') => void>(() => {});

  // Load queue metadata and runs whenever the queue changes
  useEffect(() => {
    if (!queueId) return;
    fetchQueue(queueId)
      .then(setQueue)
      .catch((e) => console.error('Failed to load queue', e));
    setRunsLoading(true);
    Promise.all([
      fetchQueueRuns(queueId, 'needs_my_review'),
      fetchQueueRuns(queueId, 'completed'),
    ])
      .then(([needsReview, completed]) => {
        setNeedsReviewRuns(needsReview);
        setCompletedRuns(completed);
      })
      .catch((e) => console.error('Failed to load runs', e))
      .finally(() => setRunsLoading(false));
  }, [queueId]);

  // Fetch needs_others_review when multi-reviewer queue
  useEffect(() => {
    if (!queueId || !queue || !queue.num_reviewers_per_item || queue.num_reviewers_per_item <= 1) {
      setNeedsOthersReviewRuns([]);
      return;
    }
    fetchQueueRuns(queueId, 'needs_others_review')
      .then(setNeedsOthersReviewRuns)
      .catch((e) => console.error('Failed to load needs_others_review runs', e));
  }, [queueId, queue?.id, queue?.num_reviewers_per_item]);

  // Keyboard shortcuts: H=prev, L=next, Esc=blur, Cmd/Ctrl+Enter=complete
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      const tag = (document.activeElement as HTMLElement | null)?.tagName;
      const inInput = tag === 'INPUT' || tag === 'TEXTAREA' || (document.activeElement as HTMLElement | null)?.isContentEditable;

      if (e.key === 'Escape') {
        (document.activeElement as HTMLElement | null)?.blur();
        return;
      }

      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
        e.preventDefault();
        completeRef.current?.();
        return;
      }

      if (!inInput) {
        if (e.key === 'h') {
          goToAdjacentRunRef.current('prev');
        }
        if (e.key === 'l') {
          goToAdjacentRunRef.current('next');
        }
      }
    }
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  const allRuns = [...needsReviewRuns, ...needsOthersReviewRuns, ...completedRuns];
  const selectedRun =
    allRuns.find((r) => r.queue_run_id === selectedQueueRunId) ?? needsReviewRuns[0] ?? null;

  function handleSelectRun(queueRunId: string) {
    setSelectedQueueRunId(queueRunId);
  }

  // h/l step through the "Needs Review" list. No-op at a list boundary.
  function goToAdjacentRun(direction: 'prev' | 'next') {
    const list = needsReviewRuns;
    if (list.length === 0) return;
    const idx = list.findIndex((r) => r.queue_run_id === selectedQueueRunId);
    if (idx === -1) {
      handleSelectRun(list[0].queue_run_id);
      return;
    }
    const nextIdx = direction === 'prev' ? idx - 1 : idx + 1;
    if (nextIdx < 0 || nextIdx >= list.length) return;
    handleSelectRun(list[nextIdx].queue_run_id);
  }

  useEffect(() => {
    goToAdjacentRunRef.current = goToAdjacentRun;
  });

  async function handleComplete() {
    if (!selectedRun) return;
    const queueRunId = selectedRun.queue_run_id;
    try {
      await markRunComplete(queueRunId);
      const remainingNeedsReview = needsReviewRuns.filter((r) => r.queue_run_id !== queueRunId);
      setNeedsReviewRuns(remainingNeedsReview);
      setCompletedRuns((prev) => [
        { ...selectedRun, last_reviewed_time: new Date().toISOString() },
        ...prev,
      ]);
      setSelectedQueueRunId(remainingNeedsReview[0]?.queue_run_id);
    } catch (e) {
      console.error('Failed to mark complete', e);
    }
  }

  useEffect(() => {
    completeRef.current = handleComplete;
  });

  if (!queueId) {
    return (
      <div className="flex h-screen flex-col bg-surface-level-1">
        <div className="flex flex-1 items-center justify-center">
          <span className="text-sm text-tertiary">
            No queueId in context — this app must be set as an annotation queue's active layout, or run with --queue-id locally.
          </span>
        </div>
      </div>
    );
  }

  if (!queue) {
    return (
      <div className="flex h-screen flex-col bg-surface-level-1">
        <LinearProgress />
      </div>
    );
  }

  const contentLoading = runsLoading && !selectedRun;

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-surface-level-1 p-space-4">
      <div className="flex min-h-0 flex-1 overflow-hidden rounded-lg border border-secondary">
        {/* Left: 280px run list */}
        <div className="flex h-full w-[280px] min-w-[280px] max-w-[280px] flex-col overflow-hidden">
          <RunList
            needsReviewRuns={needsReviewRuns}
            needsOthersReviewRuns={needsOthersReviewRuns}
            completedRuns={completedRuns}
            selectedQueueRunId={selectedQueueRunId ?? selectedRun?.queue_run_id}
            loading={runsLoading}
            numReviewersPerItem={queue.num_reviewers_per_item}
            onSelectRun={handleSelectRun}
          />
        </div>

        {/* Center: flex-1 inputs/outputs */}
        <div className="relative flex min-w-0 flex-1 flex-col overflow-auto">
          {selectedRun || contentLoading ? (
            <RunViewer
              inputs={selectedRun?.inputs ?? null}
              outputs={selectedRun?.outputs ?? null}
              error={selectedRun?.error}
              loading={contentLoading}
            />
          ) : (
            <div className="flex flex-1 items-center justify-center">
              <span className="text-sm text-tertiary">Select a run to review</span>
            </div>
          )}
        </div>

        {/* Right: 450px feedback panel */}
        <div className="flex h-full w-[450px] min-w-[450px] max-w-[450px] flex-col border-l border-secondary">
          <FeedbackPanel
            queue={queue}
            runId={selectedRun?.id}
            queueRunId={selectedRun?.queue_run_id}
            onComplete={handleComplete}
            totalNeedsReview={needsReviewRuns.length}
          />
        </div>
      </div>
    </div>
  );
}
