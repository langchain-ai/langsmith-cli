import { useEffect, useMemo, useState } from 'react';
import { ProjectBar } from './components/ProjectBar';
import { AllProjectsView } from './components/AllProjectsView';
import { OverviewPanel } from './components/OverviewPanel';
import { CompositionPanel } from './components/CompositionPanel';
import { EconomicsPanel } from './components/EconomicsPanel';
import { BehaviorPanel } from './components/BehaviorPanel';
import { ActivityPanel } from './components/ActivityPanel';
import { ContextPanel } from './components/ContextPanel';
import { fetchProjectRuns } from './api';
import { countBy } from './lib/aggregate';
import { integrationOf } from './lib/normalize';
import { colorAt, OTHER } from './lib/palette';
import { ALL_PROJECTS, type ProjectRuns } from './types';

const EMPTY: ProjectRuns = { roots: [], llm: [], tool: [], subagents: [] };

export function App(_props: { data: unknown; metadata?: RenderMetadata }) {
  const [projectId, setProjectId] = useState('');
  const [runs, setRuns] = useState<ProjectRuns>(EMPTY);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);

  const scan = projectId === ALL_PROJECTS;

  useEffect(() => {
    setRuns(EMPTY);
    setFailed(false);
    if (!projectId || scan) return;
    setLoading(true);
    fetchProjectRuns(projectId)
      .then(setRuns)
      .catch((e) => {
        console.error('Failed to load coding runs', e);
        setFailed(true);
      })
      .finally(() => setLoading(false));
  }, [projectId, scan]);

  // Stable integration→color map (by root frequency) shared across panels.
  const colorOf = useMemo(() => {
    const counts = countBy(runs.roots, integrationOf);
    const all = new Set<string>();
    for (const rs of [runs.roots, runs.llm, runs.tool, runs.subagents]) for (const r of rs) all.add(integrationOf(r));
    const ordered = [...all].sort((a, b) => (counts.get(b) ?? 0) - (counts.get(a) ?? 0) || a.localeCompare(b));
    const m = new Map<string, string>();
    ordered.forEach((integ, i) => m.set(integ, colorAt(i)));
    return (integ: string) => m.get(integ) ?? OTHER;
  }, [runs]);

  const empty = runs.roots.length === 0 && runs.llm.length === 0 && runs.tool.length === 0;

  return (
    <div className="flex min-h-screen flex-col bg-surface-level-1">
      <ProjectBar selectedProjectId={projectId} onSelect={setProjectId} />
      <div className="flex-1 p-6">{renderBody()}</div>
    </div>
  );

  function renderBody() {
    if (scan) return <AllProjectsView />;
    if (!projectId) return centered('Select a project to see its coding-agent runs, or scan all projects.');
    if (loading) return centered('Loading coding runs…');
    if (failed) return centered('Failed to load runs — check the console and your access.');
    if (empty) return centered('No coding-agent runs found in this project (last 7 days).');

    return (
      <div className="mx-auto flex max-w-6xl flex-col gap-4">
        <p className="text-xs text-tertiary">
          Last 7 days · each query returns the 100 most-recent matches (older runs are not counted).
        </p>
        <OverviewPanel roots={runs.roots} />
        <CompositionPanel roots={runs.roots} llm={runs.llm} colorOf={colorOf} />
        <EconomicsPanel roots={runs.roots} colorOf={colorOf} />
        <BehaviorPanel tool={runs.tool} subagents={runs.subagents} llm={runs.llm} />
        <ActivityPanel roots={runs.roots} colorOf={colorOf} />
        <ContextPanel roots={runs.roots} colorOf={colorOf} />
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
