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
import { Badge } from '@/components/langsmith/design-system/components/Badge';
import { Skeleton } from '@/components/langsmith/design-system/components/Skeleton';
import { Spinner } from '@/components/langsmith/design-system/components/Spinner';
import { Text } from '@/components/langsmith/design-system/components/Text';
import { Tooltip } from '@/components/langsmith/design-system/components/Tooltip';
import type { QueueItem } from '../types';
import { itemLabel } from '../types';
import type { ItemSection as ItemSectionData } from '../hooks/useItemSection';
import { getCollapsedPreview } from '../lib/messages';
import { cn } from '../lib/utils';

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
          className="flex items-center gap-space-2 border-b border-default px-space-3 py-2.5"
        >
          <Skeleton className={cn('h-3.5 shrink-0', nameW)} />
          <Skeleton className={cn('h-3 min-w-0 opacity-60', previewW)} />
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
  const isThread = item.item_type === 'THREAD';
  const TypeIcon = isThread ? MessageChatCircleIcon : ZapIcon;

  return (
    <button
      type="button"
      className={cn(
        'flex w-full items-center gap-space-2 border-b border-default px-space-3 py-2.5 text-left transition-colors duration-fast',
        isSelected ? 'bg-selected' : 'hover:bg-surface-level-2'
      )}
      onClick={onClick}
    >
      <Badge
        size="xxs"
        rounded="xs"
        color={isThread ? 'primary' : 'secondary'}
        leftDecorator={TypeIcon}
        className="shrink-0 uppercase"
      >
        {isThread ? 'Thread' : 'Run'}
      </Badge>
      <Text
        as="span"
        variant="md"
        color={isFullyReserved ? 'quaternary' : 'primary'}
        className="shrink-0 truncate"
      >
        {itemLabel(item)}
      </Text>
      {preview && (
        <Text
          as="span"
          variant="sm"
          color={isFullyReserved ? 'quaternary' : 'tertiary'}
          className="min-w-0 flex-1 truncate"
        >
          {preview}
        </Text>
      )}
      <div className="ml-auto flex shrink-0 items-center gap-1.5">
        {showProgress && (
          <Badge size="xxs" rounded="xs" leftDecorator={CheckCircleIcon}>
            {`${completedCount}/${numReviewersPerItem}`}
          </Badge>
        )}
        {isFullyReserved && (
          <Tooltip title="Already picked up by the maximum number of reviewers">
            <Badge size="xxs" rounded="xs" aria-label="Fully reserved">
              <Lock01Icon className="size-3" />
            </Badge>
          </Tooltip>
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
          'flex shrink-0 items-center gap-space-2 bg-surface-level-2 px-space-3 py-space-2 transition-colors duration-fast hover:bg-surface-level-3',
          open && 'border-b border-default',
          !open && isLast && 'border-b border-default'
        )}
        onClick={() => setOpen((v) => !v)}
      >
        {open ? (
          <ChevronDownIcon className="size-3.5 text-icon-tertiary" />
        ) : (
          <ChevronRightIcon className="size-3.5 text-icon-tertiary" />
        )}
        <Icon className="size-3.5 text-icon-tertiary" />
        <Text as="span" variant="md" weight="medium" color="secondary">
          {label}
        </Text>
        <div className="ml-auto flex items-center gap-1.5">
          {loading && <Spinner size="xs" className="text-icon-tertiary" />}
          <Text as="span" variant="sm" color="tertiary">
            {String(Math.max(total, items.length))}
          </Text>
        </div>
      </button>

      {open && (
        <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto">
          {loading ? (
            <RunListSkeletons />
          ) : items.length === 0 ? (
            <div className="flex items-center justify-center px-space-3 py-space-5">
              <Text variant="sm" color="tertiary">
                No items
              </Text>
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
                <div ref={sentinelRef} className="flex items-center justify-center py-space-3">
                  {loadingMore && <Spinner size="xs" className="text-icon-tertiary" />}
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
    <div className="flex h-full flex-col divide-y divide-default overflow-hidden border-r border-default">
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
