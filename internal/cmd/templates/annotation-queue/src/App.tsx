import { useEffect, useRef, useState } from 'react';
import { FeedbackPanel } from './components/FeedbackPanel';
import { LinearProgress } from './components/LinearProgress';
import { QueueBar } from './components/QueueBar';
import { RunList } from './components/RunList';
import { RunViewer } from './components/RunViewer';
import { ThreadViewer } from './components/ThreadViewer';
import { fetchQueue, markItemComplete } from './api';
import { useHydratedItem } from './hooks/useHydratedItem';
import { useItemSection } from './hooks/useItemSection';
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
  const [selectedItemId, setSelectedItemId] = useState<string | undefined>(undefined);
  const [completing, setCompleting] = useState(false);
  const [completeError, setCompleteError] = useState<string | null>(null);

  const isMultiReviewer = !!queue?.num_reviewers_per_item && queue.num_reviewers_per_item > 1;
  const needsReview = useItemSection(queueId || undefined, 'needs_my_review');
  const needsOthersReview = useItemSection(
    queueId || undefined,
    'needs_others_review',
    isMultiReviewer
  );
  const completed = useItemSection(queueId || undefined, 'completed');

  const completeRef = useRef<() => void>(() => {});
  const goToAdjacentItemRef = useRef<(direction: 'prev' | 'next') => void>(() => {});

  useEffect(() => {
    setQueue(null);
    setSelectedItemId(undefined);
    if (!queueId) return;
    fetchQueue(queueId)
      .then(setQueue)
      .catch((e) => console.error('Failed to load queue', e));
  }, [queueId]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      const tag = (document.activeElement as HTMLElement | null)?.tagName;
      const inInput =
        tag === 'INPUT' ||
        tag === 'TEXTAREA' ||
        (document.activeElement as HTMLElement | null)?.isContentEditable;

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
          goToAdjacentItemRef.current('prev');
        }
        if (e.key === 'l') {
          goToAdjacentItemRef.current('next');
        }
      }
    }
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  const allItems = [...needsReview.items, ...needsOthersReview.items, ...completed.items];
  const selectedListItem =
    allItems.find((i) => i.id === selectedItemId) ?? needsReview.items[0] ?? null;

  const { item: selectedItem, loading: hydrating } = useHydratedItem(selectedListItem);

  function handleSelectItem(itemId: string) {
    setSelectedItemId(itemId);
    setCompleteError(null);
  }

  function goToAdjacentItem(direction: 'prev' | 'next') {
    const list = needsReview.items;
    if (list.length === 0) return;
    const idx = list.findIndex((i) => i.id === selectedItemId);
    if (idx === -1) {
      handleSelectItem(list[0].id);
      return;
    }
    const nextIdx = direction === 'prev' ? idx - 1 : idx + 1;
    if (nextIdx < 0 || nextIdx >= list.length) return;
    handleSelectItem(list[nextIdx].id);
  }

  useEffect(() => {
    goToAdjacentItemRef.current = goToAdjacentItem;
  });

  function handleComplete() {
    if (!selectedItem || completing) return;
    const itemId = selectedItem.id;
    setCompleting(true);
    setCompleteError(null);
    markItemComplete(itemId)
      .then(() => {
        const remainingNeedsReview = needsReview.items.filter((i) => i.id !== itemId);
        needsReview.removeItem(itemId);
        completed.prependItem({
          ...selectedItem,
          last_reviewed_time: new Date().toISOString(),
        });
        setSelectedItemId(remainingNeedsReview[0]?.id);
      })
      .catch((e) => {
        console.error('Failed to mark complete', e);
        setCompleteError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => setCompleting(false));
  }

  useEffect(() => {
    completeRef.current = handleComplete;
  });

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

  if (!queue) {
    return (
      <div className="flex h-screen flex-col bg-surface-level-1">
        <QueueBar selectedQueueId={queueId} onSelect={setQueueId} />
        <LinearProgress />
      </div>
    );
  }

  const contentLoading =
    ((needsReview.loading || completed.loading) && !selectedListItem) || hydrating;

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-surface-level-1">
      <QueueBar selectedQueueId={queueId} onSelect={setQueueId} />
      <div className="m-4 mt-3 flex min-h-0 flex-1 overflow-hidden rounded-lg border border-secondary">
        <div className="flex h-full w-[280px] min-w-[280px] max-w-[280px] flex-col overflow-hidden">
          <RunList
            needsReview={needsReview}
            needsOthersReview={needsOthersReview}
            completed={completed}
            selectedItemId={selectedItemId ?? selectedItem?.id}
            numReviewersPerItem={queue.num_reviewers_per_item}
            onSelectItem={handleSelectItem}
          />
        </div>

        <div className="relative flex min-w-0 flex-1 flex-col overflow-auto">
          {selectedItem || contentLoading ? (
            selectedItem?.item_type === 'THREAD' ? (
              <ThreadViewer
                messages={selectedItem?.messages}
                threadId={selectedItem?.thread_id}
                loading={contentLoading}
              />
            ) : (
              <RunViewer
                inputs={selectedItem?.inputs ?? null}
                outputs={selectedItem?.outputs ?? null}
                error={selectedItem?.error}
                loading={contentLoading}
              />
            )
          ) : (
            <div className="flex flex-1 items-center justify-center">
              <span className="text-sm text-tertiary">Select an item to review</span>
            </div>
          )}
        </div>

        <div className="flex h-full w-[450px] min-w-[450px] max-w-[450px] flex-col border-l border-secondary">
          <FeedbackPanel
            queue={queue}
            itemType={selectedItem?.item_type}
            runId={selectedItem?.item_type === 'RUN' ? selectedItem.run_id : undefined}
            feedbackThreadId={
              selectedItem?.item_type === 'THREAD' ? selectedItem.thread_id : undefined
            }
            traceId={selectedItem?.trace_id}
            sessionId={selectedItem?.project_id}
            startTime={selectedItem?.start_time}
            itemId={selectedItem?.id}
            onComplete={handleComplete}
            completing={completing}
            completeError={completeError}
            totalNeedsReview={needsReview.items.length}
          />
        </div>
      </div>
    </div>
  );
}
