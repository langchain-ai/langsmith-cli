export function toNumber(value: unknown): number | null {
  if (typeof value === 'number') return Number.isFinite(value) ? value : null;
  if (typeof value === 'boolean') return value ? 1 : 0;
  if (typeof value === 'string' && value.trim() !== '') {
    const n = Number(value);
    return Number.isFinite(n) ? n : null;
  }
  return null;
}

export function truncateText(value: unknown, length = 80): string {
  const text = typeof value === 'string' ? value : value == null ? '' : stringify(value);
  const max = Math.max(1, length);
  return text.length > max ? `${text.slice(0, max - 1).trimEnd()}…` : text;
}

export function stringify(value: unknown, indent?: number): string {
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value, null, indent) ?? String(value);
  } catch {
    return String(value);
  }
}
