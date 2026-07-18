import { useEffect, useState } from 'react';
import { ChevronDownIcon } from '@langchain/untitled-ui-icons';
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
    <div className="flex items-center gap-2 bg-surface-level-1 px-4 py-2">
      <label htmlFor="ls-queue-select" className="shrink-0 text-sm font-medium text-secondary">
        Annotation queue
      </label>
      <div className="relative min-w-0 max-w-[420px] flex-1">
        <select
          id="ls-queue-select"
          value={selectedQueueId}
          disabled={loading || failed}
          onChange={(e) => onSelect(e.target.value)}
          className="w-full appearance-none rounded-md border border-secondary bg-primary py-1.5 pl-3 pr-8 text-sm text-primary focus:border-brand focus:outline-none disabled:opacity-60"
        >
          <option value="">{placeholder}</option>
          {queues.map((q) => (
            <option key={q.id} value={q.id}>
              {q.name}
            </option>
          ))}
        </select>
        <ChevronDownIcon className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-tertiary" />
      </div>
    </div>
  );
}
