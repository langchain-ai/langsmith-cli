# Porting `mda deploy` to `langsmith deploy`

`langsmith deploy` builds a Managed Deep Agents project and ships it to LangSmith
Deployment. It is a port of the `mda deploy` command of the Managed Deep Agents
SDK's Rust CLI (MIT, Copyright (c) 2026 LangChain, Inc.).

Source of truth for the port (read-only):

    /Users/ramon/.t3/worktrees/open-swe/t3-9d70cc8b/vendor/managed-deepagents-sdk/crates/mda/src

Referred to below as `$MDA`. The published npm package (dist only, read-only
reference) is at `/Users/ramon/src/managed-deepagents-npm`, the Python package at
`/Users/ramon/.t3/worktrees/open-swe/t3-9d70cc8b/vendor/managed-deepagents-sdk/packages/pypi`.

## Ground rules

1. **Line by line.** Port each Rust module function by function, in the same
   order, keeping doc comments (adapted) and every user-facing message verbatim,
   except where this document says otherwise. A reviewer should be able to put a
   Go file next to its Rust file and follow along.
2. **Naming is mechanical.** Rust `crate::foo::bar_baz` becomes Go
   `<package>.BarBaz` in the package the table below maps `foo` to; structs,
   enums and fields become CamelCase with the same words (`assistant_id` →
   `AssistantID`, `is_factory` → `IsFactory`). Packages ported in parallel call
   each other by these names before they exist; keep to the rule so the pieces
   fit together.
3. **No source parsing.** Everything the Rust CLI reads from authored source
   with `oxc` or the `python3` AST scripts is read by importing the project's
   modules in a helper subprocess instead (package `introspect`). Go never parses
   TypeScript or Python.
4. **Authentication is the LangSmith CLI's.** Never require or mint an API key.
   OAuth profiles are first class. All LangSmith traffic goes through
   `internal/client.Client` (the `SDK` field), which attaches the active
   credential: `--api-key`/`LANGSMITH_API_KEY`, or the selected profile's API
   key or OAuth token, refreshed under the shared lock. `mda`'s `login.rs`, its
   PAT cache (`state.rs`) and the paste-a-key prompt are dropped; where `mda`
   would sign in, point at `langsmith auth login` (or run that flow when
   interactive). A `LANGSMITH_API_KEY` in the project `.env` is still honoured,
   as `mda` does, ahead of profiles (`--api-key` comes first). The CLI's own
   credentials only go to the origin of the CLI's API URL, never to one a
   project names, and deploy's clients are `client.Options.Isolated`, so the
   SDK adds no ambient credential or workspace to them.
5. **Use the SDK.** Use `langsmith-go` typed services where they exist
   (`Workspaces`, `Repos`, `Repos.Directories`, `Commits`, `Sandboxes.*`,
   `Agents`, `Info`, `Settings`). For endpoints it lacks (host backend
   deployments, workspace secrets, Agent Server crons, channel apps…), send the
   request through `SDK.Execute` with an absolute URL, as
   `client.Client.doHTTP` does, so the SDK still attaches and refreshes auth.
   Never build `X-Api-Key` headers by hand. The workspace (`X-Tenant-Id`) comes
   from the client options.
6. **Telemetry is dropped.** Remove `telemetry::*`, `otel_logger`,
   `update_facts`, `refresh_identity` and `telemetry::stage::enter` calls.
7. **Cancellation is a `context.Context`.** `Interrupt` and `interrupt.check()`
   become `ctx` (from `signal.NotifyContext` in the command) and `ctx.Err()`.
   Every function that does I/O, runs a child or sleeps takes `ctx` first.
8. **Errors.** `anyhow::Result<T>` → `(T, error)`; `.context("x")` /
   `with_context` → `fmt.Errorf("x: %w", err)`; `bail!` → `fmt.Errorf`.
   `Result<Option<T>>` → `(T, bool, error)` or `(*T, error)`, whichever reads
   better. Error categories (`error.rs`) live in package `errs`.
