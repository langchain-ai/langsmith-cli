import { useEffect, useState } from 'react';

interface Project {
  id: string;
  name: string;
  run_count?: number | null;
  last_run_start_time?: string | null;
}

function fmtRelative(iso?: string | null): string {
  if (!iso) return '—';
  const mins = Math.round((Date.now() - new Date(iso).getTime()) / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  if (mins < 1440) return `${Math.round(mins / 60)}h ago`;
  return `${Math.round(mins / 1440)}d ago`;
}

// A tiny, genuinely working starting point: list the workspace's most
// recently active tracing projects. Delete this and build whatever you
// want — see AGENTS.md for the full API surface.
export function App(_props: { data: unknown }) {
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    window.langsmith
      .call('GET /api/v1/sessions', { params: { limit: '10', include_stats: 'true' } })
      .then((res) => setProjects(res as Project[]))
      .catch((e) => setError(e instanceof Error ? e.message : String(e)));
  }, []);

  return (
    <div className="min-h-screen bg-surface-level-1 p-6 text-primary">
      <h1 className="text-lg font-semibold">Recent projects</h1>

      {error && <p className="mt-3 text-sm text-error-primary">{error}</p>}

      {!error && !projects && <p className="mt-3 text-sm text-tertiary">Loading…</p>}

      {projects && projects.length === 0 && (
        <p className="mt-3 text-sm text-tertiary">No tracing projects in this workspace yet.</p>
      )}

      {projects && projects.length > 0 && (
        <ul className="mt-3 divide-y divide-secondary rounded-lg border border-secondary">
          {projects.map((p) => (
            <li key={p.id} className="flex items-center justify-between px-4 py-2 text-sm">
              <span className="font-medium">{p.name}</span>
              <span className="text-tertiary">
                {p.run_count ?? 0} runs · {fmtRelative(p.last_run_start_time)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
