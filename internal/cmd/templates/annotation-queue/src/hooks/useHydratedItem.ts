import { useEffect, useState } from 'react';
import { fetchRun, fetchThreadMessages } from '../api';
import type { QueueItem } from '../types';

/**
 * Merges a list membership stub with a type-specific payload hydrate.
 * RUN → GET /v2/runs/{id}; THREAD → POST /v1/trajectory (format: messages).
 */
export function useHydratedItem(listItem: QueueItem | null): {
  item: QueueItem | null;
  loading: boolean;
} {
  const [hydrated, setHydrated] = useState<QueueItem | null>(listItem);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setHydrated(listItem);
    if (!listItem) {
      setLoading(false);
      return;
    }

    const projectId = listItem.project_id;
    let cancelled = false;

    if (listItem.item_type === 'RUN' && listItem.run_id && projectId) {
      setLoading(true);
      fetchRun(listItem.run_id, projectId, listItem.start_time)
        .then((run) => {
          if (cancelled) return;
          setHydrated({
            ...listItem,
            name: run.name ?? listItem.name,
            inputs: run.inputs ?? null,
            outputs: run.outputs ?? null,
            error: run.error ?? null,
            trace_id: run.trace_id,
            start_time: run.start_time ?? listItem.start_time,
            project_id: run.project_id ?? listItem.project_id,
          });
        })
        .catch((e) => console.error('Failed to hydrate run', e))
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    } else if (listItem.item_type === 'THREAD' && listItem.thread_id && projectId) {
      setLoading(true);
      fetchThreadMessages(listItem.thread_id, projectId)
        .then((messages) => {
          if (cancelled) return;
          setHydrated({
            ...listItem,
            name: listItem.name ?? listItem.thread_id,
            messages,
          });
        })
        .catch((e) => console.error('Failed to hydrate thread', e))
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    } else {
      setLoading(false);
    }

    return () => {
      cancelled = true;
    };
  }, [
    listItem?.id,
    listItem?.item_type,
    listItem?.run_id,
    listItem?.thread_id,
    listItem?.project_id,
    listItem?.start_time,
  ]);

  return { item: hydrated, loading };
}
