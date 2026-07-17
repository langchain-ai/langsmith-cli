import { useEffect, useRef, useState } from 'react';
import { FeedbackPanel } from './components/FeedbackPanel';
import { LinearProgress } from './components/LinearProgress';
import { QueueBar } from './components/QueueBar';
import { RunList } from './components/RunList';
import { RunViewer } from './components/RunViewer';
import { fetchQueue, markRunComplete } from './api';
import { useRunSection } from './hooks/useRunSection';
import type { AnnotationQueue } from './types';

interface Props {
  /** Optional starting queue. Apps are uniform now and normally receive {},
   * so this is usually empty and the user picks a queue via the QueueBar. */
  queueId?: string;
  /** Host render metadata; `metadata.mode` is "dark"|"light". The sandbox sets
   * html.dark from it, so this UI needs no branching. */
  metadata?: RenderMetadata;
}

export function App({ queueId: initialQueueId }: Props) {
  const [queueId, setQueueId] = useState(initialQueueId ?? '');
  const [queue, setQueue] = useState<AnnotationQueue | null>(null);
  const [selectedQueueRunId, setSelectedQueueRunId] = useState<string | undefined>(undefined);

  const isMultiReviewer = !!queue?.num_reviewers_per_item && queue.num_reviewers_per_item > 1;
  const needsReview = useRunSection(queueId || undefined, 'needs_my_review');
  const needsOthersReview = useRunSection(queueId || undefined, 'needs_others_review', isMultiReviewer);
  const completed = useRunSection(queueId || undefined, 'completed');

  // Keep a ref to the complete handler so hotkey always calls the latest version
  const completeRef = useRef<(() => Promise<void>) | null>(null);
  // Same trick for h/l navigation, which needs the latest run list + selection
  const goToAdjacentRunRef = useRef<(direction: 'prev' | 'next') => void>(() => {});

  // Load queue metadata whenever the queue changes. Run sections load
  // themselves (see useRunSection), keyed on the same queueId.
  useEffect(() => {
    // Clear the previous queue's rubric first so switching queues doesn't
    // flash stale data from the one before.
    setQueue(null);
    setSelectedQueueRunId(undefined);
    if (!queueId) return;
    fetchQueue(queueId)
      .then(setQueue)
      .catch((e) => console.error('Failed to load queue', e));
  }, [queueId]);

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

  const allRuns = [...needsReview.runs, ...needsOthersReview.runs, ...completed.runs];
  const selectedRun =
    allRuns.find((r) => r.queue_run_id === selectedQueueRunId) ?? needsReview.runs[0] ?? null;

  function handleSelectRun(queueRunId: string) {
    setSelectedQueueRunId(queueRunId);
  }

  // h/l step through the "Needs Review" list. No-op at a list boundary.
  function goToAdjacentRun(direction: 'prev' | 'next') {
    const list = needsReview.runs;
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
      const remainingNeedsReview = needsReview.runs.filter((r) => r.queue_run_id !== queueRunId);
      needsReview.removeRun(queueRunId);
      completed.prependRun({ ...selectedRun, last_reviewed_time: new Date().toISOString() });
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
        <QueueBar selectedQueueId={queueId} onSelect={setQueueId} />
        <div className="flex flex-1 items-center justify-center">
          <span className="text-sm text-tertiary">Select an annotation queue to start reviewing.</span>
        </div>
      </div>
    );
  }

  if (!queue) {
    return (
      <div className="flex h-screen flex-col bg-surface-level-1">
        <QueueBar selectedQueueId={queueId} onSelect={setQueueId} />
        <LinearProgress />
      </div>
    );
  }

  const contentLoading = (needsReview.loading || completed.loading) && !selectedRun;

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-surface-level-1">
      <QueueBar selectedQueueId={queueId} onSelect={setQueueId} />
      <div className="flex min-h-0 flex-1 overflow-hidden rounded-lg border border-secondary m-space-4 mt-space-3">
        {/* Left: 280px run list */}
        <div className="flex h-full w-[280px] min-w-[280px] max-w-[280px] flex-col overflow-hidden">
          <RunList
            needsReview={needsReview}
            needsOthersReview={needsOthersReview}
            completed={completed}
            selectedQueueRunId={selectedQueueRunId ?? selectedRun?.queue_run_id}
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
            totalNeedsReview={needsReview.runs.length}
          />
        </div>
      </div>
    </div>
  );
}
