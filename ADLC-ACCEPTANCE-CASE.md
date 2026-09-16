# Coding-agent onboarding: end-to-end ADLC acceptance case

Status: test specification, not an executed workflow. Commands refer to the local
integration build. No new live resources or inference were used to prepare this case.

## Objective

Determine whether a coding agent can take a user from an empty tracing project
through a working agent, curated dataset, offline evaluation, online monitoring,
Insights analysis, human review, and a verified improvement. Record both product
outcomes and the guidance needed to reach them.

The CLI is the resource/control interface, not the agent runtime. Application
scaffolding, instrumentation, model calls, and launching offline experiments use
application/SDK code today. Do not invent `init`, `experiment run`, or
`experiment compare` commands. Engine/rule work is outside this case.

## Scenario and approvals

User prompt: “Help me build and evaluate a refund-support agent in LangSmith.”

Use Demo Workspace, an existing saved profile, and uniquely named test resources.
Resolve the workspace before creating the first project. Get approval for the
specific live resources, model usage budget, evaluator sampling, and cleanup.
Do not use real customer data, payments, emails, or refunds.

Before implementation, the coding agent should ask:

- What is the refund policy, including boundary dates and exceptions?
- Which decisions require human approval? What must the agent never do?
- Which outcomes matter: correct decisions, grounded explanations, escalation?
- Which model/provider is available and what is the inference budget?

Proposed synthetic policy, subject to user confirmation: unopened items are
eligible within 30 days inclusive; opened items are denied; damaged items and
missing order information escalate. These are fixture rules, not inferred policy.

## Test phases

| Phase | Action and current interface | Evidence required to pass |
| --- | --- | --- |
| 1. Project | CLI `project create`; record workspace/project IDs | Project exists in the selected workspace; creation alone is not claimed to enable tracing |
| 2. Agent | Coding agent writes a small agent with synthetic order lookup, policy lookup, and escalation tools; optional policy-specialist subagent | Real model execution, tool calls, and delegation if configured; no external business actions |
| 3. Tracing | Configure SDK tracing separately from inference credentials; execute 3 smoke inputs; inspect with CLI `trace list/get` | Correct project, root/child hierarchy, actual inputs/outputs, timing and errors; no exposed secrets |
| 4. Dataset | CLI `dataset create`, `example create`, optionally `dataset add` preview/apply | 12 user-reviewed cases, correct references, memberships, and provenance; observed agent answers are not automatically treated as gold |
| 5. Offline baseline | Validate evaluator logic on observed output shape; optionally register code evaluator with CLI; launch experiment using SDK | Results for all 12 cases; distinguish missing/error results from scores; record dataset snapshot, agent revision, model and evaluator versions |
| 6. Online monitoring | Ask user to approve metrics; register project-targeted evaluator through CLI; send new bounded synthetic traffic | Evaluator enabled on intended roots, feedback actually appears, known failures are caught; rule creation alone is not success |
| 7. Insights | CLI create dry-run, then approved report creation, status reads and evidence reads | Completed report over intended time/sample; findings traceable to evidence; no unsupported completeness claim |
| 8. Human review | Review suspicious cases; optionally CLI queue create/add/items | User confirms intended behavior before references or evaluation criteria change |
| 9. Improve and re-evaluate | Change agent, not holdout labels; launch second SDK experiment on same snapshot/split | Before/after evidence for every case; improvement measured without silently changing the benchmark |
| 10. Handoff | Record resource IDs, commands, links, results, remaining gaps, and cleanup choices | Another engineer can reproduce the run; paid monitoring is not accidentally left running |

## Dataset design

Use 12 cases across eligible returns, opened-item denials, damaged-item escalation,
missing information, and day-30/day-31 boundaries. Assign 8 to `development` and
4 to `holdout`; use metadata for scenario labels such as `damaged_item`.
Keep development and holdout disjoint for this test. Include deliberately wrong
fixture outputs to check evaluator calibration, but do not store them as gold.

An example input might be `{ "order_id": "demo-001", "question": "Can I return this?" }`.
The reviewed reference might be `{ "decision": "approve" }`.
Confirm the actual agent output shape from smoke traces before writing extraction
or evaluator code. Proposed metrics: exact decision correctness offline, and
policy-grounded decision plus required-field checks online.

Online evaluators do not receive dataset reference answers. They must use approved
policy and facts actually present in the evaluated run. Missing facts must produce
an explicit unavailable/error outcome, not a fabricated passing score.

## Representative commands

These are templates, not a fully runnable harness. Substitute captured IDs and
approved names. `lsadlc` denotes `bin/langsmith` with explicit saved profile,
workspace, and JSON output; ignore ambient API keys when selecting a saved profile.
`evaluators.py` is to be authored only after inspecting actual agent outputs.

