import { useEffect, useMemo, useState } from 'react';
import { Pickers } from './components/Pickers';
import { SummaryPanel } from './components/SummaryPanel';
import { ExampleTable } from './components/ExampleTable';
import { Scoreboard } from './components/Scoreboard';
import { Scorecard } from './components/Scorecard';
import { DeltaHistogram } from './components/DeltaHistogram';
import { ScatterPlot } from './components/ScatterPlot';
import { Section } from './components/primitives';
import { Spinner } from './components/Spinner';
import { fetchComparison, fetchExperiments, fetchFeedbackConfigs } from './api';
import { costOf, latencyMs, scoreFor } from './lib/delta';
import { buildMetrics, comparisonColor, letterFor } from './lib/metrics';
import { cn } from './lib/utils';
import type { Aggregate, ExampleWithRuns, ExperimentView, Experiment } from './types';

const EXAMPLE_LIMIT = 25;

function mean(values: number[]): number | null {
  return values.length ? values.reduce((a, b) => a + b, 0) / values.length : null;
}

export function App(_props: { data: unknown; metadata?: RenderMetadata }) {
  const [datasetId, setDatasetId] = useState('');

  // The full list (not just what's paginated into the dataset/baseline
  // pickers) — needed to look experiments up by id for the Compare
  // checkboxes and the ordered/colored experiment views below.
  const [experiments, setExperiments] = useState<Experiment[]>([]);
  const [experimentsLoading, setExperimentsLoading] = useState(false);
  const [baselineId, setBaselineId] = useState('');
  const [comparisonIds, setComparisonIds] = useState<string[]>([]);

  const [examples, setExamples] = useState<ExampleWithRuns[]>([]);
  const [examplesLoading, setExamplesLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const [lowerIsBetter, setLowerIsBetter] = useState<Record<string, boolean>>({});
  const [metricId, setMetricId] = useState('');

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

  // Fetch the per-example comparison whenever the selection changes. The
  // previous render is kept on screen (dimmed via examplesLoading) instead of
  // being cleared up front, so re-picking a comparison doesn't flash back to
  // a bare loading message.
  useEffect(() => {
    setFailed(false);
    if (!datasetId || !baselineId) {
      setExamples([]);
      return;
    }
    setExamplesLoading(true);
    fetchComparison(datasetId, selectedIds, EXAMPLE_LIMIT)
      .then((rows) => setExamples(rows ?? []))
      .catch((e) => {
        console.error('Failed to load comparison', e);
        setFailed(true);
        setExamples([]);
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

  // Ordered experiments with a display letter and series color (baseline first).
  const expViews = useMemo<ExperimentView[]>(
    () =>
      orderedExperiments.map((x, i) => ({
        id: x.id,
        name: x.name,
        letter: letterFor(i),
        isBaseline: i === 0,
        color: i === 0 ? 'var(--text-tertiary)' : comparisonColor(i - 1),
      })),
    [orderedExperiments]
  );

  const metrics = useMemo(() => buildMetrics(feedbackKeys, lowerIsBetter), [feedbackKeys, lowerIsBetter]);

  // Keep the shared metric selection valid as feedback keys load.
  useEffect(() => {
    setMetricId((prev) => (metrics.some((m) => m.id === prev) ? prev : (metrics[0]?.id ?? '')));
  }, [metrics]);

  const selectedMetric = useMemo(
    () => metrics.find((m) => m.id === metricId) ?? metrics[0],
    [metrics, metricId]
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
    if (!datasetId) {
      return (
        <GuideState step={1} heading="Pick a dataset to get started" subtext="Every experiment you compare has to run against this dataset." />
      );
    }
    if (experimentsLoading && experiments.length === 0) {
      return <GuideState spinner heading="Loading experiments…" />;
    }
    if (experiments.length === 0) {
      return (
        <GuideState
          step={2}
          heading="This dataset doesn't have any experiments yet"
          subtext="Run an evaluation against it, then come back here to compare results."
        />
      );
    }
    if (!baselineId) {
      return (
        <GuideState step={2} heading="Choose a baseline experiment" subtext="Every other experiment gets compared against this one." />
      );
    }
    if (comparisonIds.length === 0) {
      return (
        <GuideState
          step={3}
          heading="Pick one or more experiments to compare"
          subtext="Check any experiment above to see how it stacks up against the baseline."
        />
      );
    }
    if (examples.length === 0 && examplesLoading) {
      return <GuideState spinner heading="Loading comparison…" />;
    }
    if (failed) {
      return <GuideState tone="error" heading="Couldn't load this comparison" subtext="Check the console and your access to this dataset." />;
    }
    if (examples.length === 0) {
      const comparedNames = expViews.slice(1).map((x) => x.name).join(', ') || 'the selected comparison';
      return (
        <GuideState
          heading="No shared examples for this selection"
          subtext={`${expViews[0]?.name ?? 'The baseline'} and ${comparedNames} don't have any traced examples in common here. Try a different baseline or comparison above.`}
        />
      );
    }

    const capped = examples.length >= EXAMPLE_LIMIT;

    return (
      <div
        className={cn(
          'mx-auto flex max-w-6xl flex-col gap-8 motion-safe:transition-opacity motion-safe:duration-normal',
          examplesLoading && 'pointer-events-none opacity-50'
        )}
      >
        <Section
          title="Summary"
          note="Comparison bars are colored vs the baseline: lower latency/cost/tokens is better; feedback honors is_lower_score_better."
        >
          <SummaryPanel experiments={expViews} aggregates={aggregates} metrics={metrics} />
        </Section>

        {selectedMetric && (
          <>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="text-base font-semibold text-primary">Focus metric</h2>
              {metricSelect()}
            </div>

            <Section title="Scoreboard" note={`${selectedMetric.label} for each experiment, vs the baseline.`}>
              <Scoreboard experiments={expViews} aggregates={aggregates} metric={selectedMetric} />
            </Section>

            <Section title="Regression scorecard" note="Per comparison, the share of examples that beat, tied, or lost to the baseline — across every metric.">
              <Scorecard examples={examples} experiments={expViews} metrics={metrics} />
            </Section>

            <Section title="Delta distribution" note={`How ${selectedMetric.label} shifted per example — green bars are wins, red bars are regressions.`}>
              <DeltaHistogram examples={examples} experiments={expViews} metric={selectedMetric} />
            </Section>

            <Section title="Baseline vs comparison" note="Each point is one example; the dashed line is parity.">
              <ScatterPlot examples={examples} experiments={expViews} metric={selectedMetric} />
            </Section>
          </>
        )}

        <Section title="Per-example" note={capped ? `Showing first ${EXAMPLE_LIMIT} examples.` : undefined}>
          <ExampleTable examples={examples} experiments={expViews} metric={selectedMetric} />
        </Section>
      </div>
    );
  }

  function metricSelect() {
    return (
      <label className="flex items-center gap-2 text-xs text-tertiary">
        Metric
        <select
          value={metricId}
          onChange={(e) => setMetricId(e.target.value)}
          className="rounded-md border border-secondary bg-primary px-2 py-1 text-xs text-primary focus:border-brand focus:outline-none"
        >
          {metrics.map((m) => (
            <option key={m.id} value={m.id}>
              {m.label}
            </option>
          ))}
        </select>
      </label>
    );
  }
}

// The empty/blocked state for every step of the guide above — echoes the
// same numbered-badge language as the Pickers stepper, so "you're on step 2"
// reads the same whether the reason is up top or down here.
function GuideState({
  step,
  heading,
  subtext,
  spinner,
  tone,
}: {
  step?: number;
  heading: string;
  subtext?: string;
  spinner?: boolean;
  tone?: 'error';
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 py-24 text-center">
      {spinner ? (
        <Spinner size="md" />
      ) : step != null ? (
        <span
          className={cn(
            'flex size-10 items-center justify-center rounded-full border-2 text-base font-semibold',
            tone === 'error' ? 'border-error text-error-primary' : 'border-brand text-brand-primary'
          )}
        >
          {step}
        </span>
      ) : null}
      <div className="flex flex-col gap-1">
        <span className={cn('text-base font-semibold', tone === 'error' ? 'text-error-primary' : 'text-primary')}>{heading}</span>
        {subtext && <span className="max-w-md text-sm text-tertiary">{subtext}</span>}
      </div>
    </div>
  );
}
