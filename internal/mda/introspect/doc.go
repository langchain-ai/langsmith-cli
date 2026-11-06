// Package introspect answers what `langsmith deploy` needs to know about a
// project's authored modules by importing them, never by parsing them. It
// replaces every place the mda Rust CLI read authored source: `agent_config.rs`,
// `typescript.rs`, `text_module.rs`, `python_extract*`,
// `compile/{model,identity,exports,static_extract}.rs`, and the source-reading
// halves of `compile/{memory,sandbox_recipe,schedules,requirements,local_packages}.rs`.
//
// # Helpers
//
// Two helper programs are embedded in the binary. For a TypeScript project,
// `helper/introspect.mjs` runs under `node` from the project root and imports
// the modules with tsx, after registering loader hooks (`helper/hooks.mjs`) that
// inline `import x from "./f.md" with { type: "text" }` the way the build does.
// The hooks are registered with `module.registerHooks` where Node.js has it, so
// they also serve the text imports of CommonJS modules (`.cts`, `require`), and
// with `module.register` otherwise. The helper needs Node.js 20.6 or newer
// (`helper/toolchain.mjs`).
//
// tsx 4.23.15, esbuild 0.28.2 and es-module-lexer 3.0.4 (all MIT) are installed
// with `npm ci --ignore-scripts` from the embedded `helper/package-lock.json`,
// which pins each package's integrity hash, into the CLI's cache
// (`os.UserCacheDir()/langsmith/mda-helper/deps-<key>`), never into the project.
// The key hashes the manifest and lockfile, this binary's GOOS/GOARCH, and the
// arch and major version of the Node.js that runs the helper. An install is
// marked complete only once the helper loaded the packages from it; a helper
// that later reports its packages do not load gets them reinstalled, once.
//
// For a Python project, `helper/introspect.py` runs with the project interpreter
// ([proc.ProjectPython]), isolated (`-I`) and without writing bytecode (`-B`),
// with the project root first on `sys.path`.
//
// Importing a module runs its top-level code, which may read the environment as
// it will when deployed, so the helper runs with the project's `.env` values
// under the process environment, which wins ([ImportOptions]).
//
// A helper reads a request naming the project's modules on stdin and writes one
// JSON document ([DocumentVersion]) to the file the request names in `output`;
// not to stdout, where a project module may write at the file-descriptor level.
// The document has one section per module: what was read off its exports, or
// why that failed (a syntax error, an exception while importing). A module
// that fails to import is an error naming it, never "not configured". The
// document also carries the connector requirements, MCP connection entries and
// channel requirements the project's own `managed-deepagents` computes from the
// collected `tools/mcp` and `channels/` exports, so the emit functions need no
// build directory. Go decodes the document strictly: a field either side has
// and the other lacks is an error, so the two cannot drift apart unnoticed.
//
// The helper runs once per project per process; every question about the same
// project is answered from the cached document.
//
// # The document
//
//	{
//	  "version": 1,
//	  "error": {"kind": "helper" | "toolchain" | "node", "message": "..."},  // only when the helper could not run
//	  "sdk": {"version": "0.10.0" | null, "named_factories": true},
//	  "agent": {"file": "agent.ts", "error": null, "value": {
//	    "kind": "static" | "factory" | "bare_function" | "missing" | "other",
//	    "name": {"kind": "STATIC", "value": "x"} | {"kind": "DYNAMIC"} | {"kind": "ABSENT"},
//	    "model": "openai:gpt-5" | null,
//	    "declares_disable_memory": false,
//	    "unsupported_keys": ["memory"],
//	    "async_subagents": [{"kind": "STATIC", "value": "researcher"}]}},
//	  "identity": {"file": "identity.ts", "value": {"declared": true,
//	    "auth": "backend" | "langsmith_api_key" | "validated_token" | null,
//	    "auth_omitted": false, "scope": {"credentials": "user"} | null, "declares_memory": false}} | null,
//	  "memory": {"file": "memory.ts", "value": {"kind": "agent" | "user" | "agent_user" | "none" |
//	    "absent" | "duplicate_agent" | "duplicate_user" | "shorthand" | "reserved" | "unknown" |
//	    "dynamic", "detail": "..." | null}} | null,
//	  "sandbox": {"file": "sandbox/index.ts", "value": {"declared": true, "is_sandbox": true,
//	    "options": {"dockerImage": "python:3.12"}, "errors": {"registry": "property `registry` is not static: ..."}}} | null,
//	  "mcp": {"file": "tools/mcp.ts", "value": {"has_export": true, "has_default": false}} | null,
//	  "legacy_connectors": [{"file": "connectors/x.ts", "value": {"has_export": false, "has_default": false}}],
//	  "channels": [{"file": "channels/slack.ts", "value": {"has_export": true, "has_default": false,
//	    "ingress": "slack" | "github" | "linear" | null}}],
//	  "schedules": [{"file": "schedules/daily.ts", "value": {"declared": true, "is_schedule": true,
//	    "config": {"cron": "0 9 * * *", "prompt": "..."}, "error": null}}],
//	  "connector_requirements": {"file": "tools/mcp.ts", "value": {"mcp": {...}}},  // when tools/mcp exports mcp
//	  "mcp_connections": {"file": "tools/mcp.ts", "value": [{"connector_name": "mcp", ...}]},
//	  "channel_requirements": {"file": "channels/slack.ts", "value": {"slack": {...}}}  // when a channel is exported
//	}
//
// A section that failed has `"error": {"kind": "syntax" | "import" | "evaluate" |
// "helper", "message": ..., "line": ..., "column": ..., "detail": ...}` and a null
// value. A whole-document error of kind `helper` is the project's (its
// `managed-deepagents` does not import), `toolchain` the cached packages', and
// `node` a Node.js too old for the helper.
//
// In lex mode (`{"mode": "lex", "files": [{"source": ...}]}`) the TypeScript
// helper writes `{"version": 1, "files": [{"imports": [...], "error": null | "..."}]}`
// with each module-level import / export-from statement and its UTF-8 byte
// span, for [TransformTypescriptSources]. In check
// mode (`{"mode": "check"}`) it loads its packages and writes `{"version": 1}`.
package introspect
