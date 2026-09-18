# Manual testing

Use this checklist with the local build, a test workspace, and synthetic traces.
Run sections in order; stop on unexpected errors. **Nothing is deleted by default.**
Paid inference is optional. Requires bash/zsh and `jq`; keep generated files private.

## 1. Build and sign in

From the repository root:

```bash
make build
./bin/langsmith --version
LS_PROFILE='cli-review'
LS_TEST_DIR="$(mktemp -d)"
LS_TEST_TAG="cli-review-$(date +%Y%m%d-%H%M%S)"
set -o pipefail

lsreview() {
  env -u LANGSMITH_API_KEY -u LANGCHAIN_API_KEY \
    -u LANGSMITH_PROJECT -u LANGSMITH_ENDPOINT \
    -u LANGSMITH_WORKSPACE_ID -u LANGSMITH_TENANT_ID \
    ./bin/langsmith --profile "$LS_PROFILE" "$@"
}
lsreview auth login
lsreview workspace list
```

Complete browser authorization. Copy a test workspace ID, then:

```bash
LS_WORKSPACE_ID='REPLACE_WITH_TEST_WORKSPACE_ID'
lsreview workspace set-default "$LS_WORKSPACE_ID"
lsdemo() { lsreview --workspace "$LS_WORKSPACE_ID" --format json "$@"; }
```

The wrapper ignores ambient overrides without changing your shell. A custom
`LANGSMITH_CONFIG_FILE`, if set, remains in effect. Never paste credentials into
commands or review notes. A revoked login requires another `lsreview auth login`.

## 2. Create a project and test defaults — writes

```bash
lsdemo project create --name "$LS_TEST_TAG" --set-default | tee "$LS_TEST_DIR/project.json"
LS_TEST_PROJECT_ID="$(jq -er '.id' "$LS_TEST_DIR/project.json")"
lsdemo trace list --limit 5
lsdemo project configure
lsdemo project clear-default
lsdemo project set-default "$LS_TEST_PROJECT_ID"
```

Expect `status: created`, `default_saved: true`, then no traces. Defaults are scoped
to the profile, workspace, and endpoint. Project creation does not instrument an app.
Clearing the default removes only the local selection, not the project.

Choose an existing **synthetic** source project and root trace:

```bash
lsdemo project list --limit 20
LS_SOURCE_PROJECT_ID='REPLACE_WITH_SYNTHETIC_PROJECT_ID'
lsdemo trace list --project-id "$LS_SOURCE_PROJECT_ID" --limit 3 --include-io
LS_TRACE_ID='REPLACE_WITH_ROOT_RUN_ID'
```

This explicit project must override the saved empty project. Keep the source ID
separate from `LS_TEST_PROJECT_ID`; never use production data for this walkthrough.
