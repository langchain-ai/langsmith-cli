import { useEffect, useState } from 'react';
import { fetchProjects } from '../api';
import { ALL_PROJECTS, type Project } from '../types';

function ChevronDownIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className={className}>
      <path d="m6 9 6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

interface Props {
  selectedProjectId: string;
  onSelect: (projectId: string) => void;
}

// The runs query is project-scoped, so the app picks its own project here.
export function ProjectBar({ selectedProjectId, onSelect }: Props) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setLoading(true);
    fetchProjects()
      .then((ps) => setProjects(ps ?? []))
      .catch((e) => {
        console.error('Failed to load projects', e);
        setFailed(true);
      })
      .finally(() => setLoading(false));
  }, []);

  const placeholder = loading
    ? 'Loading projects…'
    : failed
      ? 'Failed to load projects'
      : projects.length === 0
        ? 'No tracing projects in this workspace'
        : 'Select a project…';

  return (
    <div className="flex items-center gap-2 bg-surface-level-1 px-4 py-2">
      <label htmlFor="ls-project-select" className="shrink-0 text-sm font-medium text-secondary">
        Project
      </label>
      <div className="relative min-w-0 max-w-[420px] flex-1">
        <select
          id="ls-project-select"
          value={selectedProjectId}
          disabled={loading || failed}
          onChange={(e) => onSelect(e.target.value)}
          className="w-full appearance-none rounded-md border border-secondary bg-primary py-1.5 pl-3 pr-8 text-sm text-primary focus:border-brand focus:outline-none disabled:opacity-60"
        >
          <option value="">{placeholder}</option>
          {projects.length > 0 && <option value={ALL_PROJECTS}>All projects (coding-agent scan)</option>}
          {projects.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
        <ChevronDownIcon className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-tertiary" />
      </div>
    </div>
  );
}
