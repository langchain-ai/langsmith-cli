import { fetchQueues } from '../api';
import type { AnnotationQueue } from '../types';
import { SearchableSelect } from './SearchableSelect';

interface Props {
  selectedQueueId: string;
  onSelect: (queueId: string) => void;
}

// Apps get no host context, so the app picks its own queue from this bar's
// list of the workspace's queues — paginated (25/page) and searchable by
// name server-side via fetchQueues' name_contains.
export function QueueBar({ selectedQueueId, onSelect }: Props) {
  return (
    <div className="flex items-center gap-2 bg-surface-level-1 px-4 py-2">
      <label htmlFor="ls-queue-select" className="shrink-0 text-sm font-medium text-secondary">
        Annotation queue
      </label>
      <SearchableSelect<AnnotationQueue>
        id="ls-queue-select"
        value={selectedQueueId}
        onSelect={(queue) => onSelect(queue.id)}
        fetchPage={fetchQueues}
        placeholder="Select a queue…"
        searchPlaceholder="Search queues by name…"
        emptyLabel="No annotation queues in this workspace"
      />
    </div>
  );
}
