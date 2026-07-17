import { useEffect, useMemo, useState } from 'react';
import { ProjectBar } from './components/ProjectBar';
import { PieChart, type Slice } from './components/PieChart';
import { IntegrationBreakdown } from './components/IntegrationBreakdown';
import { fetchCodingRuns } from './api';
import { colorAt } from './lib/palette';
import type { IntegrationStat, Run } from './types';

// A non-empty string metadata value from extra.metadata, else undefined.
function rawMeta(run: Run, key: string): string | undefined {
  const v = run.extra?.metadata?.[key];
  return typeof v === 'string' && v ? v : undefined;
}

function meta(run: Run, key: string): string {
  return rawMeta(run, key) ?? 'unknown';
}

// Root runs carry the model under "model"; "ls_model_name" is on child LLM runs.
function modelName(run: Run): string {
  return rawMeta(run, 'ls_model_name') ?? rawMeta(run, 'model') ?? 'unknown';
}

export function App(_props: { data: unknown; metadata?: RenderMetadata }) {
  const [projectId, setProjectId] = useState('');
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setRuns([]);
    setFailed(false);
    if (!projectId) return;
    setLoading(true);
    fetchCodingRuns(projectId)
      .then(setRuns)
      .catch((e) => {
        console.error('Failed to load coding runs', e);
        setFailed(true);
      })
      .finally(() => setLoading(false));
  }, [projectId]);

  // Aggregate by ls_integration, sorted by count so colors stay stable.
  const stats = useMemo<IntegrationStat[]>(() => {
    const map = new Map<string, { count: number; errors: number; models: Map<string, number> }>();
    for (const r of runs) {
      const integ = meta(r, 'ls_integration');
      const model = modelName(r);
      const entry = map.get(integ) ?? { count: 0, errors: 0, models: new Map() };
      entry.count++;
      if (r.error) entry.errors++;
      entry.models.set(model, (entry.models.get(model) ?? 0) + 1);
      map.set(integ, entry);
    }
    return [...map.entries()]
      .map(([integration, e]) => ({
        integration,
        count: e.count,
        errors: e.errors,
        models: [...e.models.entries()]
          .map(([model, count]) => ({ model, count }))
          .sort((a, b) => b.count - a.count),
      }))
      .sort((a, b) => b.count - a.count);
  }, [runs]);

  const colorByIntegration = useMemo(() => {
    const m = new Map<string, string>();
    stats.forEach((s, i) => m.set(s.integration, colorAt(i)));
    return m;
  }, [stats]);
  const colorOf = (integration: string) => colorByIntegration.get(integration) ?? colorAt(0);

  const slices: Slice[] = stats.map((s) => ({
    label: s.integration,
    value: s.count,
    color: colorOf(s.integration),
  }));

  return (
    <div className="flex min-h-screen flex-col bg-surface-level-1">
      <ProjectBar selectedProjectId={projectId} onSelect={setProjectId} />
      <div className="flex-1 p-6">{renderBody()}</div>
    </div>
  );

  function renderBody() {
    if (!projectId) {
      return centered('Select a project to see its coding-agent runs.');
    }
    if (loading) {
      return centered('Loading coding runs…');
    }
    if (failed) {
      return centered('Failed to load runs — check the console and your access.');
    }
    if (runs.length === 0) {
      return centered('No coding-agent runs found in this project (last 7 days).');
    }
    return (
      <div className="mx-auto flex max-w-4xl flex-col gap-8">
        <section className="flex flex-col gap-4 rounded-lg border border-secondary p-6">
          <h2 className="text-base font-semibold text-primary">
            Integration share · {runs.length} run{runs.length === 1 ? '' : 's'}
          </h2>
          <PieChart slices={slices} />
        </section>

        <section className="flex flex-col gap-4">
          <h2 className="text-base font-semibold text-primary">Models by integration</h2>
          <IntegrationBreakdown stats={stats} colorOf={colorOf} />
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
