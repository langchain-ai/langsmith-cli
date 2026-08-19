import { Text } from '@/components/langsmith/design-system/components/Text';
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
    <div className="flex gap-space-4 bg-surface-level-1 px-space-4 py-space-4">
      <span
        className={`flex size-7 shrink-0 items-center justify-center rounded-full border-2 text-sm font-semibold ${
          done ? 'border-brand bg-brand text-brand-on-fill' : 'border-brand bg-surface-level-1 text-brand-primary'
        }`}
      >
        {done ? '✓' : 1}
      </span>
      <div className="flex flex-1 flex-col gap-1.5">
        <div className="flex flex-wrap items-baseline gap-x-space-2 gap-y-0.5">
          <Text as="span" variant="md" weight="semibold">
            Select a tracing project
          </Text>
          <Text as="span" variant="sm" color="tertiary">
            Every stat below is scoped to it.
          </Text>
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
