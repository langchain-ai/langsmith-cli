import { fetchProjects } from '../api';
import { ALL_PROJECTS, type Project } from '../types';
import { SearchableSelect } from './SearchableSelect';

const ALL_PROJECTS_ITEM: Project = { id: ALL_PROJECTS, name: 'All projects (coding-agent scan)' };

// The synthetic "All projects" entry only makes sense on the first,
// unsearched page — searching for a project by name means the user wants a
// specific one. It occupies one slot of that page, so real projects are
// fetched one short there and every later page's offset is shifted back by
// one to compensate (see the offset math below).
async function fetchProjectsPage(search: string, offset: number, limit: number): Promise<Project[]> {
  if (search) return fetchProjects(search, offset, limit);
  if (offset === 0) {
    const real = await fetchProjects('', 0, limit - 1);
    return [ALL_PROJECTS_ITEM, ...real];
  }
  return fetchProjects('', offset - 1, limit);
}

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
        fetchPage={fetchProjectsPage}
        placeholder="Select a project…"
        searchPlaceholder="Search projects by name…"
        emptyLabel="No tracing projects in this workspace"
      />
    </div>
  );
}
