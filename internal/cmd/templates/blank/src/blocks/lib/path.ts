export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** Reads a dot path such as "items.0.id"; undefined when any segment is missing. */
export function getPath(value: unknown, path: string): unknown {
  let current: unknown = value;
  for (const segment of path.split('.')) {
    if (Array.isArray(current)) current = current[Number(segment)];
    else if (isRecord(current)) current = current[segment];
    else return undefined;
  }
  return current;
}

/** JSON with sorted object keys, so equal requests produce equal keys. */
export function stableKey(value: unknown): string {
  return JSON.stringify(value, (_key, v: unknown) => {
    if (!isRecord(v)) return v;
    const sorted: Record<string, unknown> = {};
    for (const k of Object.keys(v).sort()) sorted[k] = v[k];
    return sorted;
  });
}
