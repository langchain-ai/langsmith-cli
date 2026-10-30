// The TypeScript introspection helper of `langsmith deploy`.
//
// Reads one JSON request on stdin and writes one JSON document to the file the
// request names in `output`.
//
// - `{"mode": "project", ...}` imports the project's modules (with tsx, from the
//   project root, so they resolve the project's own dependencies) and reports
//   what deploy needs to know about the values they export.
// - `{"mode": "lex", "files": [...]}` lists the import statements of each
//   source, for the source rewrites compile makes.
// - `{"mode": "check"}` loads the toolchain, to verify a fresh install.
//
// The document goes to a file, not stdout: a project module may write to file
// descriptor 1 directly, which nothing in this process can keep out of stdout.
// What the project's modules print through `process.stdout` goes to stderr. A
// module that fails to import is reported in its section, never read as "not
// configured".
import { readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { pathToFileURL } from "node:url";
import {
  NodeVersionError,
  ToolchainError,
  checkNode,
  errorText,
  loadToolchain,
  registerLoaders,
} from "./toolchain.mjs";

const VERSION = 1;

process.stdout.write = process.stderr.write.bind(process.stderr);

/** Where a named factory carries its metadata (`defineAgent(name, factory)`). */
const AGENT_FACTORY = Symbol.for("managed-deepagents.agent-factory");
const CHANNEL_BRAND = Symbol.for("mda.channel");
const INGRESS_PROVIDERS = new Set(["slack", "github", "linear"]);
const UNSUPPORTED_AGENT_KEYS = [
  "backend",
  "store",
  "checkpointer",
  "instructions",
  "mcp",
  "memory",
  "sandbox",
  "skills",
  "systemPrompt",
];
const SANDBOX_KEYS = [
  "snapshotName",
  "snapshotId",
  "dockerImage",
  "registry",
  "fsCapacityBytes",
  "scope",
];

class NotJson extends Error {}

async function main() {
  let output;
  try {
    const request = JSON.parse(readFileSync(0, "utf8"));
    output = request.output;
    if (typeof output !== "string" || output === "") {
      throw new Error("the request names no `output` file");
    }
    finish(output, await respond(request));
  } catch (error) {
    if (output === undefined) {
      process.stderr.write(`introspection helper: ${errorText(error)}\n`);
      process.exit(1);
    }
    const kind = errorKind(error);
    finish(output, { error: { kind, message: kind === "helper" ? errorText(error) : error.message } });
  }
}

async function respond(request) {
  checkNode();
  switch (request.mode) {
    case "check":
      await loadToolchain();
      return {};
    case "lex":
      await loadToolchain();
      return await lexFiles(request.files);
    case "project":
      await loadToolchain();
      await registerLoaders();
      return await introspectProject(request);
  }
  throw new Error(`unknown helper mode ${JSON.stringify(request.mode)}`);
}

/** Write the document and exit, whatever timers the project's modules left. */
function finish(output, document) {
  writeFileSync(output, JSON.stringify({ version: VERSION, ...document }) + "\n");
  process.exit(0);
}

function errorKind(error) {
  if (error instanceof NodeVersionError) return "node";
  if (error instanceof ToolchainError) return "toolchain";
  return "helper";
}

// --- lex mode ---------------------------------------------------------------

async function lexFiles(files) {
  const { lexImports, byteOffsets } = await import("./lex.mjs");
  return {
    files: files.map(({ source }) => {
      let records;
      try {
        records = lexImports(source);
      } catch (error) {
        return { imports: [], error: String(error?.message ?? error) };
      }
      const offsets = byteOffsets(
        source,
        records.flatMap((record) => [record.start, record.end]),
      );
      return {
        imports: records.map((record, index) => ({
          statement: record.statement,
          specifier: record.specifier,
          type_only: record.typeOnly,
          start: offsets[2 * index],
          end: offsets[2 * index + 1],
          attributes: record.attributes,
          default_binding: record.defaultBinding,
        })),
        error: null,
      };
    }),
  };
}

// --- project mode -----------------------------------------------------------

async function introspectProject(request) {
  const root = request.root;
  const require = createRequire(path.join(root, "package.json"));
  const document = { sdk: await installedSdk(require, root) };

  const load = async (rel) => {
    if (rel == null) return null;
    try {
      return { rel, module: await import(pathToFileURL(path.join(root, rel)).href) };
    } catch (error) {
      return { rel, error: describeError(error, path.join(root, rel)) };
    }
  };
  const section = (loaded, read) => {
    if (loaded == null) return null;
    if (loaded.error) return failed(loaded.rel, loaded.error);
    try {
      return succeeded(loaded.rel, read(loaded.module));
    } catch (error) {
      return failed(loaded.rel, valueError(error));
    }
  };

  document.agent = section(await load(request.agent), readAgent);
  document.identity = section(await load(request.identity), (m) => readIdentity(m.identity));
  document.memory = section(await load(request.memory), (m) => readMemory(m.memory));
  document.sandbox = section(await load(request.sandbox), readSandbox);
  const mcp = await load(request.mcp);
  document.mcp = section(mcp, (m) => exportsOf(m, "mcp"));
  document.legacy_connectors = [];
  for (const rel of request.legacy_connectors) {
    document.legacy_connectors.push(section(await load(rel), (m) => exportsOf(m, "connector")));
  }
  const channelModules = [];
  document.channels = [];
  for (const rel of request.channels) {
    document.channels.push(
      section(await load(rel), (m) => {
        const exported = exportsOf(m, "channel");
        if (exported.has_export) channelModules.push({ source: rel, module: m });
        return { ...exported, ingress: exported.has_export ? channelIngress(m.channel) : null };
      }),
    );
  }
  document.schedules = [];
  for (const rel of request.schedules) {
    document.schedules.push(section(await load(rel), readSchedule));
  }

  if (document.mcp && (document.mcp.error || document.mcp.value.has_export)) {
    const connectors = mcp.error
      ? { file: mcp.rel, error: mcp.error }
      : await collected(require, root, mcp.rel, (runtime) =>
          runtime.collectConnectors([
            { source: mcp.rel, module: { connector: mcp.module.mcp } },
          ]),
        );
    document.connector_requirements = derive(connectors, (runtime, list) =>
      plainJson(runtime.requirementsFromConnectors(list)),
    );
    document.mcp_connections = derive(connectors, (_runtime, list) => mcpConnections(list));
  }
  const channelFailure = document.channels.find((entry) => entry.error);
  if (channelFailure) {
    document.channel_requirements = failed(channelFailure.file, channelFailure.error);
  } else if (channelModules.length > 0) {
    const channels = await collected(require, root, channelModules[0].source, (runtime) =>
      runtime.collectChannels(channelModules),
    );
    document.channel_requirements = derive(channels, (runtime, list) =>
      plainJson(runtime.requirementsFromChannels(list)),
    );
  }
  return document;
}

/**
 * The project's own `managed-deepagents`: its version, and whether it has named
 * factories (`defineAgent(name, factory)`).
 */
async function installedSdk(require, root) {
  let entry;
  let sdk;
  try {
    entry = require.resolve("managed-deepagents");
    sdk = await import(pathToFileURL(entry).href);
  } catch (error) {
    throw new Error(`cannot import managed-deepagents from ${root}: ${String(error?.message ?? error)}`);
  }
  return { version: packageVersion(entry), named_factories: sdk.AGENT_FACTORY === AGENT_FACTORY };
}

/** The version in the `managed-deepagents` package.json above `entry`. */
function packageVersion(entry) {
  for (let dir = path.dirname(entry); ; dir = path.dirname(dir)) {
    try {
      const manifest = JSON.parse(readFileSync(path.join(dir, "package.json"), "utf8"));
      if (manifest.name === "managed-deepagents") {
        return typeof manifest.version === "string" ? manifest.version : null;
      }
    } catch {
      // Not the package root yet.
    }
    if (path.dirname(dir) === dir) return null;
  }
}

/** Run `collect` against the project's own `managed-deepagents/runtime`. */
async function collected(require, root, file, collect) {
  let runtime;
  try {
    runtime = await import(pathToFileURL(require.resolve("managed-deepagents/runtime")).href);
  } catch (error) {
    return {
      file,
      error: {
        kind: "helper",
        message:
          `cannot import managed-deepagents/runtime from ${root}: ${String(error?.message ?? error)}. ` +
          "Install the project's dependencies (`npm install`).",
      },
    };
  }
  try {
    return { file, runtime, list: collect(runtime) };
  } catch (error) {
    return { file, error: valueError(error) };
  }
}

function derive(collection, read) {
  if (collection.error) return failed(collection.file, collection.error);
  try {
    return succeeded(collection.file, read(collection.runtime, collection.list));
  } catch (error) {
    return failed(collection.file, valueError(error));
  }
}

function succeeded(file, value) {
  return { file, error: null, value };
}

function failed(file, error) {
  return { file, error, value: null };
}

function readAgent(module) {
  const fields = {
    kind: "missing",
    name: { kind: "ABSENT" },
    model: null,
    declares_disable_memory: false,
    unsupported_keys: [],
    async_subagents: [],
  };
  if (!("agent" in module)) return fields;
  const agent = module.agent;
  if (typeof agent === "function") {
    const metadata = agent[AGENT_FACTORY];
    if (metadata === undefined) return { ...fields, kind: "bare_function" };
    return {
      ...fields,
      kind: "factory",
      name: nameValue(metadata?.name),
      async_subagents: Array.isArray(metadata?.asyncSubagents)
        ? metadata.asyncSubagents.map(nameValue)
        : [],
    };
  }
  if (agent === null || typeof agent !== "object" || agent.kind !== "deep-agent") {
    return { ...fields, kind: "other" };
  }
  const config = agent.config !== null && typeof agent.config === "object" ? agent.config : {};
  return {
    ...fields,
    kind: "static",
    name: "name" in config ? nameValue(config.name) : { kind: "ABSENT" },
    model: typeof config.model === "string" ? config.model : null,
    declares_disable_memory: "disableMemory" in config,
    unsupported_keys: UNSUPPORTED_AGENT_KEYS.filter((key) => key in config),
    async_subagents: (Array.isArray(config.subagents) ? config.subagents : [])
      .filter((spec) => spec !== null && typeof spec === "object" && spec.async === true)
      .map((spec) => nameValue(spec.name)),
  };
}

function nameValue(name) {
  return typeof name === "string" ? { kind: "STATIC", value: name } : { kind: "DYNAMIC" };
}

function readIdentity(identity) {
  if (identity === null || typeof identity !== "object" || identity.kind !== "identity") {
    return { declared: false, auth: null, auth_omitted: false, scope: null, declares_memory: false };
  }
  const config = identity.config ?? {};
  const http = config.ingress?.http;
  const auth =
    http === "backend"
      ? "backend"
      : http?.mode === "langsmith_api_key"
        ? "langsmith_api_key"
        : http?.mode === "validated_token"
          ? "validated_token"
          : null;
  const scope = config.scope !== null && typeof config.scope === "object" ? config.scope : null;
  return {
    declared: true,
    auth,
    auth_omitted: false,
    scope: typeof scope?.credentials === "string" ? { credentials: scope.credentials } : null,
    declares_memory: scope !== null && "memory" in scope,
  };
}

/** Classify the root `memory` declaration the way `memory_layers.py` did. */
function readMemory(memory) {
  if (memory === null || typeof memory !== "object" || memory.kind !== "memory") {
    return { kind: "absent", detail: null };
  }
  const scope = memory.config?.scope;
  if (!Array.isArray(scope)) return { kind: "dynamic", detail: "a computed value" };
  return reduceLayers(scope.map(layerEntry));
}

function layerEntry(layer) {
  if (layer === null || typeof layer !== "object" || Array.isArray(layer)) {
    return typeof layer === "string"
      ? { kind: "shorthand", detail: JSON.stringify(layer) }
      : { kind: "dynamic", detail: "a dynamic list entry" };
  }
  let type = null;
  let hasId = false;
  for (const [key, value] of Object.entries(layer)) {
    if (key === "type") {
      if (typeof value !== "string") return { kind: "dynamic", detail: "a computed layer `type`" };
      type = value;
    } else if (key === "id") {
      if (value === undefined) continue;
      hasId = true;
      if (typeof value !== "string") return { kind: "dynamic", detail: "a computed layer `id`" };
    } else if (key !== "allow") {
      return { kind: "reserved", detail: `the key \`${key}\`` };
    }
  }
  if (type === null) return { kind: "unknown", detail: "a layer with no `type`" };
  if (type === "agent") {
    return hasId
      ? { kind: "unknown", detail: "an `id` on the agent layer" }
      : { kind: "agent", detail: null };
  }
  if (type === "user") return { kind: "user", detail: null };
  return { kind: "unknown", detail: `type ${JSON.stringify(type)}` };
}

function reduceLayers(entries) {
  let agent = false;
  let user = false;
  for (const entry of entries) {
    if (entry.kind === "agent") {
      if (agent) return { kind: "duplicate_agent", detail: null };
      agent = true;
    } else if (entry.kind === "user") {
      if (user) return { kind: "duplicate_user", detail: null };
      user = true;
    } else {
      return entry;
    }
  }
  const kind = agent && user ? "agent_user" : agent ? "agent" : user ? "user" : "none";
  return { kind, detail: null };
}

function readSandbox(module) {
  if (!("sandbox" in module)) return { declared: false, is_sandbox: false, options: {}, errors: {} };
  const sandbox = module.sandbox;
  if (sandbox === null || typeof sandbox !== "object" || sandbox.kind !== "sandbox") {
    return { declared: true, is_sandbox: false, options: {}, errors: {} };
  }
  const given = sandbox.options !== null && typeof sandbox.options === "object" ? sandbox.options : {};
  const options = {};
  const errors = {};
  for (const key of SANDBOX_KEYS) {
    if (!(key in given) || given[key] === undefined) continue;
    try {
      options[key] = plainJson(given[key]);
    } catch (error) {
      if (!(error instanceof NotJson)) throw error;
      errors[key] = `property \`${key}\` is not static: ${error.message}`;
    }
  }
  return { declared: true, is_sandbox: true, options, errors };
}

function readSchedule(module) {
  if (!("schedule" in module)) return { declared: false, is_schedule: false, config: null, error: null };
  const schedule = module.schedule;
  if (schedule === null || typeof schedule !== "object" || schedule.kind !== "schedule") {
    return { declared: true, is_schedule: false, config: null, error: null };
  }
  const { kind: _kind, ...config } = schedule;
  try {
    return { declared: true, is_schedule: true, config: plainJson(config), error: null };
  } catch (error) {
    if (!(error instanceof NotJson)) throw error;
    return { declared: true, is_schedule: true, config: null, error: error.message };
  }
}

function exportsOf(module, name) {
  return { has_export: name in module, has_default: "default" in module };
}

function channelIngress(channel) {
  if (channel === null || typeof channel !== "object") return null;
  if (channel[CHANNEL_BRAND] !== true) return null;
  return INGRESS_PROVIDERS.has(channel.kind) ? channel.kind : null;
}

/** The connection declarations of the collected `defineMcp(...)` connectors. */
function mcpConnections(connectors) {
  const entries = [];
  for (const connector of connectors) {
    if (!connector || typeof connector !== "object" || connector.kind !== "mcp_servers") continue;
    const connectorName = typeof connector.name === "string" ? connector.name : "";
    const servers =
      "servers" in connector && connector.servers && typeof connector.servers === "object"
        ? connector.servers
        : null;
    if (!servers) continue;
    for (const [serverName, server] of Object.entries(servers)) {
      if (!server || typeof server !== "object") continue;
      const url = typeof server.url === "string" ? server.url : "";
      const connection =
        "connection" in server && server.connection && typeof server.connection === "object"
          ? server.connection
          : null;
      const slug = typeof connection?.slug === "string" ? connection.slug : "";
      const ownerPolicy =
        connection?.owner &&
        typeof connection.owner === "object" &&
        typeof connection.owner.type === "string"
          ? connection.owner.type
          : "";
      if (!url || !slug || !ownerPolicy) continue;
      entries.push({
        connector_name: connectorName,
        server_name: serverName,
        url,
        slug,
        owner_policy: ownerPolicy,
      });
    }
  }
  return entries;
}

/** `value` as plain JSON data, or a `NotJson` naming what is not. */
function plainJson(value, seen = new Set()) {
  switch (typeof value) {
    case "string":
    case "boolean":
      return value;
    case "number":
      if (!Number.isFinite(value)) throw new NotJson("a non-finite number is not plain JSON data");
      return value;
    case "function":
      throw new NotJson("a function is not plain JSON data");
    case "undefined":
      throw new NotJson("undefined is not plain JSON data");
    case "symbol":
      throw new NotJson("a symbol is not plain JSON data");
    case "bigint":
      throw new NotJson("a bigint is not plain JSON data");
  }
  if (value === null) return null;
  if (seen.has(value)) throw new NotJson("a circular reference is not plain JSON data");
  seen.add(value);
  try {
    if (Array.isArray(value)) {
      const items = [];
      for (let index = 0; index < value.length; index++) {
        if (!(index in value)) throw new NotJson("array holes are not supported");
        items.push(plainJson(value[index], seen));
      }
      return items;
    }
    const prototype = Object.getPrototypeOf(value);
    if (prototype !== Object.prototype && prototype !== null) {
      const name = value.constructor?.name || "object";
      throw new NotJson(`an instance of ${name} is not plain JSON data`);
    }
    const object = {};
    for (const [key, item] of Object.entries(value)) {
      if (item === undefined) continue;
      try {
        object[key] = plainJson(item, seen);
      } catch (error) {
        if (!(error instanceof NotJson)) throw error;
        throw new NotJson(`property \`${key}\` is not static: ${error.message}`);
      }
    }
    return object;
  } finally {
    seen.delete(value);
  }
}

function describeError(error, file) {
  const located = Array.isArray(error?.errors) ? error.errors[0] : undefined;
  if (located?.location && typeof located.text === "string") {
    // An esbuild (tsx) syntax error: report its position in the module.
    return {
      kind: "syntax",
      message: located.text,
      line: located.location.line,
      column: located.location.column + 1,
    };
  }
  // Errors thrown in tsx's loader thread arrive with only their message.
  const reported = ESBUILD_ERROR.exec(String(error?.message ?? ""));
  if (reported && path.resolve(reported[1]) === path.resolve(file)) {
    return {
      kind: "syntax",
      message: reported[4],
      line: Number(reported[2]),
      column: Number(reported[3]) + 1,
    };
  }
  return { kind: "import", message: errorText(error) };
}

const ESBUILD_ERROR = /^Transform failed with \d+ errors?:\n(.+?):(\d+):(\d+): ERROR: (.*)$/m;

function valueError(error) {
  return { kind: "evaluate", message: errorText(error) };
}

await main();
