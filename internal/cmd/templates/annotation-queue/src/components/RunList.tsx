import { useEffect, useRef, useState } from 'react';
import {
  CheckCircleIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  ClockIcon,
  LayersTwo01Icon,
  Lock01Icon,
  MessageChatCircleIcon,
  ZapIcon,
} from '@langchain/untitled-ui-icons';
import type { QueueItem } from '../types';
import { itemLabel } from '../types';
import type { ItemSection as ItemSectionData } from '../hooks/useItemSection';
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

interface ItemListRowProps {
  item: QueueItem;
  isSelected: boolean;
  onClick: () => void;
  numReviewersPerItem?: number | null;
}

function ItemListRow({ item, isSelected, onClick, numReviewersPerItem }: ItemListRowProps) {
  const preview =
    item.item_type === 'THREAD'
      ? item.thread_id ?? 'thread'
      : getCollapsedPreview(item.inputs ?? null);
  const completedCount = item.completed_by?.length ?? 0;
  const reservedCount = item.reserved_by?.length ?? 0;
  const isFullyReserved =
    !!numReviewersPerItem && reservedCount >= numReviewersPerItem;
  const showProgress = !!numReviewersPerItem && numReviewersPerItem > 1 && completedCount > 0;
  const TypeIcon = item.item_type === 'THREAD' ? MessageChatCircleIcon : ZapIcon;

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
          'inline-flex shrink-0 items-center gap-1 rounded px-1 py-0.5 text-[10px] font-semibold uppercase',
          item.item_type === 'THREAD'
            ? 'bg-brand-muted text-brand-primary'
            : 'bg-secondary text-tertiary'
        )}
        title={item.item_type}
      >
        <TypeIcon className="h-3 w-3" />
        {item.item_type === 'THREAD' ? 'Thread' : 'Run'}
      </span>
      <span
        className={cn(
          'shrink-0 truncate text-sm',
          isFullyReserved ? 'text-quaternary' : 'text-primary'
        )}
      >
        {itemLabel(item)}
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
  label: string;
  icon: React.ElementType;
  section: ItemSectionData;
  selectedItemId: string | undefined;
  defaultOpen: boolean;
  isLast?: boolean;
  numReviewersPerItem?: number | null;
  onSelectItem: (itemId: string) => void;
}

function ItemSection({
  label,
  icon: Icon,
  section,
  selectedItemId,
  defaultOpen,
  isLast,
  numReviewersPerItem,
  onSelectItem,
}: SectionProps) {
  const { items, total, loading, loadingMore, hasMore, loadMore } = section;
  const [open, setOpen] = useState(defaultOpen);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open || !hasMore) return;
    const root = scrollRef.current;
    const target = sentinelRef.current;
    if (!root || !target) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) loadMore();
      },
      { root, rootMargin: '150px' }
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, [open, hasMore, loadMore]);

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
          <span className="text-xs text-tertiary">{Math.max(total, items.length)}</span>
        </div>
      </button>

      {open && (
        <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto">
          {loading ? (
            <RunListSkeletons />
          ) : items.length === 0 ? (
            <div className="flex items-center justify-center px-3 py-6">
              <span className="text-xs text-tertiary">No items</span>
            </div>
          ) : (
            <>
              {items.map((item) => (
                <ItemListRow
                  key={item.id}
                  item={item}
                  isSelected={item.id === selectedItemId}
                  onClick={() => onSelectItem(item.id)}
                  numReviewersPerItem={numReviewersPerItem}
                />
              ))}
              {hasMore && (
                <div ref={sentinelRef} className="flex items-center justify-center py-3">
                  {loadingMore && <Spinner size="sm" />}
                </div>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}

interface Props {
  needsReview: ItemSectionData;
  needsOthersReview: ItemSectionData;
  completed: ItemSectionData;
  selectedItemId: string | undefined;
  numReviewersPerItem?: number | null;
  onSelectItem: (itemId: string) => void;
}

export function RunList({
  needsReview,
  needsOthersReview,
  completed,
  selectedItemId,
  numReviewersPerItem,
  onSelectItem,
}: Props) {
  const showNeedsOthersReview = (numReviewersPerItem ?? 0) > 1;

  return (
    <div className="flex h-full flex-col divide-y divide-secondary overflow-hidden border-r border-secondary">
      <ItemSection
        label="Needs Review"
        icon={LayersTwo01Icon}
        section={needsReview}
        selectedItemId={selectedItemId}
        defaultOpen={true}
        numReviewersPerItem={numReviewersPerItem}
        onSelectItem={onSelectItem}
      />
      {showNeedsOthersReview && (
        <ItemSection
          label="Needs Others' Review"
          icon={ClockIcon}
          section={needsOthersReview}
          selectedItemId={selectedItemId}
          defaultOpen={false}
          numReviewersPerItem={numReviewersPerItem}
          onSelectItem={onSelectItem}
        />
      )}
      <ItemSection
        label="Completed"
        icon={CheckCircleIcon}
        section={completed}
        selectedItemId={selectedItemId}
        defaultOpen={false}
        isLast={true}
        onSelectItem={onSelectItem}
      />
    </div>
  );
}
