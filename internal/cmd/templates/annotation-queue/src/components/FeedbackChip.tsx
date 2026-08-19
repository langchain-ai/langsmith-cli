import { Spinner } from '@/components/langsmith/design-system/components/Spinner';
import { Text } from '@/components/langsmith/design-system/components/Text';
import { getColorForFeedbackKey, formatFeedbackValue } from '../lib/feedbackColors';

interface FeedbackChipProps {
  feedbackKey: string;
  score?: number | null;
  value?: string | null;
  isLoading?: boolean;
}

// Composed rather than a plain <Badge>: the swatch is the key's identity color
// (the same hash LangSmith uses elsewhere), which is data, not a variant — so
// it can't come out of the Badge color scale.
export function FeedbackChip({ feedbackKey, score, value, isLoading }: FeedbackChipProps) {
  const color = getColorForFeedbackKey(feedbackKey);
  const displayValue = formatFeedbackValue(score, value);

  return (
    <div className="flex h-6 flex-row items-center gap-1.5 rounded-sm border border-default bg-surface-level-1 px-space-2">
      <span
        className="size-1.5 shrink-0 rounded-xs"
        style={{ backgroundColor: color }}
        aria-hidden
      />
      <Text as="span" variant="sm" weight="medium" className="max-w-[200px] truncate">
        {feedbackKey}
      </Text>
      {isLoading ? (
        <Spinner size="xs" className="text-icon-tertiary" />
      ) : displayValue != null ? (
        <Text as="span" variant="sm" color="secondary" className="min-w-0 max-w-[100px] truncate">
          {displayValue}
        </Text>
      ) : null}
    </div>
  );
}