9. **CLI wording.** Messages naming `mda deploy` say `langsmith deploy`
   instead. Messages about commands this CLI does not have (`mda dev`,
   `mda init`, `mda delete`, `mda connections …`) keep pointing at `mda` (the
   SDK's CLI still has them).
10. **Tests.** Port each module's `#[cfg(test)] mod tests` for the code you
    port (skip tests of code this port replaces). Follow the repository's
    rules: `t.Context()`, `httptest` servers for HTTP, no conditional asserts,
    specific expectations. `go test ./internal/mda/<pkg>/...` must pass.
11. **Dependencies.** Only OSI-licensed modules. Only the owner of a wave
    edits `go.mod` (see below); never add a `replace`. Never edit
    `internal/generated/`.
12. **Style.** `gofmt`, `go vet` clean, golangci-lint clean. Comments: keep
    the Rust doc comments (adapted to Go), no new narration.

## Package map

All packages live under `internal/mda/`.

| Go package | Ports (`$MDA/…`) |
|---|---|
| `errs` | `error.rs`, `command.rs` |
| `proc` | `child.rs`, `toolchain.rs`, `interrupt.rs` (as ctx helpers), `path_util.rs` |
| `ui` | cliclack (intro/outro/log/spinner/prompts/notes) and `cli_output.rs`; `secret_input.rs` |
| `project` | `project.rs`, `naming.rs`, `context_layout.rs`, `version.rs`, `install.rs`, `dependencies.rs`, `local_context_hub.rs`, `deploy/env_file.rs` |
| `hostapi` | `langsmith/*` except `login.rs` (dropped), plus `github.rs` |
| `deployment` | `deploy/helpers.rs`, `deploy/langsmith_context.rs`, the credential resolution in `deploy/mod.rs` (`resolve_api_key`, `client_without_prompt`, …, rewritten per rule 4) |
| `introspect` | `agent_config.rs`, `typescript.rs`, `text_module.rs`, `python_extract*`, `compile/{model,identity,exports,static_extract}.rs`, and the source-reading halves of `compile/{memory,sandbox_recipe,schedules,requirements,local_packages}.rs`, plus the embedded helper programs |
| `compile` | the rest of `compile/*` |
| `connection` | `connection/*` (what deploy reaches) |
| `channels` | `deploy/{channel_setup,channel_app,channel_icon,slack_app,trigger_channels,trigger_icon}.rs`, `deploy/github_channel/*`, `deploy/linear_channel/*` |
| `deploy` | `deploy/{mod,archive,bake,context_hub,image,upsell}.rs` |
| `internal/cmd/deploy.go` | `DeployArgs` / `run_deploy` in `cli.rs` |

Not ported: `dev`, `chat`, `init`, `eval`, `logs`, `delete`, `channel`,
`coding_agent`, `telemetry`, `state`, `http`, `deploy/prepare.rs` (run by the
remote build, not by deploy), `langsmith/login.rs`. Port a helper from them
only when deploy reaches it.

## The installed SDK replaces the launcher

`mda` is launched by the npm or PyPI package, which sets `MDA_INSTALL_CHANNEL`,
`MDA_BUNDLED_RUNTIME_PATH` and `MDA_RUNTIME_VERSION`. `langsmith deploy` has no
launcher, so `project.Installation` is resolved from the project's own installed
`managed-deepagents`:

- TypeScript: Node resolution of `managed-deepagents/package.json` from the
  project root (`node_modules` walking up). Runtime source = `<pkg>/dist`
  (marker `runtime/index.js`), version = its `package.json` `version`.
- Python: the project interpreter (`proc.ProjectPython`), asked for
  `managed_deepagents`'s directory (marker `runtime.py`) and
  `importlib.metadata.version("managed-deepagents")`.

Missing package → error telling the author to install the project's
dependencies (`npm install` / `uv sync`). `validate_authoring_package_version`
compares the declared version with the installed one.

`mda` ships in lockstep with its runtime; `langsmith deploy` vendors whichever
release the project installed, so:

- `ResolveInstallation` fails fast unless the installed release is in
  `project.SupportedSDKRange()` (`>=0.10.0, <0.11.0`, by release segment): the
  generated entry calls `asyncSubagentGraphs` / `async_subagent_graphs`,
  `asyncSubagent` and `browserHandoffWatcher` / `handoff_watcher`, which
  arrived in 0.10.0 on both registries. A package installed from a local
  directory (editable or `file:` install, `npm link`) is a source tree and is
  not range-checked.
- What compile copied from the SDK's own manifest comes from the installed
  package (`Installation.RuntimeManifest()`): npm `dependencies`, `main` and
  `exports` (rebased from `dist/`); PyPI `Requires-Dist` without an extra and
  `Requires-Python`, gathered by the install probe. The vendored
  `__runtime__/package.json` also lists the npm dependencies, and every managed
  dependency the runtime declares takes the runtime's range.

## Introspection (the "run it, don't parse it" contract)

`introspect` runs one helper per language that imports the project's modules
and prints one JSON document; Go decodes it into the same structs the Rust
extractors return (`AgentStaticConfig`, `IdentitySummary`, memory layers,
sandbox bases, schedule configs, `ChannelScan`, `ConnectorRequirements`,
`ChannelRequirements`, `McpConnectionEntry` …). Its exported Go functions keep
the Rust names and signatures of the functions they replace
(`ReadAgentStaticConfig(ctx, p)`, `ReadIdentitySummary`,
`IdentityRequiresIngressSecret`, `ResolveMemory`, `ProjectUsesSandbox`,
`ScanExportedConnectors`, `ScanExportedChannels`,
`EmitConnectorRequirementsForEntries`, …), so `compile`, `deploy` and
`connection` call them exactly as the Rust code calls its extractors.

- The helpers are embedded in the binary (`go:embed`): `helper/introspect.mjs`
  run with `tsx` for TypeScript projects, `helper/introspect.py` run with the
  project interpreter for Python projects. The TypeScript helper registers a
  Node module hook that serves `import x from "./f.md" with { type: "text" }`.
- The TypeScript helper also does the two source rewrites compile needs
  (inline text modules; list imports that leave the project root), using
  `es-module-lexer` for import positions.
- `tsx`, `esbuild` and `es-module-lexer` are installed with `npm ci` from an
  embedded lockfile into a CLI-owned cache directory
  (`os.UserCacheDir()/langsmith/mda-helper`), keyed by platform and Node.js
  major version, never into the project. The helper writes its document to a
  file the request names, not stdout.
- Modules import with the project's `.env` under the process environment;
  deploy hands its parsed `.env` to `introspect.ImportProject`, which runs
  under a spinner.
- Agent factories use the new SDK form `defineDeepAgent("name", factory, { asyncSubagents })`
  / `define_deep_agent("name", factory, async_subagents=[...])`: the exported
  value carries its name and async subagent names. A bare function export (the
  old factory form) is an error telling the author to wrap it, or, when the
  installed SDK has no named factories (no published release has them yet),
  that it does not, naming its version.
- Values the Rust code only accepted as literals (schedules, sandbox config)
  must be JSON values; the helper rejects functions, class instances and other
  non-JSON values with the same messages.

## Notes from finished waves

Read the package docs of finished packages before using them.

- `errs`: category constants are unprefixed (`errs.UserInput`, `errs.Remote`).
  `errs` imports `proc`, so `proc` never imports `errs`. Prompt aborts are
  `errs.ErrCancelled`; `errs.IsCancellation` also matches `context.Canceled`.
  HTTP error types implement `errs.StatusError` (`HTTPStatus() int`) so
  `errs.CategoryOf` sees their status.
- `proc`: `proc.Spawn` gives a nil stdio stream the null device (as `exec.Cmd`
  does), unlike Rust's inherit; set `cmd.Stdout = os.Stdout` to pass output
  through. `TeeStderr` is called before `Spawn`. `proc.InstallInterrupt` is the
  command's signal handling (first Ctrl-C cancels ctx).
- `ui`: package-level functions mirror cliclack (`ui.Intro`, `ui.LogInfo`,
  `ui.Spinner()`, `ui.Confirm(p).Interact(ctx)`, `ui.Password`, `ui.Select[T]`,
  `ui.NoteLinks` …) on `ui.Default()`; tests swap it with `ui.SetDefault`.
  Prompts without a terminal return `ui.ErrNotInteractive`. `ui.ResolveValue`
  takes a `ui.LookupEnv` closure instead of the project `.env` map.
- `project`: `project.LoadProject(ctx, root) (*Project, *Installation, error)`
  replaces `Installation::from_env` + `load_project`; the installed SDK version
  is `Installation.PackageVersion()` (no global `version::package_version`).
- `hostapi`: Go initialisms (`APIError`, `IsSelfHosted`). Enum values carry
  their type name (`hostapi.DeploymentTypeDev`, `hostapi.RepoOwnershipMissing`);
  data-carrying enums are structs with a `Kind` field; Rust `as_str` is
  `String()`; `Option<String>` is a plain string, empty meaning none.
  `hostapi.New(ls *client.Client, hostURL, platformURL)` and
  `hostapi.NewContextHubClient(ls, base)`; credentials and workspace come from
  the CLI client. Absolute-URL requests use `client.(*Client).DoAbsolute`.
  The GCS signed-URL upload must carry no LangSmith credential: plain
  `net/http`.
- `deployment`: `deployment.CLI{APIURL, APIKey, Options, Login}` is what the
  cmd layer passes in (`Options` resolves the CLI's own options lazily); `deployment.ResolveLangSmithContext(ctx, cli, envPath, envVars,
  hostOverride, workspaceFlag, workspaceHint)` replaces
  `LangSmithContext::resolve` and builds `Client()` / `ContextHubClient()`.
  `ResolveCredential` replaces `resolve_api_key`. Secrets helpers take
  `*[]hostapi.Secret`. `RecipeDigest` takes pointer optionals.
- `introspect`: read its `doc.go`. It owns the whole of `compile/memory.rs`
  (`ResolveMemory`, `MemoryDecision`, `DeclareMemorySnippet`) and the
  discovery locators (`FindSandboxEntry`, `FindMcpEntry`,
  `FindChannelEntries`, `FindIdentityEntry`, `FindMemoryEntry`,
  `EntryStemName`, `ReadSetupScript`, …). For `compile`: sandbox manifest
  writing/loading, `read_schedule_crons` and its cron logic, the requirement
  post-processing helpers, `read_instructions`, `project_has_skills`,
  `find_browser_entry`, `misspelled_channel_directory_warning` and the import
  specifiers remain to port. Use `introspect.SandboxRegistryManifest`,
  `introspect.ScheduleDeclaration`, `introspect.ConnectorRequirements`,
  `ChannelRequirements`, `McpConnectionEntry` (alias them where compile's API
  re-exports them). TypeScript sources are rewritten in one batch with
  `introspect.TransformTypescriptSources`.
- `connection`: `CreateDiscoveredMcpConnection(ctx, *hostapi.Client, slug,
  mcpServerURL)`, `WaitForOAuthSession`, `WaitForOAuthStatus`,
  `PollOAuthSession`, `OAuthWaitMessages`.
- `channels`: `channels.Prepare(requirements, wait, projectRoot,
  projectFiles, language, assistantID) (*ChannelSetup, error)`, then
  `Preflight(ctx, client, existingDeployment func(ctx) (map[string]any,
  error), envVars)`, `NeedsLiveURL()`, `SetUp(ctx, channels.SetupTarget{…})`;
  `channels.ListsEventsURL(manifest)` for `http_channel_links`.
- `compile`: `compile.CompileForDeploy(ctx, p, outDir, compile.DeployContext{…},
  inst)` with `inst` the `*project.Installation`; `compile.ShowBuildSummary`.
  Functions Rust re-exported from `compile` (memory, identity, sandbox use,
  emits) are called on `introspect`. Requirement helpers and loaders
  (`compile.LoadSandboxRecipeManifest`, `compile.RequiredEnvFromConnectorRequirements`,
  …) are in `compile`.
- `deploy`: `deploy.Run(ctx, cli, deploy.DeployOptions{…})` returns the
  session's `errs.CommandCompletion`, and an error only for failures before the
  intro. The preflight secret checks are methods of one `secretPreflight`
  (the Rust threaded the same six arguments through each). Not ported, as
  only `dev`/`chat`/`delete` reach them: `missing_local_model_api_key`,
  `delete_managed_crons`, `ensure_project_langsmith_api_key` and the other
  PAT helpers (rule 4). `internal/cmd/deploy.go` signs in through `loginAt`,
  the `langsmith auth login` device flow at the deploy's platform API.

## Waves

Each wave's packages are ported in parallel. Package owners only edit their own
directory. The wave owner listed first may edit `go.mod`.

1. `errs`+`proc`+`ui` (owns go.mod), `project`; SDK change in the open-swe tree.
2. `hostapi` (owns go.mod), `introspect`.
3. `deployment` + `compile`.
4. `connection`, `channels`.
5. `deploy` + `internal/cmd/deploy.go`, then integration and review.
