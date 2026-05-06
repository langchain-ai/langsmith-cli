# Integration Test Convention

## Goal

Establish a single, reusable pattern for integration tests in `langsmith-cli` so that every command surface (`hub`, `prompt`, `evaluator`, `experiment`, `dataset`) can adopt it as it grows. Avoid the situation where the second contributor to write integration tests copies the first author's file and silently drifts.

## Scope

This spec defines the structural rules. The first implementation is `internal/cmd/hub_integration_test.go`; the same shape is expected for any future `*_integration_test.go` file in this repo.

## Rules

### 1. Build tag

Every integration test file begins with:

```go
//go:build integration

package cmd
```

The file is excluded from the default build. `go test ./...` (the standard CI command) ignores it. Opt in with `go test -tags=integration ./internal/cmd/` or `make test-integration`.

### 2. Env var gate

Even with the build tag enabled, every integration test calls `requireIntegrationEnv(t)` as its first statement. The helper skips with `t.Skip` if `LANGSMITH_API_KEY` is not set. This makes `go test -tags=integration ./...` safe to run anywhere; absent credentials produce a no-op rather than a failure.

```go
func TestX_Integration(t *testing.T) {
    requireIntegrationEnv(t)
    // ...
}
```

### 3. Resource naming

Test resources (repo handles, dataset names, project names) use a UUID-suffixed prefix to avoid collisions between concurrent CI runs and to make stale-resource cleanup easy.

```go
handle := randomHandle("cli-int")
// "cli-int-a1b2c3d4"
```

The `cli-int-` prefix is reserved for ephemeral integration test resources. A maintainer can run `langsmith hub list --query cli-int` to find leftovers.

### 4. Cleanup

Every test that creates a remote resource registers a `t.Cleanup` to delete it. Cleanup runs even when the test fails or panics.

```go
scheduleDelete(t, handle)
```

The cleanup function swallows errors. We do not want a flaky delete to mask the original failure.

### 5. Test shape

Each command surface gets one file (`<surface>_integration_test.go`). The file contains:

- A "headline" test that walks through a realistic user journey (init, create, read, list, update, delete) with assertions on JSON output and filesystem state at every step.
- One or two narrower tests for error paths the journey skips (404 on missing resource, guard rails, etc.).

Avoid splitting the headline into many tiny tests; the journey's value comes from running the steps in sequence the way a real user would.

### 6. Helpers

Integration tests use these helpers, defined in the same file (no shared package yet):

- `requireIntegrationEnv(t)` skip-if-no-credentials gate
- `randomHandle(prefix string) string` UUID-suffixed name generator
- `runHub(t, args...)` invokes the cobra root, returns parsed JSON
- `runHubExpectError(t, args...)` invokes and asserts non-nil error
- `scheduleDelete(t, handle)` registers cleanup
- For round-trip tests: `seedSkillContent`, `walkAndHash`, `assertSameTree`, `hashContents`

When a second command surface adopts this pattern, copy the helpers it needs and adjust names (`runHub` → `runPrompt`, etc.). Promote to `internal/cmdutil/integration` only when three commands use the same helper, not before.

## CI (follow-up)

The CI integration job is not in the initial PR (workflow files require an elevated GitHub token scope to write). A maintainer should add it as a small follow-up. Recommended shape: a separate `integration` job in `.github/workflows/ci.yml`:

- Runs on `pull_request`, `push` to main, and a daily schedule (`0 8 * * *` UTC).
- Gated on `secrets.LANGSMITH_API_KEY_TEST != ''` so forks and unconfigured repos see the job skipped.
- Uses `LANGSMITH_API_KEY=${{ secrets.LANGSMITH_API_KEY_TEST }}` and `LANGSMITH_ENDPOINT=${{ secrets.LANGSMITH_ENDPOINT_TEST }}`. The `_TEST` suffix is mandatory: it prevents accidentally pointing the job at a personal API key or a production tenant.

The integration job is intentionally not a required check on PRs. Schema drift catches the kind of bug that needs to be fixed but does not block unrelated work.

## Local development

```bash
export LANGSMITH_API_KEY="lsv2_pt_..."
make test-integration
```

The Makefile target runs only the integration tests (`-run Integration`). Running the full suite with the build tag (`go test -tags=integration ./internal/cmd/`) also works and additionally runs the unit tests.

## How to extend

To add integration tests for a new command surface (example: `prompt`):

1. Create `internal/cmd/prompt_integration_test.go` with `//go:build integration` at the top.
2. Copy the helpers from `hub_integration_test.go` that you need; rename `runHub` to `runPrompt`.
3. Write a `TestPromptIntegration_UserJourney` headline test plus a small number of narrower tests.
4. Update this spec doc's "first implementations" list below if helpful.

## First implementations

- `internal/cmd/hub_integration_test.go` covering `hub init/push/pull/list/get/delete`.

## Out of scope

- Acceptance tests that build the binary and shell out (`os/exec`). The in-process cobra invocation is sufficient for everything we currently want to verify.
- Cross-tenant or destructive-against-production tests. Use a dedicated test tenant.
- Performance benchmarks. Different concern, separate runner config.
