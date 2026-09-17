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

## 3. Import a trace into a dataset — preview, write, retry

```bash
lsdemo dataset create --name "$LS_TEST_TAG" | tee "$LS_TEST_DIR/dataset.json"
LS_DATASET_ID="$(jq -er '.id' "$LS_TEST_DIR/dataset.json")"
lsdemo dataset add --dataset "$LS_DATASET_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --trace-id "$LS_TRACE_ID" --dry-run --output "$LS_TEST_DIR/selection.json"
jq . "$LS_TEST_DIR/selection.json"
```

Inspect the IDs and input payload. Expect one inputs-only example; no reference
answer is inferred. The output file is private and refuses to overwrite a file.

```bash
lsdemo dataset add --dataset "$LS_DATASET_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --selection "$LS_TEST_DIR/selection.json" | tee "$LS_TEST_DIR/import.json"
LS_EXAMPLE_ID="$(jq -er '.results[0].example_id' "$LS_TEST_DIR/import.json")"
lsdemo dataset add --dataset "$LS_DATASET_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --selection "@$LS_TEST_DIR/selection.json"
```

Expect `counts.created: 1`, then `counts.skipped: 1`, the same example ID, and
`failed: 0`. Complete this retry **before** editing the example.

For bounded filter discovery, preview with `--error --last-n-minutes 1440 --limit 2`
instead of `--trace-id`. Inspect `selection_info.has_more`; a page is not a complete
dataset. Thread imports produce separate root-turn examples, not one conversation.

## 4. Edit examples, splits, and versions — writes

```bash
lsdemo dataset version get --dataset "$LS_DATASET_ID" | tee "$LS_TEST_DIR/version.json"
LS_BEFORE="$(jq -er '.version.as_of' "$LS_TEST_DIR/version.json")"
lsdemo example update "$LS_EXAMPLE_ID" --outputs '{"answer":"Reviewed test answer"}' --split test
lsdemo example list --dataset "$LS_DATASET_ID" --split test --limit 20
lsdemo dataset split list --dataset "$LS_DATASET_ID"
lsdemo dataset version diff --dataset "$LS_DATASET_ID" --from "$LS_BEFORE" --to latest
lsdemo example update "$LS_EXAMPLE_ID" --clear-splits
lsdemo dataset split list --dataset "$LS_DATASET_ID"
```

Expect the replacement output, `test` membership, and a modified example ID in the
diff. Clearing memberships does not delete the example. Versions are automatic;
splits are memberships, not separately created resources.

