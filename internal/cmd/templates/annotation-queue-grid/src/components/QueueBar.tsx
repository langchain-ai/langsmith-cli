import { Text } from '@/components/langsmith/design-system/components/Text';
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
    <div className="flex items-center gap-space-2 bg-surface-level-1 px-space-4 py-space-2">
      <Text as="label" variant="md" weight="medium" color="secondary" htmlFor="ls-queue-select" className="shrink-0">
        Annotation queue
      </Text>
      <SearchableSelect<AnnotationQueue>
        id="ls-queue-select"
        className="min-w-0 max-w-[420px] flex-1"
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
