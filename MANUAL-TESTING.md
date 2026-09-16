# CLI integration testing

All commands below use the local `bin/langsmith` build, not your globally installed
release. The walkthrough creates disposable resources and optionally invokes
model inference; review each step's scope before running it.

## 1. Build and select the demo workspace

From the repository root:

```bash
make build
bin/langsmith --version
```

Use your configured `demo` profile. List workspaces and copy the Demo Workspace ID:

```bash
env -u LANGSMITH_API_KEY -u LANGCHAIN_API_KEY bin/langsmith --profile demo --format json workspace list
```

Replace placeholders before continuing. These commands assume bash/zsh and `jq`.
The wrapper ignores ambient API keys so the saved profile is used, without changing
your shell's credentials. It always requests JSON for copy/paste and assertions.

```bash
LS_WORKSPACE_ID='REPLACE_WITH_DEMO_WORKSPACE_ID'
LS_TEST_TAG="cli-integration-$(date +%Y%m%d-%H%M%S)"
LS_TEST_DIR="$(mktemp -d)"
set -o pipefail
lsdemo() {
  env -u LANGSMITH_API_KEY -u LANGCHAIN_API_KEY ./bin/langsmith \
    --profile demo --workspace "${LS_WORKSPACE_ID:?Set the demo workspace ID}" \
    --format json "$@"
}
```

Stop if any command fails. JSON files in `LS_TEST_DIR` can contain trace data; keep
them private. Capturing an ID in a shell variable prints nothing—that is normal.
Commands piped through `tee` below show and save the actual responses.

## 2. Choose an existing synthetic trace (read-only)

```bash
lsdemo project list --limit 20
```

Choose a demo tracing project you own. This is the **source** project, not the empty
project we create in step 3. Never use customer or production traces for this guide.

```bash
LS_SOURCE_PROJECT_ID='REPLACE_WITH_SOURCE_PROJECT_ID'
lsdemo trace list --project-id "$LS_SOURCE_PROJECT_ID" --limit 3
LS_TRACE_ID='REPLACE_WITH_ONE_ROOT_RUN_ID_FROM_THAT_PROJECT'
```

These reads should return resources from the selected workspace. Check the project
and root-run ID before continuing; feedback and queue steps will reference this run.

## 3. Create a project and dataset (writes)

```bash
lsdemo project create --name "$LS_TEST_TAG" --description 'Disposable CLI integration project' \
  | tee "$LS_TEST_DIR/project.json"
LS_TEST_PROJECT_ID="$(jq -er '.id' "$LS_TEST_DIR/project.json")"

lsdemo dataset create --name "$LS_TEST_TAG" | tee "$LS_TEST_DIR/dataset.json"
LS_DATASET_ID="$(jq -er '.id' "$LS_TEST_DIR/dataset.json")"
```

Expect `status: created` and resource IDs. Project output includes `workspace_id`.
The new project is empty: creating it does not instrument an agent or create traces.
If creation is unverified, inspect the workspace before repeating the write.

## 4. Preview, import, and retry a trace (preview read-only; apply writes)

```bash
lsdemo dataset add --dataset "$LS_DATASET_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --trace-id "$LS_TRACE_ID" --dry-run --output "$LS_TEST_DIR/selection.json"
jq . "$LS_TEST_DIR/selection.json"
```

Expect one example, `reference_mode: inputs-only`, and `selection_info.has_more:
false` for this explicit ID. Inspect the actual input payload before applying.
The preview file is created privately and refuses to overwrite an existing file.

```bash
lsdemo dataset add --dataset "$LS_DATASET_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --selection "$LS_TEST_DIR/selection.json" | tee "$LS_TEST_DIR/import.json"
LS_EXAMPLE_ID="$(jq -er '.results[0].example_id' "$LS_TEST_DIR/import.json")"

lsdemo dataset add --dataset "$LS_DATASET_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --selection "$LS_TEST_DIR/selection.json"
```

