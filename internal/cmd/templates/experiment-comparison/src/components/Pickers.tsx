import type { Dataset, Experiment } from '../types';

interface Props {
  datasets: Dataset[];
  datasetsLoading: boolean;
  datasetId: string;
  onDataset: (id: string) => void;

  experiments: Experiment[];
  experimentsLoading: boolean;
  baselineId: string;
  onBaseline: (id: string) => void;

  comparisonIds: string[];
  onToggleComparison: (id: string) => void;
}

const selectClass =
  'min-w-0 max-w-[360px] flex-1 rounded-md border border-secondary bg-primary px-3 py-1.5 text-sm text-primary focus:border-brand focus:outline-none disabled:opacity-60';

// Dataset → baseline → one or more comparison experiments. Contextual apps are
// gone, so the app picks everything itself.
export function Pickers({
  datasets,
  datasetsLoading,
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

  return (
    <div className="flex flex-col gap-3 border-b border-secondary bg-surface-level-1 px-4 py-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <label htmlFor="ec-dataset" className="text-sm font-medium text-secondary">
          Dataset
        </label>
        <select
          id="ec-dataset"
          value={datasetId}
          disabled={datasetsLoading}
          onChange={(e) => onDataset(e.target.value)}
          className={selectClass}
        >
          <option value="">{datasetsLoading ? 'Loading datasets…' : 'Select a dataset…'}</option>
          {datasets.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </select>

        <label htmlFor="ec-baseline" className="text-sm font-medium text-secondary">
          Baseline
        </label>
        <select
          id="ec-baseline"
          value={baselineId}
          disabled={!datasetId || experimentsLoading}
          onChange={(e) => onBaseline(e.target.value)}
          className={selectClass}
        >
          <option value="">
            {experimentsLoading ? 'Loading experiments…' : 'Select a baseline…'}
          </option>
          {experiments.map((x) => (
            <option key={x.id} value={x.id}>
              {x.name}
            </option>
          ))}
        </select>
      </div>

      {baselineId && (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
          <span className="text-sm font-medium text-secondary">Compare</span>
          {others.length === 0 ? (
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
