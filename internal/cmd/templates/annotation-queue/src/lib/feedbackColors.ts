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