First apply: expect `counts.created: 1`, `failed: 0`, and source/destination IDs.
Second apply: expect `counts.skipped: 1` and the same example ID, with no duplicate.

Optional filter preview (still no dataset write):

```bash
lsdemo dataset add --dataset "$LS_DATASET_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --error --last-n-minutes 1440 --limit 2 --dry-run --output "$LS_TEST_DIR/filtered.json"
jq '.selection_info' "$LS_TEST_DIR/filtered.json"
```

Expect 0–2 selected roots; `has_more` indicates whether more eligible roots were
found. Thread imports likewise select separate root turns, not a merged conversation.
Observed outputs only become references when explicitly requested with
`--reference-mode observed`; that is intentionally not used in this guide.

## 5. Update the imported example (write)

```bash
lsdemo example list --dataset "$LS_DATASET_ID" --limit 20
lsdemo example update "$LS_EXAMPLE_ID" --outputs '{"answer":"Manually reviewed demo reference"}'
lsdemo example list --dataset "$LS_DATASET_ID" --limit 20
```

Expect the new reference output and unchanged inputs. This answer is only a test
fixture, not a useful gold label. Do the retry test in step 4 **before** this edit:
replaying the original selection afterward should protect the changed example,
not overwrite your correction.

## 6. Create, inspect, and filter feedback (creates one feedback item)

Only use the synthetic trace chosen above. The feedback is attached to that source
run, not the new empty project. Use the UI to remove it later if desired.

```bash
lsdemo run feedback create --project-id "$LS_SOURCE_PROJECT_ID" --run-id "$LS_TRACE_ID" \
  --key "$LS_TEST_TAG" --score 0 --comment 'CLI integration test' \
  | tee "$LS_TEST_DIR/feedback.json"
LS_FEEDBACK_ID="$(jq -er '.feedback_id' "$LS_TEST_DIR/feedback.json")"
lsdemo run feedback get "$LS_FEEDBACK_ID"
lsdemo run feedback list --run-id "$LS_TRACE_ID" --key "$LS_TEST_TAG" --has-score --limit 20

lsdemo run feedback create --project-id "$LS_SOURCE_PROJECT_ID" --run-id "$LS_TRACE_ID" \
  --id "$LS_FEEDBACK_ID" --key "$LS_TEST_TAG" --score 0 --comment 'CLI integration test'
```

Expect `created`, exact score `0`, a matching filtered item, then `skipped` on retry.
`unverified` exits nonzero: inspect the returned ID before retrying, and reuse it.
`--has-score=false` selects missing scores; omitting the flag selects both cases.

## 7. Create and populate an annotation queue (writes)

```bash
lsdemo queue create --name "$LS_TEST_TAG" --description 'Disposable CLI review queue' \
  --dataset "$LS_DATASET_ID" | tee "$LS_TEST_DIR/queue.json"
LS_QUEUE_ID="$(jq -er '.id' "$LS_TEST_DIR/queue.json")"
lsdemo queue list --limit 20
lsdemo queue add "$LS_QUEUE_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --trace-id "$LS_TRACE_ID" --dry-run --output "$LS_TEST_DIR/queue-plan.json"
jq . "$LS_TEST_DIR/queue-plan.json"
```

Check that the plan references only the new queue and selected source run. Then:

```bash
lsdemo queue add "$LS_QUEUE_ID" --project-id "$LS_SOURCE_PROJECT_ID" --plan "$LS_TEST_DIR/queue-plan.json"
lsdemo queue items "$LS_QUEUE_ID" --limit 20
```

Expect the selected run in the new queue. Do not use replay as a no-op assertion:
re-adding a queue item can reopen its review.

### Reviewer instructions and rubric (writes only to the new queue)

