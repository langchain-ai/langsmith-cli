import { getColorForFeedbackKey, formatFeedbackValue } from '../lib/feedbackColors';
import { Spinner } from './Spinner';

interface FeedbackChipProps {
  feedbackKey: string;
  score?: number | null;
  value?: string | null;
  isLoading?: boolean;
}

export function FeedbackChip({ feedbackKey, score, value, isLoading }: FeedbackChipProps) {
  const color = getColorForFeedbackKey(feedbackKey);
  const displayValue = formatFeedbackValue(score, value);

  return (
    <div className="flex h-6 flex-row items-center gap-1.5 rounded-sm border border-secondary bg-primary px-2">
      <div
        className="shrink-0 rounded-[2px]"
        style={{ backgroundColor: color, width: '6px', height: '6px' }}
      />
      <span className="max-w-[200px] truncate text-xs font-medium leading-normal text-primary">
        {feedbackKey}
      </span>
      {isLoading ? (
        <Spinner size="sm" />
      ) : displayValue != null ? (
        <span className="min-w-0 max-w-[100px] truncate text-xs text-secondary">
          {displayValue}
        </span>
      ) : null}
    </div>
  );
}
