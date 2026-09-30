# AGENTS.md

## Repository overview

`langsmith-cli` is an agent-first Go CLI for querying and managing LangSmith resources. It uses Cobra for commands and the generated [`langsmith-go`](https://github.com/langchain-ai/langsmith-go) SDK for LangSmith API access.

- `cmd/langsmith/`: CLI entry point.
- `internal/cmd/`: commands, flags, and command tests.
- `internal/client/`: shared LangSmith client setup.
- `internal/output/`: JSON, table, and tree output helpers.
- `scripts/`: installation scripts.
- `README.md`: installation, authentication, command, local-development, and release documentation.

## Development

The module targets the Go version declared in `go.mod`.

- Build: `make build`
- Format: `make fmt`
- Lint: `make lint`
- Vet: `make vet`
- Focused test: `go test ./internal/cmd -run '<TestName>'`

Keep commands scriptable and preserve established output formats. Add focused tests for new commands, flags, request parameters, and output behavior.

A command that selects a project takes both `--project` and `--project-id`; register the pair with `addProjectFlags` (or `addCommonFilterFlags`, which calls it) and resolve it with `resolveSessionID`. Callers building a command line programmatically should pass the UUID, since project names are user-authored and may contain shell metacharacters. `TestEveryProjectCommandAcceptsProjectID` fails if a new command offers only one of the two.

## Custom app templates

Every starter in `internal/cmd/templates/` uses the published Macaw components,
tokens, and CLI. Use Macaw components and semantic styles for all new and edited
UI, including chat. Keep the shared dependency versions, stylesheet import,
Tailwind preset, host theme provider, and generated agent guidance intact.
Validate changes by scaffolding into a temporary directory, type-checking and
building each affected starter, and previewing it in the sandbox in both themes.
Never install dependencies inside the embedded template directories.

## LangSmith API access

Use the generated Go SDK through the shared client's `SDK` field. Do not add raw API calls when the endpoint is available in `langsmith-go`, and do not copy existing raw-call patterns for new code.

If an endpoint is missing from the Go SDK, expose and configure it through the public OpenAPI and Stainless definitions in `langchain-ai/langchainplus`, release the generated `langsmith-go` client, and update this repository's SDK dependency. Prefer that workflow over adding `RawGet`, `RawPost`, `RawPatch`, or other direct HTTP calls.

Leave authentication to the SDK client. Build clients with `GetClient`/`getClient`, which resolve `--api-key`, `LANGSMITH_API_KEY`, and profiles and hand the result to the SDK, as an API key or through `langsmith.WithProfile`. The SDK applies it to every request, including the raw helpers, and refreshes a profile's OAuth tokens under a lock shared by every process using the config. Do not read tokens from the config file, set `Authorization` or `X-API-Key` headers yourself, or call the OAuth token endpoint outside `auth login`. Refreshing in the CLI once raced parallel commands on the single-use refresh token, and the server answers a replayed token by revoking every session for the user. The raw helpers send requests through `SDK.Execute` for the same reason, so do not give them their own HTTP client. The one exception is the credential-free client the `api` command uses for other hosts. If a command needs a credential as a string, as `auth token` does, use `Client.AuthHeaders`.

If the SDK's auth or request handling is missing something, fix it in the SDK instead of working around it here. Hand-written SDK code lands in `langchain-ai/langsmith-go-staging`, not `langsmith-go`, and reaches this repository through an SDK release.

## Releasing

Releases are tag-driven: pushing a `v*` tag builds and publishes the GitHub Release, so there is no version file or changelog to edit. See [Releasing](README.md#releasing) for the full procedure.
