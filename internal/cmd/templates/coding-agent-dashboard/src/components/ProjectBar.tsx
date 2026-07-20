import { fetchProjects } from '../api';
import type { Project } from '../types';
import { SearchableSelect } from './SearchableSelect';

interface Props {
  selectedProjectId: string;
  onSelect: (projectId: string) => void;
}

// One step, same guided-flow language as the experiment-comparison
// template's stepper: a numbered badge (done once picked), a title, and the
// control right below it — everything always visible and editable.
export function ProjectBar({ selectedProjectId, onSelect }: Props) {
  const done = Boolean(selectedProjectId);

  return (
    <div className="flex gap-4 bg-surface-level-1 px-4 py-4">
      <span
        className={`flex size-7 shrink-0 items-center justify-center rounded-full border-2 text-sm font-semibold ${
          done ? 'border-brand bg-brand text-brand-on-fill' : 'border-brand text-brand-primary bg-surface-level-1'
        }`}
      >
        {done ? '✓' : 1}
      </span>
      <div className="flex flex-1 flex-col gap-1.5">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
          <span className="text-sm font-semibold text-primary">Select a tracing project</span>
          <span className="text-xs text-tertiary">Every stat below is scoped to it.</span>
        </div>
        <SearchableSelect<Project>
          id="ls-project-select"
          value={selectedProjectId}
          onSelect={(project) => onSelect(project.id)}
          fetchPage={fetchProjects}
          placeholder="Select a project…"
          searchPlaceholder="Search projects by name…"
          emptyLabel="No tracing projects in this workspace"
          className="max-w-[360px]"
        />
      </div>
    </div>
  );
}
