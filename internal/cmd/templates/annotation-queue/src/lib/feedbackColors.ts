// Viridis-scale color stops (perceptually-uniform), approximating d3-scale-chromatic's
// interpolateViridis without pulling in the d3 dependency.
const VIRIDIS_STOPS: Array<[number, number, number]> = [
  [0x44, 0x01, 0x54],
  [0x3b, 0x52, 0x8b],
  [0x21, 0x90, 0x8d],
  [0x5d, 0xc9, 0x63],
  [0xfd, 0xe7, 0x25],
];

function interpolateViridis(t: number): string {
  const clamped = Math.min(1, Math.max(0, t));
  const scaled = clamped * (VIRIDIS_STOPS.length - 1);
  const i = Math.min(VIRIDIS_STOPS.length - 2, Math.floor(scaled));
  const frac = scaled - i;
  const [r1, g1, b1] = VIRIDIS_STOPS[i];
  const [r2, g2, b2] = VIRIDIS_STOPS[i + 1];
  const r = Math.round(r1 + (r2 - r1) * frac);
  const g = Math.round(g1 + (g2 - g1) * frac);
  const b = Math.round(b1 + (b2 - b1) * frac);
  return `rgb(${r}, ${g}, ${b})`;
}

/** Deterministic per-key color, matching LangSmith's fallback feedback-key color hash. */
export function getColorForFeedbackKey(key: string): string {
  const limitedKey = key
    .trim()
    .toUpperCase()
    .normalize('NFD')
    .slice(0, 2)
    .replace(/[^A-Z]/g, '@')
    .padEnd(2, '@');
  const asciiSum = limitedKey
    .split('')
    .map((c) => c.charCodeAt(0) - 64)
    .map((n, i) => n / (1 + i))
    .reduce((a, b) => a + b, 0);
  const rebased = asciiSum / (26 + 13);
  return interpolateViridis(rebased);
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
