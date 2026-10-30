// Test stub of the `managed-deepagents` npm package: the authoring functions
// return values shaped like the published package's, which is all the
// introspection helper reads.
import { CHANNEL_BRAND, CONNECTOR_BRAND } from "./runtime/index.js";

export const AGENT_FACTORY = Symbol.for("managed-deepagents.agent-factory");
const UNSUPPORTED = {
  backend: "Configure the sandbox in sandbox/.",
  store: "MDA provides the store automatically.",
  checkpointer: "MDA provides the checkpointer automatically.",
  systemPrompt: "Use `instructions` in a factory entrypoint or put the static prompt in instructions.md.",
};

function hide(target, key, value) {
  Object.defineProperty(target, key, { value, enumerable: false });
}

function definition(caller, config) {
  for (const [key, guidance] of Object.entries(UNSUPPORTED)) {
    if (key in config) throw new Error(`Remove \`${key}\` from ${caller}(...). ${guidance}`);
  }
  if ("disableMemory" in config) throw new Error(`disableMemory is no longer supported in ${caller}(...).`);
  return { kind: "deep-agent", config: { ...config } };
}

export function defineDeepAgent(config) {
  if (typeof config.name !== "string" || config.name.trim() === "") {
    throw new Error('name is required in defineDeepAgent(...). Pass a non-empty string (e.g. name: "research-assistant").');
  }
  return definition("defineDeepAgent", config);
}

export function defineAgent(...args) {
  if (typeof args[0] !== "string") return definition("defineAgent", args[0]);
  const [name, rest, options = {}] = args;
  if (typeof rest !== "function") return definition("defineAgent", { ...rest, name });
  const named = async (runtime) => rest(runtime);
  hide(named, AGENT_FACTORY, Object.freeze({ name, asyncSubagents: Object.freeze([...(options.asyncSubagents ?? [])]) }));
  return named;
}

export function defineIdentity(options) {
  if (options === null || typeof options !== "object") {
    throw new Error("defineIdentity(...) requires an options object ({ auth }).");
  }
  if (options.auth === undefined) {
    throw new Error("defineIdentity(...) requires `auth`. Use auth.langsmithApiKey() for LangSmith workspace API keys.");
  }
  const http =
    options.auth === "backend"
      ? "backend"
      : options.auth.mode === "langsmith_api_key"
        ? options.auth
        : { mode: "validated_token", providers: Array.isArray(options.auth) ? options.auth : [options.auth] };
  return { kind: "identity", config: { ingress: { http }, scope: { credentials: "user" } } };
}

export const auth = {
  langsmithApiKey: () => ({ mode: "langsmith_api_key" }),
  supabase: (options) => ({ id: "supabase", claims: { user: "sub" }, jwks: `https://${options.projectRef}.supabase.co/jwks`, audience: "authenticated" }),
};

export function memoryLayer(options = {}) {
  if (Object.keys(options).some((key) => key !== "allow")) throw new Error("memoryLayer(...) only accepts `allow`.");
  return Object.freeze({ kind: "memory-layer", ...options });
}

export function defineMemory(options = {}) {
  if (options.scope !== undefined && (options.agent !== undefined || options.user !== undefined)) {
    throw new Error("defineMemory(...) cannot mix `scope` with `agent` or `user`.");
  }
  const layers = [];
  for (const type of ["agent", "user"]) {
    const layer = options[type];
    if (layer === undefined) continue;
    if (layer?.kind !== "memory-layer") throw new Error(`defineMemory(...) \`${type}\` must be created with memoryLayer().`);
    layers.push({ type, ...(layer.allow !== undefined ? { allow: layer.allow } : {}) });
  }
  const scope = options.scope ?? layers;
  return Object.freeze({ kind: "memory", config: Object.freeze({ version: 2, scope: Array.isArray(scope) ? scope : [scope] }) });
}

export function defineSandbox(options) {
  if (options && "scope" in options) {
    throw new TypeError("sandbox scope was removed: sandbox reuse is always one box per thread. Remove scope from the sandbox declaration.");
  }
  return { kind: "sandbox", options: { ...(options ?? {}) } };
}

export function defineSchedule(config) {
  if (typeof config.cron !== "string" || config.cron.trim().split(/\s+/).length !== 5) {
    throw new Error("defineSchedule(...) requires `cron` to be a standard 5-field cron expression.");
  }
  return { kind: "schedule", ...config };
}

export function defineMcp(config) {
  const self = { kind: "mcp_servers", ...config };
  hide(self, CONNECTOR_BRAND, true);
  hide(self, "tools", async () => []);
  hide(self, "requirements", (name) => {
    const connections = Object.values(self.servers ?? {})
      .map((server) => server.connection)
      .filter(Boolean);
    const slugs = (type) => connections.filter((c) => c.owner?.type === type).map((c) => c.slug).sort();
    return {
      name,
      provider: "mcp",
      autoReply: false,
      requiredPermissions: [],
      requiredEnv: [],
      userConnections: slugs("user"),
      agentConnections: slugs("agent"),
      providerConfig: {},
    };
  });
  return self;
}

function channel(kind, fields) {
  const self = { kind, ...fields };
  hide(self, CHANNEL_BRAND, true);
  hide(self, "events", () => undefined);
  hide(self, "requirements", (name) => ({
    name,
    provider: kind,
    eventsPath: `/channels/${name}/events`,
    requiredPermissions: [],
    requiredEnv: [],
  }));
  return self;
}

export const channels = {
  slack: (options = {}) => channel("slack", { transport: "trigger_server", app: { ...options } }),
  github: () => channel("github", {}),
  linear: () => channel("linear", {}),
  http: (adapter) => channel(adapter.provider, {}),
};
