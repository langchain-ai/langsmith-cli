import { useState } from 'react';
import {
  CheckCircleIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  ClockIcon,
  LayersTwo01Icon,
  Lock01Icon,
} from '@langchain/untitled-ui-icons';
import type { AnnotationQueueRun } from '../types';
import { getCollapsedPreview } from '../lib/messages';
import { cn } from '../lib/utils';
import { Spinner } from './Spinner';

const SKELETON_WIDTH_PAIRS: [string, string][] = [
  ['w-[35%]', 'w-[45%]'],
  ['w-[45%]', 'w-[30%]'],
  ['w-[30%]', 'w-[50%]'],
  ['w-[40%]', 'w-[35%]'],
  ['w-[25%]', 'w-[40%]'],
  ['w-[38%]', 'w-[28%]'],
  ['w-[42%]', 'w-[32%]'],
  ['w-[28%]', 'w-[48%]'],
];

function RunListSkeletons() {
  return (
    <div className="flex flex-col">
      {SKELETON_WIDTH_PAIRS.map(([nameW, previewW], i) => (
        <div
          key={i}
          className="flex items-center gap-2 border-b border-secondary px-3 py-2.5"
        >
          <div className={cn('my-0 h-3.5 shrink-0 animate-pulse rounded bg-secondary', nameW)} />
          <div className={cn('my-0 h-3 min-w-0 animate-pulse rounded bg-secondary opacity-60', previewW)} />
        </div>
      ))}
    </div>
  );
}

interface RunListItemProps {
  run: AnnotationQueueRun;
  isSelected: boolean;
  onClick: () => void;
  numReviewersPerItem?: number | null;
}

function RunListItem({ run, isSelected, onClick, numReviewersPerItem }: RunListItemProps) {
  const preview = getCollapsedPreview(run.inputs);
  const completedCount = run.completed_by?.length ?? 0;
  const reservedCount = run.reserved_by?.length ?? 0;
  const isFullyReserved =
    !!numReviewersPerItem && reservedCount >= numReviewersPerItem;
  const showProgress = !!numReviewersPerItem && numReviewersPerItem > 1 && completedCount > 0;

  return (
    <button
      type="button"
      className={cn(
        'flex w-full items-center gap-2 border-b border-secondary px-3 py-2.5 text-left transition-colors',
        isSelected ? 'bg-tertiary' : 'hover:bg-secondary'
      )}
      onClick={onClick}
    >
      <span
        className={cn(
          'shrink-0 truncate text-sm',
          isFullyReserved ? 'text-quaternary' : 'text-primary'
        )}
      >
        {run.name ?? run.id.slice(0, 8)}
      </span>
      {preview && (
        <span
          className={cn(
            'min-w-0 flex-1 truncate text-xs',
            isFullyReserved ? 'text-quaternary' : 'text-tertiary'
          )}
        >
          {preview}
        </span>
      )}
      <div className="ml-auto flex shrink-0 items-center gap-1.5">
        {showProgress && (
          <span className="flex items-center gap-0.5 rounded bg-secondary px-1 py-0.5">
            <CheckCircleIcon className="h-3 w-3 text-tertiary" />
            <span className="text-xs leading-3 text-tertiary">
              {completedCount}/{numReviewersPerItem}
            </span>
          </span>
        )}
        {isFullyReserved && (
          <span className="flex items-center rounded bg-secondary px-1 py-0.5">
            <Lock01Icon className="h-3 w-3 text-tertiary" />
          </span>
        )}
      </div>
    </button>
  );
}

interface SectionProps {
  status: string;
  label: string;
  icon: React.ElementType;
  runs: AnnotationQueueRun[];
  selectedQueueRunId: string | undefined;
  loading: boolean;
  defaultOpen: boolean;
  isLast?: boolean;
  numReviewersPerItem?: number | null;
  onSelectRun: (queueRunId: string) => void;
}

function RunSection({
  label,
  icon: Icon,
  runs,
  selectedQueueRunId,
  loading,
  defaultOpen,
  isLast,
  numReviewersPerItem,
  onSelectRun,
}: SectionProps) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div className={cn('flex flex-col', open && 'min-h-0 flex-1')}>
      <button
        type="button"
        className={cn(
          'flex shrink-0 items-center gap-2 bg-secondary px-3 py-2 hover:bg-tertiary',
          open && 'border-b border-secondary',
          !open && isLast && 'border-b border-secondary'
        )}
        onClick={() => setOpen((v) => !v)}
      >
        {open ? (
          <ChevronDownIcon className="h-3.5 w-3.5 text-tertiary" />
        ) : (
          <ChevronRightIcon className="h-3.5 w-3.5 text-tertiary" />
        )}
        <Icon className="h-3.5 w-3.5 text-tertiary" />
        <span className="text-sm font-medium text-secondary">{label}</span>
        <div className="ml-auto flex items-center gap-1.5">
          {loading && <Spinner size="sm" />}
          <span className="text-xs text-tertiary">{runs.length}</span>
        </div>
      </button>

      {open && (
        <div className="min-h-0 flex-1 overflow-y-auto">
          {loading ? (
            <RunListSkeletons />
          ) : runs.length === 0 ? (
            <div className="flex items-center justify-center px-3 py-6">
              <span className="text-xs text-tertiary">No runs</span>
            </div>
          ) : (
            runs.map((run) => (
              <RunListItem
                key={run.queue_run_id}
                run={run}
                isSelected={run.queue_run_id === selectedQueueRunId}
                onClick={() => onSelectRun(run.queue_run_id)}
                numReviewersPerItem={numReviewersPerItem}
              />
            ))
          )}
        </div>
      )}
    </div>
  );
}

interface Props {
  needsReviewRuns: AnnotationQueueRun[];
  needsOthersReviewRuns: AnnotationQueueRun[];
  completedRuns: AnnotationQueueRun[];
  selectedQueueRunId: string | undefined;
  loading: boolean;
  numReviewersPerItem?: number | null;
  onSelectRun: (queueRunId: string) => void;
}

export function RunList({
  needsReviewRuns,
  needsOthersReviewRuns,
  completedRuns,
  selectedQueueRunId,
  loading,
  numReviewersPerItem,
  onSelectRun,
}: Props) {
  const showNeedsOthersReview = (numReviewersPerItem ?? 0) > 1;

  return (
    <div className="flex h-full flex-col divide-y divide-secondary overflow-hidden border-r border-secondary">
      <RunSection
        status="needs_my_review"
        label="Needs Review"
        icon={LayersTwo01Icon}
        runs={needsReviewRuns}
        selectedQueueRunId={selectedQueueRunId}
        loading={loading}
        defaultOpen={true}
        numReviewersPerItem={numReviewersPerItem}
        onSelectRun={onSelectRun}
      />
      {showNeedsOthersReview && (
        <RunSection
          status="needs_others_review"
          label="Needs Others' Review"
          icon={ClockIcon}
          runs={needsOthersReviewRuns}
          selectedQueueRunId={selectedQueueRunId}
          loading={loading}
          defaultOpen={false}
          numReviewersPerItem={numReviewersPerItem}
          onSelectRun={onSelectRun}
        />
      )}
      <RunSection
        status="completed"
        label="Completed"
        icon={CheckCircleIcon}
        runs={completedRuns}
        selectedQueueRunId={selectedQueueRunId}
        loading={loading}
        defaultOpen={false}
        isLast={true}
        onSelectRun={onSelectRun}
      />
    </div>
  );
}
