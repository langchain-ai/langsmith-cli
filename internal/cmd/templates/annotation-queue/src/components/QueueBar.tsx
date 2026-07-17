import { useEffect, useState } from 'react';
import { fetchQueues } from '../api';
import type { AnnotationQueue } from '../types';

interface Props {
  selectedQueueId: string;
  onSelect: (queueId: string) => void;
}

// Apps get no host context, so the app picks its own queue from this bar's
// list of the workspace's queues.
export function QueueBar({ selectedQueueId, onSelect }: Props) {
  const [queues, setQueues] = useState<AnnotationQueue[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setLoading(true);
    fetchQueues()
      .then((qs) => setQueues(qs ?? []))
      .catch((e) => {
        console.error('Failed to load annotation queues', e);
        setFailed(true);
      })
      .finally(() => setLoading(false));
  }, []);

  const placeholder = loading
    ? 'Loading queues…'
    : failed
      ? 'Failed to load queues'
      : queues.length === 0
        ? 'No annotation queues in this workspace'
        : 'Select a queue…';

  return (
    <div className="flex items-center gap-2 border-b border-secondary bg-surface-level-1 px-4 py-2">
      <label htmlFor="ls-queue-select" className="shrink-0 text-sm font-medium text-secondary">
        Annotation queue
      </label>
      <select
        id="ls-queue-select"
        value={selectedQueueId}
        disabled={loading || failed}
        onChange={(e) => onSelect(e.target.value)}
        className="min-w-0 max-w-[420px] flex-1 rounded-md border border-secondary bg-primary px-3 py-1.5 text-sm text-primary focus:border-brand focus:outline-none disabled:opacity-60"
      >
        <option value="">{placeholder}</option>
        {queues.map((q) => (
          <option key={q.id} value={q.id}>
            {q.name}
          </option>
        ))}
      </select>
    </div>
  );
}
