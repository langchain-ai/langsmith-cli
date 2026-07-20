import { useCallback, type ReactNode } from 'react';
import { fetchDatasets, fetchExperiments } from '../api';
import type { Dataset, Experiment } from '../types';
import { SearchableSelect } from './SearchableSelect';
import { cn } from '../lib/utils';

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

type StepStatus = 'done' | 'active' | 'disabled';

// Dataset → baseline → one or more comparison experiments — a persistent,
// always-editable 3-step guide (not a wizard you advance through and lose
// access to). A later step disables with an inline reason until the step
// before it is satisfied; nothing here is ever hidden.
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

  const step2Status: StepStatus = !datasetId ? 'disabled' : baselineId ? 'done' : 'active';
  const step3Status: StepStatus = !baselineId ? 'disabled' : comparisonIds.length > 0 ? 'done' : 'active';

  return (
    <div className="flex flex-col bg-surface-level-1 px-4 py-4">
      <Step number={1} status={datasetId ? 'done' : 'active'} title="Choose a dataset" last={false}>
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
      </Step>

      <Step
        number={2}
        status={step2Status}
        title="Choose a baseline"
        subtitle="Every other experiment gets compared against this one."
        last={false}
      >
        {step2Status === 'disabled' ? (
          <Hint>Pick a dataset first.</Hint>
        ) : (
          <SearchableSelect<Experiment>
            id="ec-baseline"
            value={baselineId}
            onSelect={(experiment) => onBaseline(experiment.id)}
            fetchPage={fetchBaselinePage}
            placeholder="Select a baseline…"
            searchPlaceholder="Search experiments by name…"
            emptyLabel="No experiments in this dataset"
            className="max-w-[360px]"
          />
        )}
      </Step>

      <Step
        number={3}
        status={step3Status}
        title="Choose what to compare"
        subtitle="Check one or more experiments to see how they stack up against the baseline."
        last
      >
        {step3Status === 'disabled' ? (
          <Hint>Pick a baseline first.</Hint>
        ) : experimentsLoading ? (
          <Hint>Loading experiments…</Hint>
        ) : others.length === 0 ? (
          <Hint>No other experiments in this dataset.</Hint>
        ) : (
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
            {others.map((x) => (
              <label key={x.id} className="flex items-center gap-1.5 text-sm text-primary">
                <input
                  type="checkbox"
                  checked={comparisonIds.includes(x.id)}
                  onChange={() => onToggleComparison(x.id)}
                  className="accent-[var(--bg-brand)]"
                />
                <span className="truncate">{x.name}</span>
              </label>
            ))}
          </div>
        )}
      </Step>
    </div>
  );
}

function Hint({ children }: { children: ReactNode }) {
  return <span className="text-sm text-tertiary">{children}</span>;
}

const BADGE_CLASS: Record<StepStatus, string> = {
  done: 'border-brand bg-brand text-brand-on-fill',
  active: 'border-brand text-brand-primary bg-surface-level-1',
  disabled: 'border-subtle text-quaternary bg-surface-level-2',
};

const TITLE_CLASS: Record<StepStatus, string> = {
  done: 'text-primary',
  active: 'text-primary',
  disabled: 'text-quaternary',
};

function Step({
  number,
  status,
  title,
  subtitle,
  last,
  children,
}: {
  number: number;
  status: StepStatus;
  title: string;
  subtitle?: string;
  last: boolean;
  children: ReactNode;
}) {
  return (
    <div className="flex gap-4">
      <div className="flex flex-col items-center">
        <span
          className={cn(
            'flex size-7 shrink-0 items-center justify-center rounded-full border-2 text-sm font-semibold motion-safe:transition-colors motion-safe:duration-normal',
            BADGE_CLASS[status]
          )}
        >
          {status === 'done' ? '✓' : number}
        </span>
        {!last && <span className="my-1 w-px flex-1 bg-border-subtle" style={{ minHeight: 12 }} />}
      </div>
      <div className={cn('flex flex-1 flex-col gap-1.5', last ? 'pb-0' : 'pb-4')}>
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
          <span className={cn('text-sm font-semibold', TITLE_CLASS[status])}>{title}</span>
          {subtitle && <span className="text-xs text-tertiary">{subtitle}</span>}
        </div>
        {children}
      </div>
    </div>
  );
}