```bash
lsadlc project create --name ADLC_PROJECT --description 'Synthetic ADLC acceptance test'
lsadlc trace list --project-id PROJECT_ID --last-n-minutes 60 --limit 3
lsadlc trace get ROOT_RUN_ID --project-id PROJECT_ID --full

lsadlc dataset create --name ADLC_DATASET
lsadlc example create --dataset DATASET_ID \
  --inputs '{"order_id":"demo-001","question":"Can I return this?"}' \
  --outputs '{"decision":"approve"}' \
  --metadata '{"scenario":"eligible_return","source":"reviewed_synthetic"}' \
  --split development
lsadlc example list --dataset DATASET_ID --split development --limit 20
lsadlc dataset version get --dataset DATASET_ID --as-of latest

# Registering an evaluator does not execute the agent against the dataset.
lsadlc evaluator upload evaluators.py --name ADLC_DECISION_CORRECTNESS \
  --function decision_correctness --dataset ADLC_DATASET
# Launch the baseline using an application-specific SDK evaluate/aevaluate script.
lsadlc experiment list --dataset ADLC_DATASET --limit 5
lsadlc experiment get EXPERIMENT_NAME_OR_ID

# Use deterministic code first; subjective LLM judging is a separately approved option.
lsadlc evaluator upload evaluators.py --name ADLC_ONLINE_POLICY \
  --function online_policy_check --project-id PROJECT_ID --sampling-rate 1
# Inspect the effective selector before sending traffic. Code upload's trace-filter
# selects whole traces; it is not a root-only run filter. This build requires the
# UI or generic API to set a root-only filter for a code evaluator when needed.
# Generate 10 new synthetic requests after the rule is configured.
lsadlc run feedback list --run-id NEW_ROOT_RUN_ID --limit 20

lsadlc insights create --project-id PROJECT_ID --last-n-hours 1 \
  --sample 10 --model openai \
  --user-context '{"Business goal":"Correct refund decisions and appropriate escalation"}' \
  --dry-run
# Only with approved model usage: repeat without --dry-run; capture job ID.
lsadlc insights get JOB_ID --project-id PROJECT_ID
lsadlc insights runs JOB_ID --project-id PROJECT_ID --limit 20
```

Sampling 1 is intentional only for this bounded synthetic project, not a production
default. Insights sample 10 means up to 10 eligible roots, not necessarily the ten
latest requests; use an exact time window when isolating a batch. Do not repeat
create to poll. Bound polling and stop on terminal failure or timeout.

## Restaurant integration observations (September 16, 2026)

A separate retained synthetic restaurant scenario exercised the lifecycle above
using the local integration build. These results do not mean the refund scenario
or every CLI command has been tested end-to-end.

- A 12-example golden dataset with development/holdout splits and a version tag
  supported two real-model experiments. Action correctness improved from 7/12 to
  9/12; both metrics eventually reported all 12 scores. Execution used SDK
  `evaluate`, not a CLI experiment runner.
- Nine two-turn online conversations received thread-judge feedback. Feedback was
  visible in individual thread stats before propagating to the thread list. This
  verified live evaluation, not thread backfill.
- The judge missed structured ownership/tool evidence and produced questionable
  cancellation scores. Validate rendered judge inputs and calibrate against known
  cases; successful execution is not proof of evaluator accuracy.
- Initial Insights reports each returned all nine eligible latest-root-per-thread
  representatives, rather than all 15 individual turns. Later batches increased
  the population. Report narratives covered narrower subsets than evidence lists.
- Manual categories and a numeric satisfaction attribute were accepted. Creating
  a one-off job did not create a dashboard card. A saved configuration and a run
  using its ID were needed; configuration creation and scheduling used generic
  API calls, not first-class Insights commands.
- Review queues and regression datasets were exercised separately from the fixed
  golden benchmark. Automatic dataset imports copy candidate outputs; review is
  required before treating those outputs as references.
- Fresh integration tests during the final review were blocked by sandbox access
  to the Go build cache. Prior capability-branch CI does not validate subsequent
  uncommitted integration changes; the combined PR needs its own CI run.

## Test the agent experience, not just the endpoints

For each phase record:

1. User request and any clarification asked by the coding agent.
2. Exact command/SDK call, exit status, returned IDs, and redacted output.
3. What help/docs/skills revealed, and what required source-code research.
4. Whether the agent identified the next useful lifecycle step without prompting.
5. Manual intervention, retries, missing information, and surprising defaults.

Run a discovery pass where the agent starts with CLI help and installed skills,
without this command recipe. Record where it gets stuck. Then run a guided pass
using this case. This distinguishes CLI discoverability from a prewritten script's
ability to call working endpoints. Compare completion, incorrect actions, tool
calls, human interventions, missing-result handling, latency, and model spend.

## Known blockers and limits

- Default version get is broken without explicit `--as-of latest`; exact timestamp
  lookup/tagging has a fractional-second issue. Until fixed, capture the version
  returned by explicit latest and use timestamp-string example reads; do not claim
  that a tag was reliably verified.
- Bulk-edit previews omit explicitly empty map fields. Avoid those writes in this
  lifecycle demo until fixed; keep a separate regression case for the bug.
- No dedicated onboarding/scaffolding command or offline experiment runner exists.
- The installed evaluator skill says LLM-judge creation is unsupported, but this
  build has `evaluator create-llm`. Treat current command help/source as evidence;
  record stale skill guidance rather than silently following it.
- Multipart attachment upload is missing from the pinned Go SDK; PDFs are excluded.
- No automatic Insights wait or experiment comparison command is assumed.

## Completion criteria

Do not call this passed because resource creation succeeded. Require a real traced
agent, reviewed dataset, complete offline results, feedback from newly generated
online traffic, a finished Insights report with evidence, and a repeatable second
evaluation. Report quality regressions honestly; an experiment need not improve
for the tooling workflow to function, but it must make regressions visible.
