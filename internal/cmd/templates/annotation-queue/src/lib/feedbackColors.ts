import { getColorByString } from '@/components/langsmith/design-system/utils/get-color-by-string';

/** Deterministic per-key color. The design system's own hash — the same one
 * LangSmith uses for feedback-key swatches — so a key is the same color here
 * as it is in the product. */
export function getColorForFeedbackKey(key: string): string {
  return getColorByString(key);
}

export const scoreFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 0,
  maximumFractionDigits: 4,
});

export function formatFeedbackValue(
  score: number | null | undefined,
  value: string | null | undefined
): string | undefined {
  if (value != null) return value;
  if (score != null) return scoreFormatter.format(score);
  return undefined;
}
