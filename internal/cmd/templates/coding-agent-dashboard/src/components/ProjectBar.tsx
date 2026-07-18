import { fetchProjects } from '../api';
import type { Project } from '../types';
import { SearchableSelect } from './SearchableSelect';

interface Props {
  selectedProjectId: string;
  onSelect: (projectId: string) => void;
}

// The runs query is project-scoped, so the app picks its own project here —
// paginated (25/page) and searchable by name server-side.
export function ProjectBar({ selectedProjectId, onSelect }: Props) {
  return (
    <div className="flex items-center gap-2 bg-surface-level-1 px-4 py-2">
      <label htmlFor="ls-project-select" className="shrink-0 text-sm font-medium text-secondary">
        Project
      </label>
      <SearchableSelect<Project>
        id="ls-project-select"
        value={selectedProjectId}
        onSelect={(project) => onSelect(project.id)}
        fetchPage={fetchProjects}
        placeholder="Select a project…"
        searchPlaceholder="Search projects by name…"
        emptyLabel="No tracing projects in this workspace"
      />
    </div>
  );
}
