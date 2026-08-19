import { interpolateViridis } from 'd3-scale-chromatic';

// hash a random string and obtain a color from
export function getColorByString(str: string) {
  const limitedKey = str
    .trim()
    .toUpperCase()
    .normalize('NFD')
    // limit to first 2 chars
    .slice(0, 2)
    // @ is a placeholder for non-letters
    .replaceAll(/[^A-Z]/g, '@')
    // pad to 2 chars
    .padEnd(2, '@');
  // minimum char code is 64 (@), max is 90 (Z)
  // so we subtract 64 to get a range of 0-26
  // then we give more weight to the first letter
  // and sum the two values to get a single integer
  const asciiSum = limitedKey
    .split('')
    .map((c) => c.charCodeAt(0) - 64)
    .map((n, i) => n / (1 + i))
    .reduce((a, b) => a + b, 0);
  // the max value is 26 + 13 = 39, so we divide by 39 to get a range of 0-1
  const rebased = asciiSum / (26 + 13);
  // then we use d3-scale-chromatic to get a color from the viridis scale
  return interpolateViridis(rebased);
}
