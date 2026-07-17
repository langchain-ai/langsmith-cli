import { useEffect, useMemo, useState } from 'react';
import { Pickers } from './components/Pickers';
import { SummaryPanel } from './components/SummaryPanel';
import { ExampleTable } from './components/ExampleTable';
import { fetchComparison, fetchDatasets, fetchExperiments, fetchFeedbackConfigs } from './api';
import { costOf, latencyMs, scoreFor } from './lib/delta';
import type { Aggregate, Dataset, ExampleWithRuns, Experiment } from './types';

const EXAMPLE_LIMIT = 25;

function mean(values: number[]): number | null {
  return values.length ? values.reduce((a, b) => a + b, 0) / values.length : null;
}

export function App(_props: { data: unknown; metadata?: RenderMetadata }) {
  const [datasets, setDatasets] = useState<Dataset[]>([]);
  const [datasetsLoading, setDatasetsLoading] = useState(true);
  const [datasetId, setDatasetId] = useState('');

  const [experiments, setExperiments] = useState<Experiment[]>([]);
  const [experimentsLoading, setExperimentsLoading] = useState(false);
  const [baselineId, setBaselineId] = useState('');
  const [comparisonIds, setComparisonIds] = useState<string[]>([]);

  const [examples, setExamples] = useState<ExampleWithRuns[]>([]);
  const [examplesLoading, setExamplesLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const [lowerIsBetter, setLowerIsBetter] = useState<Record<string, boolean>>({});

  useEffect(() => {
    fetchDatasets()
      .then((ds) => setDatasets(ds ?? []))
      .catch((e) => console.error('Failed to load datasets', e))
      .finally(() => setDatasetsLoading(false));
  }, []);

  // Reset downstream selections and load this dataset's experiments.
  useEffect(() => {
    setExperiments([]);
    setBaselineId('');
    setComparisonIds([]);
    setExamples([]);
    if (!datasetId) return;
    setExperimentsLoading(true);
    fetchExperiments(datasetId)
      .then((xs) => setExperiments(xs ?? []))
      .catch((e) => console.error('Failed to load experiments', e))
      .finally(() => setExperimentsLoading(false));
  }, [datasetId]);

  const selectedIds = useMemo(
    () => (baselineId ? [baselineId, ...comparisonIds] : []),
    [baselineId, comparisonIds]
  );

  // Fetch the per-example comparison whenever the selection changes.
  useEffect(() => {
    setExamples([]);
    setFailed(false);
    if (!datasetId || !baselineId) return;
    setExamplesLoading(true);
    fetchComparison(datasetId, selectedIds, EXAMPLE_LIMIT)
      .then((rows) => setExamples(rows ?? []))
      .catch((e) => {
        console.error('Failed to load comparison', e);
        setFailed(true);
      })
      .finally(() => setExamplesLoading(false));
  }, [datasetId, baselineId, selectedIds.join(',')]);

  // Feedback keys present across all runs, and their score direction.
  const feedbackKeys = useMemo(() => {
    const keys = new Set<string>();
    for (const ex of examples) {
      for (const run of ex.runs) {
        for (const k of Object.keys(run.feedback_stats ?? {})) keys.add(k);
      }
    }
    return [...keys].sort();
  }, [examples]);

  useEffect(() => {
    if (feedbackKeys.length === 0) {
      setLowerIsBetter({});
      return;
    }
    fetchFeedbackConfigs(feedbackKeys)
      .then((configs) => {
        const map: Record<string, boolean> = {};
        for (const c of configs) map[c.feedback_key] = c.is_lower_score_better === true;
        setLowerIsBetter(map);
      })
      .catch((e) => console.error('Failed to load feedback configs', e));
  }, [feedbackKeys.join(',')]);

  const orderedExperiments = useMemo(
    () => selectedIds.map((id) => experiments.find((x) => x.id === id)).filter(Boolean) as Experiment[],
    [selectedIds, experiments]
  );

  // Per-experiment aggregates derived over the fetched examples.
  const aggregates = useMemo(() => {
    const out: Record<string, Aggregate> = {};
    for (const id of selectedIds) {
      const runs = examples.map((ex) => ex.runs.find((r) => r.session_id === id));
      const latencies = runs.map(latencyMs).filter((v): v is number => v != null);
      const costs = runs.map(costOf).filter((v): v is number => v != null);
      const tokens = runs
        .map((r) => (typeof r?.total_tokens === 'number' ? r.total_tokens : null))
        .filter((v): v is number => v != null);
      const avgScores: Record<string, number> = {};
      for (const key of feedbackKeys) {
        const scores = runs.map((r) => scoreFor(r, key)).filter((v): v is number => v != null);
        const m = mean(scores);
        if (m != null) avgScores[key] = m;
      }
      out[id] = {
        experimentId: id,
        runCount: runs.filter(Boolean).length,
        avgLatencyMs: mean(latencies),
        totalCost: costs.length ? costs.reduce((a, b) => a + b, 0) : null,
        avgTokens: mean(tokens),
        avgScores,
      };
    }
    return out;
  }, [examples, selectedIds, feedbackKeys]);

  return (
    <div className="flex min-h-screen flex-col bg-surface-level-1">
      <Pickers
        datasets={datasets}
        datasetsLoading={datasetsLoading}
        datasetId={datasetId}
        onDataset={setDatasetId}
        experiments={experiments}
        experimentsLoading={experimentsLoading}
        baselineId={baselineId}
        onBaseline={setBaselineId}
        comparisonIds={comparisonIds}
        onToggleComparison={(id) =>
          setComparisonIds((prev) =>
            prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
          )
        }
      />
      <div className="flex-1 p-6">{renderBody()}</div>
    </div>
  );

  function renderBody() {
    if (!datasetId) return centered('Select a dataset to begin.');
    if (!baselineId) return centered('Select a baseline experiment to compare against.');
    if (examplesLoading) return centered('Loading comparison…');
    if (failed) return centered('Failed to load comparison — check the console and your access.');
    if (examples.length === 0) return centered('No examples found for this selection.');

    const capped = examples.length >= EXAMPLE_LIMIT;
    return (
      <div className="mx-auto flex max-w-6xl flex-col gap-8">
        <section className="flex flex-col gap-3">
          <h2 className="text-base font-semibold text-primary">Summary</h2>
          <SummaryPanel
            experiments={orderedExperiments}
            aggregates={aggregates}
            feedbackKeys={feedbackKeys}
            lowerIsBetter={lowerIsBetter}
          />
          <p className="text-xs text-tertiary">
            Comparison columns are colored vs the baseline: lower latency/cost/tokens is better;
            feedback honors is_lower_score_better.
          </p>
        </section>

        <section className="flex flex-col gap-3">
          <h2 className="text-base font-semibold text-primary">Per-example</h2>
          <ExampleTable
            examples={examples}
            experiments={orderedExperiments}
            primaryKey={feedbackKeys[0]}
            lowerIsBetter={lowerIsBetter[feedbackKeys[0]] ?? false}
          />
          {capped && (
            <p className="text-xs text-tertiary">Showing first {EXAMPLE_LIMIT} examples.</p>
          )}
        </section>
      </div>
    );
  }
}

function centered(message: string) {
  return (
    <div className="flex h-full items-center justify-center">
      <span className="text-sm text-tertiary">{message}</span>
    </div>
  );
}
