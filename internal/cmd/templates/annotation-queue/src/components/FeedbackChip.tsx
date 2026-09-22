import { Badge } from '@langchain/macaw-components/Badge';
import { formatFeedbackValue } from '../lib/feedbackColors';
import { Spinner } from '@langchain/macaw-components/Spinner';

interface FeedbackChipProps {
  feedbackKey: string;
  score?: number | null;
  value?: string | null;
  isLoading?: boolean;
}

export function FeedbackChip({ feedbackKey, score, value, isLoading }: FeedbackChipProps) {
  const displayValue = formatFeedbackValue(score, value);

  return (
    <span className="inline-flex min-w-0 items-center gap-space-2">
      <Badge color="secondary" className="max-w-[240px] truncate">
        {displayValue == null ? feedbackKey : `${feedbackKey}: ${displayValue}`}
      </Badge>
      {isLoading && <Spinner size="sm" />}
    </span>
  );
}
