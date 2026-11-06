// Test stub of `managed-deepagents/runtime`: the collectors and requirement
// helpers introspection calls, shaped like the published package's.
export const CONNECTOR_BRAND = Symbol.for("mda.connector");
export const CHANNEL_BRAND = Symbol.for("mda.channel");

function stemName(source, kind) {
  const base = source.replace(/^.*\//, "").replace(/\.(ts|tsx|mts|cts|js|mjs|py)$/, "");
  if (!base || base === kind) throw new Error(`invalid ${kind} path "${source}"`);
  return base;
}

function collect(modules, key, brand, kind) {
  const collected = [];
  const names = new Set();
  for (const { source, module } of modules) {
    const candidate = module[key];
    if (candidate === undefined || candidate === null || candidate[brand] !== true) continue;
    const name = stemName(source, kind);
    if (names.has(name)) throw new Error(`duplicate ${kind} name '${name}'`);
    names.add(name);
    Object.defineProperty(candidate, "name", { value: name, enumerable: false, configurable: true });
    collected.push(candidate);
  }
  return collected;
}

function requirementsFrom(items) {
  const out = {};
  for (const item of items) {
    if (typeof item.requirements !== "function" || !item.name) continue;
    out[item.name] = item.requirements(item.name);
  }
  return out;
}

export function collectConnectors(modules) {
  return collect(modules, "connector", CONNECTOR_BRAND, "connector");
}

export function requirementsFromConnectors(connectors) {
  return requirementsFrom(connectors);
}

export function collectChannels(modules) {
  return collect(modules, "channel", CHANNEL_BRAND, "channel");
}

export function requirementsFromChannels(channels) {
  return requirementsFrom(channels);
}
