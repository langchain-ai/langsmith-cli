# langsmith-cli

An agent-first CLI for querying and managing [LangSmith](https://smith.langchain.com) resources.

Built for AI coding agents (deepagents, Claude Code, Cursor, etc.) and developers who need fast, scriptable access to projects, traces, runs, datasets, evaluators, experiments, and threads.

## Installation

### Install script (recommended)

macOS / Linux:

```bash
curl -fsSL https://cli.langsmith.com/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://cli.langsmith.com/install.ps1 | iex
```

### Upgrade

```bash
langsmith self-update
```

### GitHub releases

Download the latest binary for your platform from [GitHub Releases](https://github.com/langchain-ai/langsmith-cli/releases).

## Authentication

Set your API key as an environment variable:

```bash
export LANGSMITH_API_KEY="lsv2_pt_..."
```

Optionally set defaults:

```bash
export LANGSMITH_ENDPOINT="https://api.smith.langchain.com"  # For self-hosted
export LANGSMITH_WORKSPACE_ID="<workspace-id>"                # Default workspace
export LANGSMITH_PROJECT="my-default-project"                 # Default project for queries
```

Or save the credentials in a profile:

```bash
langsmith auth login                    # OAuth
langsmith profile create prod           # API key, taken from $LANGSMITH_API_KEY
```

Profiles live in `~/.langsmith/config.json`, written owner-only; the CLI warns
if it is readable by other users.

Other settings can be passed as flags:

```bash
langsmith --workspace <workspace-id> trace list --project my-app
```

## Quick Start

```bash
# List tracing projects
langsmith project list

# List recent traces in a project
langsmith trace list --project my-app --limit 5

# Get a specific trace with full detail
langsmith trace get <trace-id> --project my-app --full

# List LLM calls with token counts
langsmith run list --project my-app --run-type llm --include-metadata

# List datasets
langsmith dataset list

# List experiments for a dataset
langsmith experiment list --dataset my-eval-set
```

## Output Formats

```bash
langsmith trace list --project my-app
```

```bash
langsmith --format=json trace list --project my-app
```

```bash
langsmith trace list --project my-app -o traces.json
```

## Selecting a project

Every command that operates on a project takes either `--project <name>` or
`--project-id <session UUID>`, and `$LANGSMITH_PROJECT` supplies the name when
neither is set. The two flags are mutually exclusive.

```bash
langsmith trace list --project 'my-app'
langsmith trace list --project-id 519bb9dd-079b-4488-8610-e330951ea3e4
```

Prefer `--project-id` when a program is building the command line. Project names
are free-form — users can create one containing spaces, quotes, or shell
metacharacters — so a name has to be quoted correctly at every call site, and a
name that is quoted wrongly matches nothing and returns an empty result rather
than an error. A UUID needs no quoting. `--project-id` also skips the name
lookup, saving a round-trip.

`langsmith project list` returns the `id` to use.

## Command Reference

### `project` — Create, list, and delete tracing projects

A tracing project (session) is a namespace that groups related traces together. This lists only tracing projects, not experiments — use `experiment list` for those.

Results are **paginated** — by default, only the first **20** projects are returned (use `--limit` to change). Projects are sorted by **most recent activity** (`last_run_start_time`, descending).

```bash
# Create an empty tracing project in a specific workspace
langsmith --profile demo --workspace <workspace-id> project create --name my-app
langsmith project create --name my-app --description "Application traces"

# List tracing projects (default: 20 results, most recently active first)
langsmith project list
langsmith project list --limit 50

# Filter by name
langsmith project list --name-contains chatbot

# Machine-readable JSON
langsmith --format=json project list

# Permanently delete a project and all of its traces (requires confirmation)
langsmith project delete --project-id 519bb9dd-079b-4488-8610-e330951ea3e4
```

Creating a project does not instrument your application or enable tracing. Configure
its LangSmith tracing credentials and `LANGSMITH_PROJECT` separately, run the app,
then verify with `langsmith trace list --project-id <returned-id>`. Creation returns
JSON containing `status`, `id`, and `name`; it does not silently reuse an existing project.

### `trace` — Query and export traces

A trace is a tree of runs representing one end-to-end invocation of your application.

Results are **paginated** — by default, only the first **20** traces are returned (use `--limit` to change). Traces are sorted **newest-first** by start time. By default, only traces from the **last 7 days** are returned; use `--since` or `--last-n-minutes` to change the time window.

```bash
# List recent traces (default: 20 results, newest first)
langsmith trace list --project my-app
langsmith trace list --project my-app --limit 50 --last-n-minutes 60

# Filter traces
langsmith trace list --project my-app --error           # Only errors
langsmith trace list --project my-app --min-latency 5   # Slow traces (>5s)
langsmith trace list --project my-app --tags production  # By tag
langsmith trace list --project my-app --name "agent"     # By name

# Include additional fields
langsmith trace list --project my-app --include-metadata   # + status, duration, tokens, costs
langsmith trace list --project my-app --include-io         # + inputs, outputs, error
langsmith trace list --project my-app --include-feedback   # + feedback_stats
langsmith trace list --project my-app --full               # All fields (metadata + io + feedback)

# Show trace hierarchy (fetches full run tree for each trace)
langsmith trace list --project my-app --show-hierarchy --limit 3

# Get a specific trace
langsmith trace get <trace-id> --project my-app --full

# Export traces to JSONL files (one per trace)
langsmith trace export ./traces --project my-app --limit 20 --full

# Custom filename pattern (supports {trace_id} and {name} placeholders)
langsmith trace export ./traces --project my-app --filename-pattern "{name}_{trace_id}.jsonl"
```

### `run` — Query individual runs

A run is a single step within a trace (LLM call, tool call, chain step, etc.).

Results are **paginated** — by default, only the first **50** runs are returned (use `--limit` to change). Runs are sorted **newest-first** by start time. By default, only runs from the **last 7 days** are returned; use `--since` or `--last-n-minutes` to change the time window.

```bash
# List LLM calls (default: 50 results, oldest first)
langsmith run list --project my-app --run-type llm
langsmith run list --project my-app --run-type tool --name search

# Find expensive calls
langsmith run list --project my-app --run-type llm --min-tokens 1000 --include-metadata

# Include feedback scores
langsmith run list --project my-app --include-feedback

# Get a specific run
langsmith run get <run-id> --full

# Export to JSONL (default limit: 100)
langsmith run export llm_calls.jsonl --project my-app --run-type llm --full
```

> **Query backend:** the CLI selects the runs query API automatically from the deployment reported by `/info` — LangSmith Cloud and self-hosted `>= 0.16` use the v2 (SmithDB) API; older self-hosted uses v1. No flag is needed. A few v2-only features (`trace messages`, `thread messages`) are unavailable on self-hosted `< 0.16`.

### `thread` — Query conversation threads

A thread groups multiple root runs sharing a thread_id (multi-turn conversations).

Results are **paginated** — by default, only the first **20** threads are returned (use `--limit` to change). Threads are sorted by **most recent activity** (newest first).

```bash
# List threads (default: 20 results, newest first; requires --project)
langsmith thread list --project my-chatbot
langsmith thread list --project my-chatbot --last-n-minutes 120

# Get all turns in a thread
langsmith thread get <thread-id> --project my-chatbot --full
```

### `dataset` — Manage evaluation datasets

List results are **paginated** — by default, only the first **100** datasets are returned (use `--limit` to change).

```bash
# List datasets (default: 100 results)
langsmith dataset list
langsmith dataset list --name-contains eval

# Get dataset details
langsmith dataset get my-dataset

# Create and delete
langsmith dataset create --name my-eval-set --description "QA pairs for v2"
langsmith dataset delete my-old-dataset --yes

# Export examples to JSON
langsmith dataset export my-dataset ./data.json --limit 500

# Upload from JSON file
langsmith dataset upload data.json --name new-dataset
```

### Curate traces into a dataset

Use an existing destination dataset. Inputs-only is the default: a failed agent's
output should not silently become the expected answer.

```bash
# One explicit root trace; no default time window is applied to this ID.
langsmith dataset add --dataset regression-tests --project-id <project-uuid> \
  --trace-id <trace-uuid> --dry-run --output root-selection.json
langsmith dataset add --dataset regression-tests --project-id <project-uuid> \
  --selection root-selection.json

# Preview a bounded sample using the existing trace filters, then import exactly it.
langsmith dataset add --dry-run --dataset regression-tests --project-id <project-uuid> \
  --error --last-n-minutes 1440 --limit 20 --output selection.json
langsmith dataset add --dataset regression-tests --project-id <project-uuid> \
  --selection selection.json

# A specific child run, extracting an object within its inputs.
langsmith dataset add --dry-run --dataset tool-tests --project-id <project-uuid> \
  --run-ids <run-uuid> --inputs-pointer /request --output step-selection.json
```

Preview returns JSON containing the actual example payloads. With `--output`, it writes
that JSON to the file instead of stdout. `--output` creates
a new private file and refuses to overwrite; protect it as trace data. Filter-based
selection defaults to 20 roots in the last seven days; use time bounds and `--limit`
explicitly for your sample. This is not representative sampling.

Use `--reference-mode observed` only when the selected outputs are suitable references.
`--outputs-pointer` can select a nested output object in that mode. For corrected
references, edit the selection's `reference_mode` to `corrected` and supply an `outputs`
object for every example. Review the file before importing it. Pointers are JSON
Pointers (including array indices) and must resolve to objects, not scalar values.

Import checks the explicit destination, API endpoint, and access to the selected
source runs. It does not rerun a filter or replace frozen IO. Source run/trace/project
provenance is recorded. Identical imports use content-derived IDs and are skipped;
changed payloads or extraction settings create new examples. This does not deduplicate
older examples created by other commands. Existing examples are never overwritten.
Each result reports its example ID and created/skipped/recovered/failed status;
partial failure exits nonzero. Retry the same file to reconcile successful writes.

Annotation-queue promotion and representative sampling are not included in these commands.

### `example` — Manage dataset examples

List results are **paginated** — by default, only the first **20** examples are returned (use `--limit` to change). Use `--offset` to paginate through results.

```bash
# List examples (default: 20 results)
langsmith example list --dataset my-dataset
langsmith example list --dataset my-dataset --split test --limit 50

# Paginate through examples
langsmith example list --dataset my-dataset --limit 20 --offset 20

# Create an example
langsmith example create --dataset my-dataset \
  --inputs '{"question": "What is LangSmith?"}' \
  --outputs '{"answer": "A platform for LLM observability"}'

# Create with metadata and split assignment
langsmith example create --dataset my-dataset \
  --inputs '{"question": "What is tracing?"}' \
  --outputs '{"answer": "Recording LLM application execution"}' \
  --metadata '{"source": "manual", "version": 2}' \
  --split test

# Delete an example
langsmith example delete <example-id> --yes
```

### `evaluator` — Manage evaluator rules

```bash
# List evaluators
langsmith evaluator list

# Upload an offline evaluator (for experiments)
langsmith evaluator upload evals.py \
  --name accuracy --function check_accuracy --dataset my-eval-set

# Upload an online evaluator (for production monitoring)
langsmith evaluator upload evals.py \
  --name latency-check --function check_latency --project my-app

# Set sampling rate (evaluate a fraction of runs, 0.0-1.0)
langsmith evaluator upload evals.py \
  --name latency-check --function check_latency --project my-app --sampling-rate 0.5

# Replace an existing evaluator
langsmith evaluator upload evals.py \
  --name accuracy --function check_accuracy_v2 --dataset my-eval-set --replace --yes

# Delete an evaluator
langsmith evaluator delete accuracy --yes

# Create an LLM-as-judge evaluator (--model-config is always required)
# model.json: copy the structured.model block from an existing evaluator or the UI.
langsmith evaluator create-llm \
  --name relevance --project my-app \
  --prompt prompt.json --schema schema.json --model-config model.json \
  --variable-mapping '{"input":"input.question","output":"output.answer"}'

# Or reference an existing Prompt Hub commit (--hub-ref replaces --prompt and --schema)
langsmith evaluator create-llm \
  --name relevance --project my-app \
  --hub-ref my-org/relevance:latest --model-config model.json
```

### `experiment` — Query experiment results

List results are **paginated** — by default, only the first **20** experiments are returned (use `--limit` to change).

```bash
# List experiments (default: 20 results)
langsmith experiment list
langsmith experiment list --dataset my-eval-set

# Get experiment results (feedback stats, run stats)
langsmith experiment get my-experiment-2024-01-15
```

### `insights` — Create and inspect Insights reports

#### Configure an analysis

Use `--file/-f` for a reviewable JSON analysis. File keys match the API:

```json
{
  "name": "Support quality",
  "model": "openai",
  "sample": 20,
  "last_n_hours": 24,
  "partitions": {
    "Refunds": "Refund eligibility and refund requests",
    "Account access": "Login problems and account recovery"
  },
  "attribute_schemas": {
    "resolved": {
      "type": "boolean",
      "description": "The requested action was actually completed"
    }
  },
  "user_context": {"Business goal": "Resolve eligible support requests"},
  "summary_prompt": "Highlight recurring failures with supporting evidence"
}
```

Save this as `analysis.json`, replace `PROJECT_ID`, and use your configured profile/workspace:

```bash
# Validate the request without creating a paid job.
bin/langsmith --format json insights create --project-id PROJECT_ID --file analysis.json --dry-run
# After reviewing the request, submit it.
bin/langsmith --format json insights create --project-id PROJECT_ID --file analysis.json
```

Dry-run validates local configuration, not provider availability, service limits, or write
permission. It may read project metadata. It does not freeze the server's sampled traces;
relative time windows are evaluated again at submission. Service caps apply, so `sample`
is a requested count, not a spending limit or a guarantee of that many analyzed traces.

Alternatively, pass `--categories categories.json` (a name-to-description object, 1–10
entries) and `--attributes attributes.json` (the `attribute_schemas` object above) with
the ordinary analysis flags. Attributes support `string`, `number`, and `boolean`,
descriptions, and optional `filter_by`. These are Insights attributes, not online evaluators.

`--cluster-model` and `--summary-model` accept `openai`, `anthropic`, or a workspace
model-settings UUID. The service validates availability. `--model` remains the fallback provider.

`--file` cannot be combined with analysis flags. Unsupported fields (including scheduler,
credential, and endpoint settings) are rejected. Files must contain one JSON object under
1 MiB. `--output/-o` writes creation or dry-run JSON to a file.

#### Reuse configurations and investigate results

```bash
bin/langsmith --format json insights create --project-id PROJECT_ID --config-id CONFIG_ID
bin/langsmith --format json insights list --project-id PROJECT_ID --config-id CONFIG_ID --limit 20 --offset 0
bin/langsmith --format json insights get JOB_ID --project-id PROJECT_ID
bin/langsmith --format json insights runs JOB_ID --project-id PROJECT_ID --cluster-id CLUSTER_ID --limit 20
```

`--config-id` runs the saved configuration exactly as stored and rejects analysis overrides.
Its dry-run shows the ID but cannot resolve the saved settings with the current SDK.
Configuration authoring and scheduling remain in the UI.

Report listing defaults to **20 reports** (previously unbounded), with a maximum page size
of 100. The JSON list remains an array. Advance `--offset` by the number returned;
a full page does not guarantee another page, and an empty page ends the listing.

`insights runs` returns `project_id`, `workspace_id`, `job_id`, `cluster_id`, `runs`,
`io_mode: "preview"`, `sort_scope: "page"`, and `pagination` with `next_offset`.
Use that offset to continue. Run records include available metadata, summaries, and
extracted attributes; full IO requires `run get --full`. Source retention or
missing runs can reduce evidence coverage. `--sort-by ATTRIBUTE --sort-order asc|desc`
sorts only the current page. No report cancellation or automatic retry of creation is offered.


Start a one-off analysis with an explicit provider, sample count, and time window:

```bash
langsmith insights create --project-id <uuid> --last-n-hours 24 --sample 20 --model openai \
  --user-context '{"Business goal":"Resolve eligible refund requests","Concern":"Claims of success after a tool error"}'
langsmith insights list --project-id <uuid> --limit 5
langsmith insights get <job-id> --project-id <uuid>
```

In an interactive terminal with pretty output, omit `--model` to choose OpenAI or
Anthropic at a prompt. There is no default choice. Non-interactive and JSON usage
requires `--model` and never prompts. The menu lists supported providers, not verified
workspace availability; credential checks still happen on the service.

Alternatively use `--start-time` and optional `--end-time` as RFC3339 timestamps.
Do not combine `--start-time` with `--last-n-hours`. Optional `--filter`, `--name`,
and `--summary-prompt` refine the report. `--user-context` is a JSON object of
question-to-answer strings, with at least one non-empty answer.

Creation can incur workspace model usage. `--model` selects `openai` or `anthropic`
using server-side configuration, not your app's local Gateway client. `--sample`
is a positive trace count, not a percentage or dollar cap; service limits apply.
No recurrence is scheduled, and model-secret validation is not bypassed.

The JSON response preserves the service's job ID, name, status, error, and project
ID. A queued response does not mean analysis is complete. Creation is not retried
automatically; if a response is lost, inspect existing jobs before submitting again.
Advanced saved configurations and separate cluster/summary model overrides remain
available through the API, not this command's initial flag surface.

### `hub` — Manage agent and skill repos on the LangSmith Hub

The hub stores versioned directories of files grouped into repos of type `agent` or `skill`. Each push creates a new commit; pull downloads a commit's files into a local directory. This is the CLI surface for the `langsmith` Python/JS SDK's hub methods (`pull_skill`, `push_skill`, `pull_agent`, `push_agent`, etc.).

```bash
# Scaffold a starter skill (or agent)
langsmith hub init --type skill --dir ./my-skill --name my-skill

# Push a local directory as a new commit (creates the repo if missing)
langsmith hub push my-skill --type skill --dir ./my-skill

# Pull a commit (latest by default; pin a tag with :ref)
langsmith hub pull my-skill --dir ./out
langsmith hub pull acme/my-skill:production --dir ./out

# Discover, inspect, delete
langsmith hub list --type skill --query foo
langsmith hub list --type skill --source external
langsmith hub get acme/my-skill
langsmith hub delete acme/my-skill --yes
```

Identifiers use `[OWNER/]REPO` format. Omitting owner defaults to `-` (the API's "current tenant" wildcard).

Push excludes `.git/`, `node_modules/`, `__pycache__/`, `.venv/`, `dist/`, `build/`, `target/`, `.next/`, `.cache/`, plus `.env*` files, common secret extensions (`.pem`, `.key`, `.pfx`, `.p12`, `.crt`), and rejects binary or oversize (>1 MiB) files. Pull wipes the destination dir before writing; non-empty directories without a `SKILL.md`/`AGENTS.md` marker require `--yes`.

### `self-update` — Update langsmith to the latest version

```bash
# Check if an update is available
langsmith self-update --dry-run

# Update to the latest version
langsmith self-update
```

If langsmith was installed through a package manager, `self-update` won't replace the
binary in place — it points you at the right command instead:

| Installed via | Update with |
| --- | --- |
| Homebrew | `brew upgrade langchain-ai/tap/langsmith-cli` |
| Scoop | `scoop update langsmith-cli` |
| `go install` | `go install github.com/langchain-ai/langsmith-cli/cmd/langsmith@latest` |

Installs from the `install.sh`/`install.ps1` scripts or a direct GitHub Releases download
are updated in place as usual. Pass `--force` to update in place regardless of how
langsmith was installed.

### `trace setup` — Trace coding agents to LangSmith

Configure Claude Code or Codex to send full-content traces (prompts, responses, tool
calls) to a LangSmith project, by writing the agent's local config files. Requires an
API key — it is written to the agent config at `0600` (OAuth profiles are not supported
here). Each command previews the exact changes and asks you to confirm (pass `--yes` to skip the prompt), then installs the plugin via the agent's own CLI.

```bash
# Bare: try both Claude Code and Codex (best-effort; an uninstalled agent just fails)
langsmith trace setup

# Configure Claude Code: API key, URL, and project as positional args (bare host gains https://)
langsmith trace setup claude demo-key dev.smith.com shared-claude

# Or take the key + URL from env/profile
langsmith trace setup claude

# Configure Codex (writes ~/.codex/config.toml + ~/.codex/langsmith.json)
langsmith trace setup codex

# Trace to a named project (default: "claude-code" / "codex", or $LANGSMITH_PROJECT)
langsmith trace setup claude --project my-agent

# Override the auto-detected name/email attached to every trace
langsmith trace setup claude --user "Jane Doe" --email jane@example.com

# Pass everything explicitly (self-hosted or a specific workspace key)
langsmith trace setup claude demo-key https://my-host/api/v1 my-team   # all positional

# Apply without the interactive confirmation prompt
langsmith trace setup claude --yes

# Write config only; skip running the plugin install
langsmith trace setup claude --no-install

# Write project-local config instead of user-global
langsmith trace setup claude --scope project    # ./.claude/settings.local.json
langsmith trace setup codex --scope project     # ./.codex/...
```

`trace setup claude` installs the plugin via `claude plugin marketplace add` + `claude plugin install`;
`trace setup codex` fetches it via `codex plugin marketplace add`. Once enabled, the plugin runs on
every session and sends your prompts, responses, and tool output to LangSmith. Your name and
email (auto-detected from `git config user.name`/`user.email`, or set via `--user`/`--email`)
are attached to every trace as `user_name`/`user_email` metadata. Verify Claude Code with
`tail -f ~/.claude/state/hook.log`.

## Filter Options

Most `trace` and `run` commands share these filter options:

| Flag | Description | Example |
|------|-------------|---------|
| `--project` | Project name | `--project my-app` |
| `--limit, -n` | Max results | `-n 10` |
| `--last-n-minutes` | Time window (overrides 7-day default) | `--last-n-minutes 60` |
| `--since` | After ISO timestamp (overrides 7-day default) | `--since 2024-01-15T00:00:00Z` |
| `--error / --no-error` | Error status | `--error` |
| `--name` | Name search (case-insensitive) | `--name ChatOpenAI` |
| `--run-type` | Run type (run commands only) | `--run-type llm` |
| `--min-latency` | Min latency (seconds) | `--min-latency 2.5` |
| `--max-latency` | Max latency (seconds) | `--max-latency 10` |
| `--min-tokens` | Min total tokens | `--min-tokens 1000` |
| `--tags` | Tags (comma-separated, OR logic) | `--tags prod,v2` |
| `--filter` | Raw LangSmith filter DSL | `--filter 'eq(status, "error")'` |
| `--trace-ids` | Specific trace IDs | `--trace-ids abc123,def456` |

## Added workflows and manual testing

This checkout contains the combined reference implementation for [PR #308](https://github.com/langchain-ai/langsmith-cli/pull/308).
Review and land the smaller capability PRs linked there instead. The commands below
describe this checkout, not the published release.

| Added commands | What they provide |
| --- | --- |
| `project create` | Create an empty tracing project. |
| `dataset add` | Preview and import root traces, child runs, or thread turns with frozen selections and retry reconciliation. |
| `example update` | Replace inputs/reference outputs while preserving omitted fields. |
| `feedback create`, `feedback list` | Write/read feedback, including stable-ID retry reconciliation. |
| `queue create`, `queue list`, `queue items`, `queue add`, `queue delete` | Manage annotation queues and reviewed additions. |
| `rule list` | Inspect project-scoped automation rules. |
| `insights create`, `insights runs` | Configure bounded reports and drill into category evidence; inspect status with `insights get`. |

These are 13 new commands. Existing dataset creation also returns a creation status.
No SDK/API changes are included. Experiment comparison, experiment execution, rule
mutations, and Engine configuration are not included in this implementation.

### 1. Build and select a safe test workspace

Use Bash or zsh, Go matching `go.mod`, `jq`, and an existing authenticated profile.
Run from the repository root, in the same shell throughout. Replace all uppercase
placeholder values before continuing. Never paste API keys into the commands.

```bash
make build
LS_BIN="$PWD/bin/langsmith"
LS_PROFILE='YOUR_SAVED_PROFILE'
LS_WORKSPACE='YOUR_WORKSPACE_UUID'
LS_SOURCE_PROJECT='YOUR_SYNTHETIC_TRACING_PROJECT_UUID'
LS_TRACE='YOUR_EXISTING_ROOT_TRACE_UUID'
LS_TEST_DIR="$(mktemp -d)"
LS_TEST_TAG="cli-manual-$(date +%Y%m%d-%H%M%S)-$$"

lscheck() {
  env -u LANGSMITH_API_KEY -u LANGCHAIN_API_KEY \
    "$LS_BIN" --profile "$LS_PROFILE" \
    --workspace "$LS_WORKSPACE" --format json "$@"
}

lscheck --version
lscheck trace get "$LS_TRACE" --project-id "$LS_SOURCE_PROJECT"
lscheck rule list --project-id "$LS_SOURCE_PROJECT"
```

Expected: verified project read access, `trace_received` for the explicit trace,
and project-scoped rules (possibly empty). Doctor does not verify write or inference
permissions. Use a project with synthetic data you are authorized to copy.

### 2. Create disposable resources — writes

```bash
lscheck project create --name "$LS_TEST_TAG" | tee "$LS_TEST_DIR/project.json"
LS_TEST_PROJECT="$(jq -er '.id' "$LS_TEST_DIR/project.json")"
lscheck dataset create --name "$LS_TEST_TAG" | tee "$LS_TEST_DIR/dataset.json"
LS_TEST_DATASET="$(jq -er '.id' "$LS_TEST_DIR/dataset.json")"
lscheck queue create --name "$LS_TEST_TAG" \
  --dataset "${LS_TEST_DATASET:?Dataset creation must succeed}" | tee "$LS_TEST_DIR/queue.json"
LS_TEST_QUEUE="$(jq -er '.id' "$LS_TEST_DIR/queue.json")"
```

Expected: each creation returns `status: created` and an ID. Stop if any command
fails; do not blindly repeat creation after an ambiguous network failure. The new
project is empty; subsequent imports read from the existing source project.

### 3. Preview, review, import, and retry a trace

```bash
lscheck dataset add --project-id "$LS_SOURCE_PROJECT" \
  --dataset "${LS_TEST_DATASET:?}" --trace-id "$LS_TRACE" \
  --dry-run --output "$LS_TEST_DIR/selection.json"
jq . "$LS_TEST_DIR/selection.json"
```

Review the saved inputs before applying. The preview writes a private file and emits
no stdout with `--output`. Default references are inputs-only: observed agent
answers are not silently accepted as ground truth.

```bash
lscheck dataset add --project-id "$LS_SOURCE_PROJECT" \
  --dataset "${LS_TEST_DATASET:?}" --selection "$LS_TEST_DIR/selection.json" \
  | tee "$LS_TEST_DIR/import.json"
LS_EXAMPLE="$(jq -er '.results[0].example_id' "$LS_TEST_DIR/import.json")"

lscheck dataset add --project-id "$LS_SOURCE_PROJECT" \
  --dataset "${LS_TEST_DATASET:?}" --selection "$LS_TEST_DIR/selection.json"
lscheck example list --dataset "${LS_TEST_DATASET:?}" --limit 5
```

Expected: first apply `created`, retry `skipped`, same example ID, `failed: 0`.

Optional selection variants, requiring valid source fixtures:

```bash
LS_CHILD_RUN='YOUR_CHILD_RUN_UUID'
LS_THREAD='YOUR_THREAD_ID'
lscheck dataset add --project-id "$LS_SOURCE_PROJECT" --dataset "${LS_TEST_DATASET:?}" \
  --run-id "$LS_CHILD_RUN" --dry-run
lscheck dataset add --project-id "$LS_SOURCE_PROJECT" --dataset "${LS_TEST_DATASET:?}" \
  --thread-id "$LS_THREAD" --limit 5 --dry-run
lscheck dataset add --project-id "$LS_SOURCE_PROJECT" --dataset "${LS_TEST_DATASET:?}" \
  --no-error --last-n-minutes 1440 --limit 5 --dry-run
```

Thread imports produce separate root-turn examples, not a merged conversation.
An empty recent selection is valid. Use `--error` or `--no-error` for execution
error filtering; raw `eq(error, true)` and `eq(error, false)` are rejected by the API.

### 4. Edit a reference and check conflict protection — writes

Use an appropriate reference object for your fixture instead of this illustrative value:

```bash
lscheck example update "${LS_EXAMPLE:?}" --outputs '{"answer":"Reviewed reference"}'
lscheck example list --dataset "${LS_TEST_DATASET:?}" --limit 5
lscheck dataset add --project-id "$LS_SOURCE_PROJECT" \
  --dataset "${LS_TEST_DATASET:?}" --selection "$LS_TEST_DIR/selection.json"
```

Expected: reference output changes while inputs remain intact. Reapplying the old
selection now exits nonzero and reports conflicting content without overwriting the edit.

### 5. Create and retry feedback — writes on the source trace

This adds test feedback to the selected source trace. Use a disposable synthetic trace.

```bash
lscheck feedback create --project-id "$LS_SOURCE_PROJECT" --run-id "$LS_TRACE" \
  --key "$LS_TEST_TAG" --score 0 --comment 'Manual CLI test' \
  | tee "$LS_TEST_DIR/feedback.json"
LS_FEEDBACK="$(jq -er '.feedback_id' "$LS_TEST_DIR/feedback.json")"
lscheck feedback create --project-id "$LS_SOURCE_PROJECT" --run-id "$LS_TRACE" \
  --key "$LS_TEST_TAG" --score 0 --comment 'Manual CLI test' --id "${LS_FEEDBACK:?}"
lscheck feedback list --run-id "$LS_TRACE" --limit 5 --offset 0
```

Expected: verified creation, followed by `skipped`; score remains numeric zero.
Keep the returned ID if a write is unverified and reconcile with that ID and payload.

### 6. Review queue additions, apply, and inspect — writes on apply

```bash
lscheck queue list --limit 5
lscheck queue add "${LS_TEST_QUEUE:?}" --project-id "$LS_SOURCE_PROJECT" \
  --trace-id "$LS_TRACE" --dry-run --output "$LS_TEST_DIR/queue-plan.json"
jq . "$LS_TEST_DIR/queue-plan.json"
```

After reviewing the exact source IDs:

```bash
lscheck queue add "${LS_TEST_QUEUE:?}" --project-id "$LS_SOURCE_PROJECT" \
  --plan "$LS_TEST_DIR/queue-plan.json"
lscheck queue items "${LS_TEST_QUEUE:?}" --limit 5
```

Expected: `submitted`, `failed: 0`, and the source trace visible in queue items.
For optional child/thread tests, replace `--trace-id` in a new plan with
`--run-id "$LS_CHILD_RUN"` or `--thread-id "$LS_THREAD"`. Queue thread additions
represent whole threads, unlike dataset turn imports. Filter additions select roots
and fail if they exceed the limit. Do not blindly replay queue writes: re-adding an
item can reopen its review. Queue item pagination uses `--cursor`.

### 7. Generate an Insights report — may incur model cost

Choose the business question and a time window containing your synthetic traces.
This starts one report, not a recurring job.

```bash
lscheck insights create --project-id "$LS_SOURCE_PROJECT" --model openai \
  --sample 5 --last-n-hours 168 --name "$LS_TEST_TAG" \
  --user-context '{"Goal":"Find failures in synthetic test traces"}' \
  | tee "$LS_TEST_DIR/insights.json"
LS_INSIGHT="$(jq -er '.id' "$LS_TEST_DIR/insights.json")"
lscheck insights get "${LS_INSIGHT:?}" --project-id "$LS_SOURCE_PROJECT"
```

Expected: creation returns a durable job ID and status. Use insights get with the same ID to inspect progress and retrieve the report when complete. Provider credentials are configured server-side.

### 8. Check JSON errors and delete the disposable queue

```bash
lscheck feedback list --run-id invalid \
  > "$LS_TEST_DIR/stdout.txt" 2> "$LS_TEST_DIR/stderr.json"
# Expected nonzero exit; stdout empty, stderr one JSON diagnostic.
wc -c "$LS_TEST_DIR/stdout.txt"
jq . "$LS_TEST_DIR/stderr.json"

lscheck queue delete "${LS_TEST_QUEUE:?}" --yes
lscheck queue items "${LS_TEST_QUEUE:?}" --limit 1
```

Expected: deletion reports `deleted`; the subsequent lookup fails with 404. Only
delete the scratch queue created above. The scratch project/dataset, source feedback,
Insights report, and local files remain for inspection. Remove them separately after
review; selection files can contain sensitive trace data.

Known gaps: existing output-file collisions are misclassified as network errors in
JSON mode, and some diagnostics omit useful recovery details. This checklist is not
a substitute for the feature PRs' unit tests or deployment/permission testing.

### Agent-facing errors

Feedback and queue lists include `pagination.returned`, `pagination.has_more`,
and `pagination.next_offset`. For a full offset page, `has_more` is null (unknown);
request `next_offset` to find out. A short page has `has_more: false`. Queue-item
lists use the server cursor and include `has_more` and `returned` directly.
These read results include `workspace_id`, or null when it was not resolved.

With `--format json`, errors returned to the executable produce a JSON diagnostic
on stderr and exit nonzero. Stdout remains reserved for results (including any
partial-write report). Diagnostics include `error.code`, `error.message`,
`error.next_steps`, and an HTTP status when available. Typed API errors distinguish
authentication, permissions, missing resources, conflicts, invalid requests, and
rate limits. Unclassified failures use `command_failed`; raw upstream messages
are omitted to avoid exposing credentials or trace data. Do not assume a failed
write did not happen: inspect partial results and resource state before retrying.
Legacy command paths that exit directly are not yet covered by this renderer.
Local resource validation uses `invalid_resource_id` and `invalid_json_object`;
an unavailable named profile uses `profile_not_found`. Each includes specific
next steps without echoing the rejected value. Other validation paths still use
the generic fallback until migrated.

### Remaining proposal work

- `rule create` / `rule delete` are outside the current CLI-only scope: the pinned generated Go SDK exposes rule listing,
  but not these mutations. Existing backend routes need public OpenAPI/Stainless
  coverage and a generated SDK release; no raw HTTP workaround is added here.
- `engine setup` / `engine configure` are outside the current CLI-only scope: they require public SDK exposure of the existing
  issues-agent routes. Atomic creation with a budget or paused state additionally
  requires backend support; setup must not enable spending before limits exist.
- Typed validation errors and legacy direct-exit migration, workspace envelopes on
  every response, and universal retry reconciliation remain separate work. This checkout does not yet satisfy
  the full proposed LLM-oriented contract.

Live end-to-end verification remains necessary before release, particularly for
workspace permissions, queue review states, and partial network failures.

## Local Development

For local dev, create a wrapper script at `~/.local/bin/langsmith` that loads your `.env` and uses `go run`:

```bash
cat > ~/.local/bin/langsmith << 'EOF'
#!/usr/bin/env bash
set -euo pipefail
cd /path/to/langsmith-cli
set -a && source .env && set +a
exec go run ./cmd/langsmith "$@"
EOF
chmod +x ~/.local/bin/langsmith
```

Ensure `~/.local/bin` is in your `PATH` before `~/go/bin`. This way commands like `langsmith sandbox list` and SSH ProxyCommand entries work without manually sourcing `.env` each time.

### Requirements

- Go 1.23+
- golangci-lint (for linting)

## Releasing

Releases are tag-driven. Pushing a `v*` tag runs [`.github/workflows/release.yml`](.github/workflows/release.yml),
which invokes GoReleaser to cross-compile linux/darwin/windows on amd64+arm64, publish the
archives and `checksums.txt`, and cut the GitHub Release with a changelog generated from the
commits since the previous tag (`docs:`, `test:`, and `ci:` commits are excluded).

```bash
git checkout main && git pull
git tag v0.2.44          # next patch after the latest tag
git push origin v0.2.44
```

There is no version file or changelog to edit — the version is stamped into the binary from the
tag via ldflags, so `git tag` is the only bump. Find the latest tag with `git tag --sort=-v:refname | head -1`.

The install scripts and `langsmith self-update` both read the latest GitHub Release, so a tag push
is all that's needed to ship to users.

## License

MIT
