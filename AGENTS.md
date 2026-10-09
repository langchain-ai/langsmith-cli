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

Before a write command relies on a partial PATCH, read the backend handler to confirm it is not a full replace. `PATCH /runs/rules/{id}`, for example, clears webhooks, filters, and add-to-dataset/queue actions that the request omits, so `--replace` sends the existing rule's settings back. Cover such commands in `internal/cmd/evaluator_integration_test.go` (`make test-integration`, needs `LANGSMITH_API_KEY`): seed a resource with non-default settings, run the command, and check every setting afterwards.

If an endpoint is missing from the Go SDK, expose and configure it through the public OpenAPI and Stainless definitions in `langchain-ai/langchainplus`, release the generated `langsmith-go` client, and update this repository's SDK dependency. Prefer that workflow over adding `RawGet`, `RawPost`, `RawPatch`, or other direct HTTP calls.

## Releasing

Releases are tag-driven: pushing a `v*` tag builds and publishes the GitHub Release, so there is no version file or changelog to edit. See [Releasing](README.md#releasing) for the full procedure.
