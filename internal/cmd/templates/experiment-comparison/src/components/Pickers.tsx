import { useCallback } from 'react';
import { fetchDatasets, fetchExperiments } from '../api';
import type { Dataset, Experiment } from '../types';
import { SearchableSelect } from './SearchableSelect';

interface Props {
  datasetId: string;
  onDataset: (id: string) => void;

  experiments: Experiment[];
  experimentsLoading: boolean;
  baselineId: string;
  onBaseline: (id: string) => void;

  comparisonIds: string[];
  onToggleComparison: (id: string) => void;
}

// Dataset → baseline → one or more comparison experiments, picked entirely
// within the app. Dataset and baseline are each a paginated (25/page),
// searchable-by-name dropdown; the full experiments list (used for the
// Compare checkboxes and elsewhere) is still fetched in full by App.tsx.
export function Pickers({
  datasetId,
  onDataset,
  experiments,
  experimentsLoading,
  baselineId,
  onBaseline,
  comparisonIds,
  onToggleComparison,
}: Props) {
  const others = experiments.filter((e) => e.id !== baselineId);

  const fetchBaselinePage = useCallback(
    (search: string, offset: number, limit: number) =>
      datasetId ? fetchExperiments(datasetId, search, offset, limit) : Promise.resolve([]),
    [datasetId]
  );

  return (
    <div className="flex flex-col gap-3 bg-surface-level-1 px-4 py-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <label htmlFor="ec-dataset" className="text-sm font-medium text-secondary">
          Dataset
        </label>
        <SearchableSelect<Dataset>
          id="ec-dataset"
          value={datasetId}
          onSelect={(dataset) => onDataset(dataset.id)}
          fetchPage={fetchDatasets}
          placeholder="Select a dataset…"
          searchPlaceholder="Search datasets by name…"
          emptyLabel="No datasets in this workspace"
          className="max-w-[360px]"
        />

        <label htmlFor="ec-baseline" className="text-sm font-medium text-secondary">
          Baseline
        </label>
        <SearchableSelect<Experiment>
          id="ec-baseline"
          value={baselineId}
          onSelect={(experiment) => onBaseline(experiment.id)}
          fetchPage={fetchBaselinePage}
          disabled={!datasetId}
          placeholder="Select a baseline…"
          searchPlaceholder="Search experiments by name…"
          emptyLabel="No experiments in this dataset"
          className="max-w-[360px]"
        />
      </div>

      {baselineId && (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
          <span className="text-sm font-medium text-secondary">Compare</span>
          {experimentsLoading ? (
            <span className="text-sm text-tertiary">Loading experiments…</span>
          ) : others.length === 0 ? (
            <span className="text-sm text-tertiary">No other experiments in this dataset.</span>
          ) : (
            others.map((x) => (
              <label key={x.id} className="flex items-center gap-1.5 text-sm text-primary">
                <input
                  type="checkbox"
                  checked={comparisonIds.includes(x.id)}
                  onChange={() => onToggleComparison(x.id)}
                  className="accent-[var(--bg-brand)]"
                />
                <span className="truncate">{x.name}</span>
              </label>
            ))
          )}
        </div>
      )}
    </div>
  );
}
