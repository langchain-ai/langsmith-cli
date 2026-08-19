import { useEffect, useState } from 'react';
import { Banner } from '@/components/langsmith/design-system/components/Banner';
import { Select } from '@/components/langsmith/design-system/components/Select';
import { Spinner } from '@/components/langsmith/design-system/components/Spinner';
import { Text } from '@/components/langsmith/design-system/components/Text';
import { ProjectBar } from './components/ProjectBar';
import { OverviewPanel } from './components/OverviewPanel';
import { RunsTable } from './components/RunsTable';
import { Section } from './components/primitives';
import { fetchCodingShare, fetchProjectStats, fetchRecentRuns } from './api';
import { cn } from './lib/utils';
import type { CodingShare, ProjectStats, Run, StatsScope } from './types';

const EMPTY_STATS: ProjectStats = { turns: null, threads: null, failedScopes: [] };

const WINDOW_OPTIONS = [
  { value: '1', label: 'Last 24 hours' },
  { value: '7', label: 'Last 7 days' },
  { value: '30', label: 'Last 30 days' },
  { value: '90', label: 'Last 90 days' },
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
      <div className="flex-1 p-space-6">{renderBody()}</div>
    </div>
  );

  function renderBody() {
    if (!projectId) {
      return (
        <GuideState step={1} heading="Pick a tracing project to get started" subtext="Every stat and the recent-runs table below are scoped to it." />
      );
    }
    if (statsLoading && !hasAnyStats) {
      return <GuideState spinner heading="Loading stats…" />;
    }
    if (crashed) {
      return <GuideState tone="error" heading="Couldn't load this project's stats" subtext="Check the console and your access to this project." />;
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
      return <GuideState heading="No data in this window" subtext="Try a wider time window above, or a different project." />;
    }

    return (
      <div className={cn('mx-auto flex max-w-6xl flex-col gap-space-5', statsLoading && 'motion-safe:transition-opacity motion-safe:duration-normal opacity-50')}>
        <div className="flex flex-wrap items-center justify-between gap-space-2">
          <Text variant="h3" as="h2">
            Overview
          </Text>
          {windowSelect()}
        </div>

        {hasFailures && (
          <Banner intent="warning" title="Some numbers are missing">
            {`Couldn't load ${stats.failedScopes.map((k) => SCOPE_LABEL[k]).join(' or ')} after retries — those numbers are showing as "—" below. Reselect the project or change the window to retry.`}
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
            <div className="flex items-center justify-center gap-space-2 py-space-6">
              <Spinner size="xs" className="text-icon-tertiary" />
              <Text variant="md" color="tertiary">
                Loading recent runs…
              </Text>
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
      <div className="flex items-center gap-space-2">
        <Text variant="sm" color="tertiary">
          Window
        </Text>
        <Select
          size="sm"
          options={WINDOW_OPTIONS}
          value={String(windowDays)}
          onChange={(value) => value && setWindowDays(Number(value))}
          triggerClassName="w-[150px]"
        />
      </div>
    );
  }
}

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
    <div className="flex h-full flex-col items-center justify-center gap-space-3 py-24 text-center">
      {spinner ? (
        <Spinner size="md" className="text-icon-tertiary" />
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
      <div className="flex flex-col gap-space-1">
        <Text variant="h3" color={tone === 'error' ? 'error' : 'primary'}>
          {heading}
        </Text>
        {subtext && (
          <Text variant="md" color="tertiary" className="max-w-md">
            {subtext}
          </Text>
        )}
      </div>
    </div>
  );
}
