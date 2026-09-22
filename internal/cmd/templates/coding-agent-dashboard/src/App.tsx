import { GuideState } from './components/GuideState';
import { Select } from '@langchain/macaw-components/Select';
import { Banner } from '@langchain/macaw-components/Banner';
import { useEffect, useState } from 'react';
import { ProjectBar } from './components/ProjectBar';
import { OverviewPanel } from './components/OverviewPanel';
import { RunsTable } from './components/RunsTable';
import { Section } from './components/primitives';
import { Spinner } from '@langchain/macaw-components/Spinner';
import { fetchCodingShare, fetchProjectStats, fetchRecentRuns } from './api';
import { cn } from '@langchain/macaw-components/utils/cn';
import type { CodingShare, ProjectStats, Run, StatsScope } from './types';

const EMPTY_STATS: ProjectStats = { turns: null, threads: null, failedScopes: [] };

const WINDOW_OPTIONS: { days: number; label: string }[] = [
  { days: 1, label: 'Last 24 hours' },
  { days: 7, label: 'Last 7 days' },
  { days: 30, label: 'Last 30 days' },
  { days: 90, label: 'Last 90 days' },
];

const SCOPE_LABEL: Record<StatsScope, string> = {
  turns: 'turn stats',
  threads: 'thread count',
};

export function App(_props: { data: unknown; metadata?: RenderMetadata }) {
  const [projectId, setProjectId] = useState('');
  const [windowDays, setWindowDays] = useState(7);

  const [stats, setStats] = useState<ProjectStats>(EMPTY_STATS);
  const [statsLoading, setStatsLoading] = useState(false);
  const [crashed, setCrashed] = useState(false);

  const [runs, setRuns] = useState<Run[]>([]);
  const [runsLoading, setRunsLoading] = useState(false);
  const [runsFailed, setRunsFailed] = useState(false);

  const [codingShare, setCodingShare] = useState<CodingShare | null>(null);

  useEffect(() => {
    setStats(EMPTY_STATS);
    setRuns([]);
    setCodingShare(null);
    setCrashed(false);
    setRunsFailed(false);
    if (!projectId) return;

    setStatsLoading(true);
    fetchProjectStats(projectId, windowDays)
      .then(setStats)
      .catch((e) => {
        console.error('Failed to load project stats', e);
        setCrashed(true);
      })
      .finally(() => setStatsLoading(false));

    setRunsLoading(true);
    fetchRecentRuns(projectId, windowDays)
      .then(setRuns)
      .catch((e) => {
        console.error('Failed to load recent runs', e);
        setRunsFailed(true);
      })
      .finally(() => setRunsLoading(false));

    // The coding-share tile has its own query (unfiltered recent runs). A
    // failure here just leaves the tile at "—" — no separate error surface.
    fetchCodingShare(projectId, windowDays)
      .then(setCodingShare)
      .catch((e) => console.error('Failed to load coding share', e));
  }, [projectId, windowDays]);

  const hasAnyStats = stats.turns != null || stats.threads != null;
  const hasFailures = stats.failedScopes.length > 0;

  return (
    <div className="flex min-h-screen flex-col bg-surface-level-1">
      <ProjectBar selectedProjectId={projectId} onSelect={setProjectId} />
      <div className="flex-1 p-6">{renderBody()}</div>
    </div>
  );

  function renderBody() {
    if (!projectId) {
      return (
        <GuideState
          step={1}
          heading="Pick a tracing project to get started"
          subtext="Every stat and the recent-runs table below are scoped to it."
        />
      );
    }
    if (statsLoading && !hasAnyStats) {
      return <GuideState spinner heading="Loading stats…" />;
    }
    if (crashed) {
      return (
        <GuideState
          tone="error"
          heading="Couldn't load this project's stats"
          subtext="Check the console and your access to this project."
        />
      );
    }
    if (!hasAnyStats && hasFailures) {
      return (
        <GuideState
          tone="error"
          heading="Still rate-limited after retries"
          subtext={`Couldn't load ${stats.failedScopes.map((k) => SCOPE_LABEL[k]).join(' or ')} — try reselecting the project in a moment.`}
        />
      );
    }
    if (!hasAnyStats) {
      return (
        <GuideState
          heading="No data in this window"
          subtext="Try a wider time window above, or a different project."
        />
      );
    }

    return (
      <div
        className={cn(
          'mx-auto flex max-w-6xl flex-col gap-6',
          statsLoading && 'motion-safe:transition-opacity motion-safe:duration-normal opacity-50'
        )}
      >
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-base font-semibold text-primary">Overview</h2>
          {windowSelect()}
        </div>

        {hasFailures && (
          <Banner intent="warning" title="Some stats are unavailable">
            Couldn't load {stats.failedScopes.map((k) => SCOPE_LABEL[k]).join(' or ')} after retries
            — those numbers are showing as "—" below. Reselect the project or change the window to
            retry.
          </Banner>
        )}

        <OverviewPanel stats={stats} codingShare={codingShare} />

        <Section
          title="Recent runs"
          note={
            runsFailed
              ? "Couldn't load recent runs — check the console and your access."
              : `Most recent ${runs.length} turn${runs.length === 1 ? '' : 's'} in this window — a sample for browsing, not the source of the stats above.`
          }
        >
          {runsLoading && runs.length === 0 ? (
            <div className="flex items-center justify-center gap-2 py-8">
              <Spinner size="sm" />
              <span className="text-sm text-tertiary">Loading recent runs…</span>
            </div>
          ) : (
            <RunsTable runs={runs} />
          )}
        </Section>
      </div>
    );
  }

  function windowSelect() {
    return (
      <label className="flex items-center gap-2 text-xs text-tertiary">
        Window
        <Select
          triggerClassName="w-auto min-w-32"
          aria-label="Window"
          value={String(windowDays)}
          onChange={(value) => {
            if (value) setWindowDays(Number(value));
          }}
          options={WINDOW_OPTIONS.map((option) => ({
            value: String(option.days),
            label: option.label,
          }))}
        />
      </label>
    );
  }
}