Use the rubric JSON example in [README.md](README.md#reviewer-instructions-and-rubric)
to prepare `rubric.json`, then run:

```bash
lsdemo queue get "$LS_QUEUE_ID"
lsdemo queue configure "$LS_QUEUE_ID" --rubric rubric.json \
  --instructions 'Check correctness and explain errors.' --dry-run
lsdemo queue configure "$LS_QUEUE_ID" --rubric rubric.json \
  --instructions 'Check correctness and explain errors.' --apply
lsdemo queue get "$LS_QUEUE_ID"
```

The preview must not change the queue. Apply acknowledges the update; the final
read should show the rubric and instructions under `queue`, with the name and
default dataset unchanged. Rubric replacement does not merge old criteria.

## 8. Validate Insights (no model job yet)

These commands sample up to 5 eligible roots from the last 24 hours.
Adjust the business context and trace time range for your application. Provider
availability comes from workspace settings, not your local application.

```bash
lsdemo insights create --project-id "$LS_SOURCE_PROJECT_ID" \
  --name "$LS_TEST_TAG" --model openai --sample 5 --last-n-hours 24 \
  --user-context '{"Business goal":"Resolve support requests accurately"}' --dry-run
lsdemo insights create --project-id "$LS_SOURCE_PROJECT_ID" \
  --name "$LS_TEST_TAG" --model openai --sample 5 --last-n-hours 24 \
  --user-context '{"Business goal":"Resolve support requests accurately"}' \
  --summary-prompt 'Summarize the request and outcome. Inputs: {{run.inputs}} Outputs: {{run.outputs}}' \
  --dry-run --preview-run "$LS_TRACE_ID"
```

Expect `status: dry_run`, the proposed request, and variable bindings/missing paths
for the explicit run. No report is created. Preview does not prove that this run
matches the time window, will be sampled, or that model access is configured.

### Optional: create a paid report

**The next command invokes workspace model inference and may incur cost.** Run it
only after approving the configuration and scope:

```bash
lsdemo insights create --project-id "$LS_SOURCE_PROJECT_ID" \
  --name "$LS_TEST_TAG" --model openai --sample 5 --last-n-hours 24 \
  --user-context '{"Business goal":"Resolve support requests accurately"}' \
  | tee "$LS_TEST_DIR/insights.json"
```

Capture the returned job ID (creation is asynchronous):

```bash
LS_INSIGHTS_JOB_ID="$(jq -er '.id' "$LS_TEST_DIR/insights.json")"
lsdemo insights get "$LS_INSIGHTS_JOB_ID" --project-id "$LS_SOURCE_PROJECT_ID"
lsdemo insights list --project-id "$LS_SOURCE_PROJECT_ID" --limit 20
```

After the report completes, inspect its evidence:

```bash
lsdemo insights runs "$LS_INSIGHTS_JOB_ID" --project-id "$LS_SOURCE_PROJECT_ID" --limit 20
```

Expect report status/results and a page of evidence. Evidence IO is preview-only;
read the source run for full payloads. Do not repeat `create` to poll a report.

## 9. Local error checks (expected nonzero exits, no writes)

Run individually; these failures are intentional:

```bash
lsdemo project create --name ' '
lsdemo run feedback list --run-id "$LS_TRACE_ID" --limit 101
lsdemo run feedback create --run-id "$LS_TRACE_ID" --key review --comment ' '
```

Expect no success result on stdout and a JSON error on stderr. To inspect terminal
formatting, repeat a **read** command with a final `--format pretty`:

```bash
lsdemo run feedback get "$LS_FEEDBACK_ID" --format pretty
```

## 10. Optional cleanup (deletion)

This deletes only the queue created in this guide; check its ID first:

```bash
printf 'Queue to delete: %s\n' "$LS_QUEUE_ID"
lsdemo queue delete "${LS_QUEUE_ID:?Queue ID must be set}" --yes
```

Retain the dataset, empty tracing project, feedback, and optional report for team
review, or delete those exact test resources through the UI. Do not delete the
source tracing project. Local JSON artifacts remain in `LS_TEST_DIR`; handle them
as trace data and remove them when no longer needed. This guide does not claim
that a general automated cleanup command has been tested.
