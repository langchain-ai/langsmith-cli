// Categorical palette of index.css tokens that flip under html.dark, so
// slices stay legible in both themes. Cycles past its length.
export const CATEGORICAL = [
  'var(--brand-400)',
  'var(--green-500)',
  'var(--orange-500)',
  'var(--purple-400)',
  'var(--acid-400)',
  'var(--red-400)',
  'var(--brand-200)',
];

export function colorAt(index: number): string {
  return CATEGORICAL[index % CATEGORICAL.length];
}
