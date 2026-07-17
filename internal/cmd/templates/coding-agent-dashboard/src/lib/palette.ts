// Categorical palette for entity identity (integrations, models). Ordered for
// max separation and validated in light + dark; the 7th+ entity shares Other.
// Status meaning (errors, success) uses semantic tokens, never these hues.
export const CATEGORICAL = [
  'var(--brand-400)',
  'var(--red-400)',
  'var(--purple-400)',
  'var(--green-500)',
  'var(--orange-500)',
  'var(--acid-400)',
];

export const OTHER = 'var(--gray-400)';

export function colorAt(index: number): string {
  return index >= 0 && index < CATEGORICAL.length ? CATEGORICAL[index] : OTHER;
}