For bulk edits, read current IDs/timestamps and prepare `edits.json` using the
[bulk-edit format](README.md#bulk-edits). Only use examples in this test dataset:

```bash
lsdemo example list --dataset "$LS_DATASET_ID" --limit 20
lsdemo example update-bulk --dataset "$LS_DATASET_ID" --file edits.json --dry-run
lsdemo example update-bulk --dataset "$LS_DATASET_ID" --file @edits.json --apply
lsdemo example list --dataset "$LS_DATASET_ID" --limit 20
```

Expect per-example results and the intended changes on readback. Replaying an old
edit file should fail its timestamp check. Writes are non-atomic; read unverified
items before preparing a retry.

Tag the original snapshot and inspect history:

```bash
lsdemo dataset version tag --dataset "$LS_DATASET_ID" --as-of "$LS_BEFORE" --tag before-edits --dry-run
lsdemo dataset version tag --dataset "$LS_DATASET_ID" --as-of "$LS_BEFORE" --tag before-edits
lsdemo dataset version get --dataset "$LS_DATASET_ID" --as-of before-edits
lsdemo dataset version list --dataset "$LS_DATASET_ID" --limit 20
```

Expect the tag to resolve to `LS_BEFORE`. Tags can move; use fixed timestamps for
reviewed writes. Version diffs return IDs, not full before/after content.

Test configuration with a permissive schema on this dataset only:

```bash
lsdemo dataset configure --dataset "$LS_DATASET_ID" \
  --file '{"outputs_schema_definition":{"type":"object"}}' --dry-run
lsdemo dataset configure --dataset "$LS_DATASET_ID" \
  --file '{"outputs_schema_definition":{"type":"object"}}' --apply
lsdemo dataset get "$LS_DATASET_ID"
```

Expect the stored schema. Omitted settings stay unchanged; server validation runs
on apply. See [configuration](README.md#configure-schemas-and-transformations) for transformations.

## 5. Import assertion references — writes

```bash
lsdemo dataset add --dataset "$LS_DATASET_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --trace-id "$LS_TRACE_ID" --assertions '[{"key":"helpful","comment":"Address the request clearly."}]' \
  --dry-run --output "$LS_TEST_DIR/assertions.json"
jq . "$LS_TEST_DIR/assertions.json"
lsdemo dataset add --dataset "$LS_DATASET_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --selection "$LS_TEST_DIR/assertions.json"
```

Expect a new example with `outputs.assertions`, not copied agent output. This stores
criteria; it does not run an evaluator. Offline experiments remain SDK workflows.

## 6. Add feedback and a review queue — writes

```bash
lsdemo run feedback create --project-id "$LS_SOURCE_PROJECT_ID" --run-id "$LS_TRACE_ID" \
  --key "$LS_TEST_TAG" --score 0 --comment 'Synthetic review test' | tee "$LS_TEST_DIR/feedback.json"
LS_FEEDBACK_ID="$(jq -er '.feedback_id' "$LS_TEST_DIR/feedback.json")"
lsdemo run feedback get "$LS_FEEDBACK_ID"
lsdemo run feedback list --run-id "$LS_TRACE_ID" --key "$LS_TEST_TAG" --has-score --limit 20
lsdemo queue create --name "$LS_TEST_TAG" --dataset "$LS_DATASET_ID" | tee "$LS_TEST_DIR/queue.json"
LS_QUEUE_ID="$(jq -er '.id' "$LS_TEST_DIR/queue.json")"
lsdemo queue list --limit 20
lsdemo queue add "$LS_QUEUE_ID" --project-id "$LS_SOURCE_PROJECT_ID" \
  --trace-id "$LS_TRACE_ID" --dry-run --output "$LS_TEST_DIR/queue-plan.json"
jq . "$LS_TEST_DIR/queue-plan.json"
lsdemo queue add "$LS_QUEUE_ID" --project-id "$LS_SOURCE_PROJECT_ID" --plan "$LS_TEST_DIR/queue-plan.json"
lsdemo queue items "$LS_QUEUE_ID" --limit 20
```

Expect score `0` (not missing) and the selected run in the new queue. Queue replay
is not a no-op test: re-adding can reopen review. Feedback belongs to the source run.

Prepare `rubric.json` using an **existing workspace feedback key** and the
[rubric format](README.md#reviewer-instructions-and-rubric):

```bash
lsdemo queue configure "$LS_QUEUE_ID" --rubric rubric.json --instructions 'Explain incorrect answers.' --dry-run
lsdemo queue configure "$LS_QUEUE_ID" --rubric @rubric.json --instructions 'Explain incorrect answers.' --apply
lsdemo queue get "$LS_QUEUE_ID"
```

Expect stored `queue.rubric_items` and `queue.rubric_instructions`. Missing feedback
configurations must fail before writing. On this test queue, `--rubric '[]'` clears
the rubric; preview before applying. Rubrics do not create automated judges.

## 7. Preview online judges; optionally enable them

```bash
lsdemo model list
LS_MODEL_ID='REPLACE_WITH_EVALUATOR_CAPABLE_CONFIGURATION_ID'
lsdemo model get "$LS_MODEL_ID"
LS_EVAL_FILTER="and(eq(is_root,true),has(tags,\"$LS_TEST_TAG\"))"
judge_args=(--name "$LS_TEST_TAG-llm" --project-id "$LS_SOURCE_PROJECT_ID"
  --model-id "$LS_MODEL_ID"
  --prompt '[["system","Score 1 when the response is helpful, including a necessary clarification; score 0 for an empty or unhelpful response."],["human","Request: {{question}} Response: {{answer}}"]]'
  --schema '{"type":"object","properties":{"helpfulness":{"type":"integer","enum":[0,1]}},"required":["helpfulness"]}'
  --variable-mapping '{"question":"input.message","answer":"output.response"}'
  --filter "$LS_EVAL_FILTER" --sampling-rate 1 --spend-limit 1)
lsdemo evaluator create-llm "${judge_args[@]}" --dry-run --preview-run "$LS_TRACE_ID"
```

Use a source trace with `inputs.message` and `outputs.response`, or adjust the
mapping to its actual fields. Expect real bindings and `bindings_validated: true`.
Mappings use singular `input`/`output`. Model availability does not verify credentials.

**Optional, potentially billable:** after reviewing the prompt, filter, and budget:

```bash
lsdemo evaluator create-llm "${judge_args[@]}"
lsdemo evaluator get "$LS_TEST_TAG-llm" --session-id "$LS_SOURCE_PROJECT_ID"
```

The $1 weekly limit is service-enforced, not a per-call hard cap. No backfill is
requested. The unique tag restricts eligibility to your upcoming test runs.

For a code evaluator, save this as `response_present.py`:

```python
def response_present(run):
    response = (run.get("outputs") or {}).get("response")
    present = isinstance(response, str) and bool(response.strip())
    return {"score": int(present), "comment": "Nonempty response" if present else "Empty response"}
```

**Optional write:** upload it with the same filter:

```bash
lsdemo evaluator upload response_present.py --function response_present \
  --name "$LS_TEST_TAG-code" --project-id "$LS_SOURCE_PROJECT_ID" --trace-filter "$LS_EVAL_FILTER"
```

Run your synthetic app twice in the source project **after** creating the rules,
with tag `LS_TEST_TAG`: once with a helpful response, once with an empty response.
Use your app's tracing instrumentation; the CLI does not run the app for you.
Copy each resulting root ID and check:

```bash
LS_EVAL_RUN_ID='REPLACE_WITH_NEW_TAGGED_ROOT_ID'
lsdemo run feedback list --run-id "$LS_EVAL_RUN_ID" --limit 20
```

Allow time for processing. Code should score nonempty/empty as `1`/`0`. Inspect the
LLM's actual inputs and judgment if its score differs from your expectation.
An empty feedback list is **pending/unverified**, not a pass. Do not recreate a
rule to poll it. Thread judges and historical backfills require separate validation;
see [online judge settings](README.md#online-judge-settings).

## 8. Preview Insights; optionally run a report

```bash
insight_args=(--project-id "$LS_SOURCE_PROJECT_ID" --name "$LS_TEST_TAG"
  --model openai --sample 5 --last-n-hours 24
  --categories '{"Reservations":"Booking requests","Cancellations":"Canceling a booking","Complaints":"Service problems"}'
  --attributes '{"user_satisfaction":{"type":"number","description":"Infer satisfaction from 1 to 10 using trace evidence."}}')
lsdemo insights create "${insight_args[@]}" --dry-run
```

Expect a preview, **not sampling or model inference**. Choose a provider configured
in your workspace and categories relevant to your traces. Requested attribute
bounds are descriptive, not enforced. Samples may contain fewer than five runs.

**Optional, billable:** remove `--dry-run` only after approval:

```bash
lsdemo insights create "${insight_args[@]}" | tee "$LS_TEST_DIR/report.json"
LS_REPORT_ID="$(jq -er '.id' "$LS_TEST_DIR/report.json")"
lsdemo insights get "$LS_REPORT_ID" --project-id "$LS_SOURCE_PROJECT_ID"
lsdemo insights list --project-id "$LS_SOURCE_PROJECT_ID" --limit 20
lsdemo insights runs "$LS_REPORT_ID" --project-id "$LS_SOURCE_PROJECT_ID" --limit 20
```

Read evidence after success; do not repeat creation to poll. A one-off report need
not appear as a saved dashboard card. For saved manual configurations, use the
documented [configuration workflow](README.md#reuse-configurations-and-investigate-results).

## 9. Input forms and expected failures — no writes

Repeat a judge **dry-run** with the same prompt saved to `prompt.json`: compare
inline JSON, `--prompt prompt.json`, and `--prompt @prompt.json` using the actual
JSON from section 7. Settings/bindings should match. Do not publish preview output;
it may contain private trace data.

Run failures individually and expect nonzero exits with JSON diagnostics on stderr:

```bash
lsdemo project create --name ' '
lsdemo run feedback list --run-id "$LS_TRACE_ID" --limit 101
lsdemo evaluator create-llm "${judge_args[@]}" --dry-run \
  --variable-mapping '{"answer":"outputs.response"}'
lsdemo evaluator create-llm "${judge_args[@]}" --dry-run --preview-run "$LS_TRACE_ID" \
  --variable-mapping '{"answer":"output.nonexistent_field"}'
```

The missing-binding check also returns preview JSON on stdout. Invalid plural roots
must not create a judge. To inspect readable terminal output, repeat a read with
`--format pretty`. These checks do not establish full backend or deployment coverage.

## 10. Retain results; cleanup is optional

Keep resources for review. If you explicitly want to delete the **test queue only**:

```bash
lsdemo queue get "${LS_QUEUE_ID:?Set the test queue ID}"
lsdemo queue delete "$LS_QUEUE_ID" --yes
```

Never delete the source project. The test dataset, project, feedback, evaluators,
and report remain. Disable the test evaluators in the UI when finished; their tag
filters remain configured until you change/remove them. Local artifacts remain in
`LS_TEST_DIR`; handle them as private trace data.
