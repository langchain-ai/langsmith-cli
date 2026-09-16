# langsmith-cli

### Client-side safety boundaries

- Model selection copies an LC v1 serialized configuration, not a live link.
  Evaluator results label this `binding: snapshot`. Nested constructor/secret nodes
  are structurally validated with bounded depth; OAuth, unknown serialized node
  types and malformed references are rejected. This does not validate provider
  compatibility or inference access. Secret references remain service-resolved.
- Idle-time updates re-read project `extra` immediately before sending an update
  and abort if it changed. This narrows but does not eliminate the race: there is
  no server-side compare-and-swap. Other field edits do not rewrite `extra`.
- Queue responses distinguish matching acknowledgements from unverified outcomes;
  they do not claim independent read-back or completed annotation. Batches are not
  atomic, and the legacy `failed` count includes uncertain outcomes. Bulk example
  updates expose `verification: acknowledged_not_read_back` on acknowledged writes.
  Inspect uncertain items before retrying; never equate a failed request with proof
  that no write occurred.

### Project settings

```bash
langsmith project configure --project-id PROJECT_ID --format json
langsmith project configure --project-id PROJECT_ID \
  --description 'Support agent' --default-dataset DATASET_ID --dry-run
# After review, rerun with --apply instead of --dry-run.
```

Supported edits: `--name`, `--description` (empty string clears text),
`--default-dataset` (name or UUID), `--trace-tier shortlived|longlived`, and
`--thread-idle-seconds` (at least 120). Only supplied fields are updated.
Reads show stored `settings`; null values do not claim effective defaults.
Previews show `previous_settings`, proposed `settings`, `changes`, and warnings.
Apply performs one update without automatic retries and verifies requested values
by reading them back. An unverified outcome requires inspection before retrying.

Renaming can affect applications tracing by name. Default dataset selection does
not import traces. Trace tier changes can affect retention/billing; the CLI does
not verify retroactive retention changes. Idle time is shared by all project thread
evaluators. Concurrent edits can race idle-time changes because they preserve and
rewrite existing `extra`. Previews are not frozen plans. Clearing default datasets,
resetting inherited tiers, arbitrary `extra`, and end timestamps are not exposed.

### Agent-facing guidance

Successful empty reads are not errors. Model, queue, feedback, dataset version/split,
and Insights evidence envelopes include `message` and `next_steps` when appropriate.
An empty page is not proof that a workspace has no resources; check filters and
pagination. Authentication failures still return errors, never synthetic empty lists.

Project creation explains that application tracing must be configured separately.
Example updates acknowledge the update; bulk and queue results explain per-item
verification and uncertain-write recovery. Dataset imports remind users to curate
reference outputs. Configuration and evaluator previews explain how to apply them
and what the preview cannot validate. Insights creation directs callers to inspect
job status and evidence, without treating submission as completed analysis.

JSON stays machine-readable and noninteractive. Existing result keys, array-shaped
read responses, and frozen selection/plan files retain their shapes. Guidance is
additive on result objects; Insights list remains an array (empty-state guidance is
shown only in pretty mode). No backend APIs or write behavior were changed.

### Saved workspace models

```bash
langsmith --format json model list
langsmith --format json model get CONFIG_UUID
langsmith evaluator create-llm --name policy-compliance --project my-project \
  --prompt prompt.json --schema schema.json --model-id CONFIG_UUID --dry-run
# Remove --dry-run after reviewing the intended evaluator settings.
```

Use `--profile` and `--workspace` to select the account and workspace. Model summaries
include IDs, names, identifiable model names, availability flags, and
`inference_verified: false`, without raw settings, credentials, or headers.
These are saved configurations, not a provider catalog. Missing fields remain null.

List returns `{workspace_id, models: [...]}`; get returns `{workspace_id, model: {...}}`.
Use the configuration's `id`, not its provider model name, with `--model-id`.
Preview and creation results include the selected safe `model` summary; file-based
model configurations return null for that summary.

`--model-id` and `--model-config` are mutually exclusive. Model selection copies
the current model settings into the evaluator; later preset edits do not update it.
The preset must explicitly allow evaluators. OAuth presets and unsupported model
serialization are rejected. Creation and dry-run do not verify credentials or
inference access. Existing JSON model files remain supported. No API or SDK changes.
The hidden `model preset list/get` and `--model-preset` spellings remain compatible
with earlier scripts, including their original JSON envelope keys.

See [MANUAL-TESTING.md](MANUAL-TESTING.md) for a bounded walkthrough using synthetic
traces and disposable resources.

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

### `project` — List and delete tracing projects

A tracing project (session) is a namespace that groups related traces together. This lists only tracing projects, not experiments — use `experiment list` for those.

Results are **paginated** — by default, only the first **20** projects are returned (use `--limit` to change). Projects are sorted by **most recent activity** (`last_run_start_time`, descending).

```bash
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

### Create a tracing project

```bash
langsmith project create --name my-app --description 'Application traces'
```

Use `--format json` for `status`, `id`, `name`, and `workspace_id`. Pretty output
shows a creation confirmation and IDs. Workspace identity comes from the service
response, falling back to the selected workspace; unknown identity stays null.
Creating a project does not instrument your app or enable evaluators.

The create request is not automatically retried. If the response is lost, inspect
the selected workspace before retrying:

```bash
langsmith project list --name-contains my-app --format json
```

Compare exact names and workspace scope; this is a substring search, not proof
that a matching project exists or that a single page contains every match.

### Agent-facing errors

With `--format json`, returned errors are emitted as JSON on stderr with a nonzero exit. Stdout is reserved for results. Diagnostics include safe codes, messages, and next steps; raw upstream messages are omitted. Legacy commands that exit directly are not covered. Always inspect remote state before retrying writes.

### Update example inputs or reference outputs

```bash
langsmith example update <example-id> --outputs '{"answer":"reviewed reference"}'
```

Accepts JSON objects or `@file`. Omitted fields are preserved. Writes are not automatically retried.

### Run feedback

```bash
langsmith run feedback create --project-id <project-id> --run-id <run-id> --key quality --score 0 --id <feedback-id>
langsmith run feedback list --run-id <run-id> --limit 20
langsmith run feedback get <feedback-id> --format json
langsmith run feedback list --run-id <run-id> --key quality --has-score --format json
langsmith run feedback list --run-id <run-id> --source app --has-comment \
  --start-time 2026-09-01T00:00:00Z --end-time 2026-09-02T00:00:00Z
```

An explicit feedback ID enables retry reconciliation: identical writes are skipped and conflicting content is refused. List results expose offset pagination; full-page completeness is unknown until the next page.

Use `--format json` for coding agents and scripts. Creation returns resource IDs,
the feedback record, and `status`: `created` (verified), `skipped` (identical existing
ID), `recovered` (read-back verified after a failed write response), or `unverified`
(unknown outcome with a nonzero exit). For an unverified write, inspect the emitted
ID with `run feedback get` and retry with that same `--id`, not a new ID.
Permission and validation failures include targeted recovery guidance.

Pretty output shows write status or feedback tables; missing scores display as
`N/A`, not zero. JSON get output includes `workspace_id`, `project_id`, `run_id`,
`feedback_id`, and the full `feedback` record. List output preserves its `items`
and pagination metadata.

List pages accept `--limit` from 1 to 100, matching the API. Comment-only creation
requires a nonblank comment; a score of zero is still valid. Terminal errors show
recovery guidance as well as the error message when a command diagnostic is available.

List filters run on the service: repeat `--key` and `--source` or use comma-separated
values. Sources are `api`, `app`, `model`, and `auto_eval`. Omitted presence filters
include both cases; `--has-score=false` and `--has-comment=false` explicitly select
missing values. Time bounds apply to feedback creation, not the source run.

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

Preview always emits JSON containing the actual example payloads. `--output` creates
a new private file and refuses to overwrite; protect it as trace data. Filter-based
selection defaults to 20 roots in the last seven days; use time bounds and `--limit`
explicitly for your sample. This is not representative sampling.

New previews include `workspace_id` when resolved and `selection_info` with `limit`,
`selected`, `has_more`, and `scope`. Filter/thread discovery probes one extra eligible
root to detect truncation; only the selected examples are saved for import. Completeness
describes discovery time, not a snapshot guarantee against later changes. A thread
becomes separate root-turn examples, not a merged conversation. Older selection files
remain accepted but lack completeness metadata.

Selection files reject unknown envelope/example fields, duplicate JSON keys (including
within inputs and outputs), and excessive nesting. Large JSON integers are preserved.
When the plan records a workspace, replay requires that same selected workspace.

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

Import JSON includes `workspace_id`, `project_id`, `dataset_id`, `total`, and `counts`
for `created`, `skipped`, `recovered`, and `failed`. Existing `results` and `failed`
fields remain available. The added preview context does not change example IDs.

Annotation-queue promotion and representative sampling are not included in these commands.


Use `dataset add --dry-run --output selection.json` to preview roots, child runs (`--run-id`), or thread turns (`--thread-id`); apply the frozen payload with `dataset add --selection selection.json`. Dataset creation returns `status: created` and disables automatic retries.

### Annotation queues

```bash
langsmith queue create --name review --dataset <dataset-id>
langsmith queue list --limit 20
langsmith queue add <queue-id> --project-id <project-id> --trace-id <trace-id> --dry-run --output plan.json
langsmith queue add <queue-id> --project-id <project-id> --plan plan.json
langsmith queue items <queue-id> --limit 20
langsmith queue delete <queue-id> --yes
```

Additions support root traces, individual runs, and threads. Filter selections operate on roots and must fit the limit. Plans freeze source IDs and scope. Queue items use cursor pagination and default to pending review. Deletion requires explicit confirmation. Re-adding an item can reopen its review; submissions are not automatically retried.

### `insights` — Create and inspect Insights reports

`insights create` creates a **report run**, not a saved Insight/dashboard card.
To run an existing dashboard Insight, pass its saved configuration ID with
`--config-id`. An unlinked one-off job can finish successfully without appearing
as a new card on the Insights dashboard. Saved-configuration authoring and
scheduling are not first-class commands in this build; those operations currently
require the UI or the generic `api` command.

Attributes belong to the analysis configuration and are supported on both inline
reports and saved-config runs. A numeric attribute's description can request a
1–10 scale, but this is not an enforced minimum/maximum constraint. Inspect actual
evidence values and missingness before treating inferred satisfaction as a measured
customer rating. Report narrative highlights can cover fewer examples than the
evidence list; do not treat their percentages as whole-project rates.

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
  "summary_prompt": "Summarize the request, action and outcome. Inputs: {{run.inputs}} Outputs: {{run.outputs}}"
}
```

Save this as `analysis.json`, replace `PROJECT_ID`, and use your configured profile/workspace:

```bash
# Validate the request without creating a paid job.
bin/langsmith --format json insights create --project-id PROJECT_ID --file analysis.json --dry-run
# After reviewing the request, submit it.
bin/langsmith --format json insights create --project-id PROJECT_ID --file analysis.json
```

Custom summary prompts summarize each run, not just the final report. Use simple
variables such as `{{run.inputs}}`, `{{run.outputs}}`, nested object paths, or
`{{all_thread_messages}}`; sections and helpers are not supported. Omit the prompt
to use the service default. Category names must be unique after trimming, and
attribute names cannot contain whitespace.

To check variables without starting analysis:

```bash
bin/langsmith --format json insights create --project-id PROJECT_ID --file analysis.json --dry-run --preview-run RUN_ID
```

The preview returns `bindings`, `missing_paths`, `unchecked_paths`, and
`paths_validated`. It preserves null, false, zero, and large integer values. Thread
messages, feedback, and fields unavailable in the SDK query are marked unchecked;
nested paths traverse objects, not array indexes. This is
not a rendered summary, a matching-run count, or proof the explicit run meets the
analysis filter/time window or will be sampled. It reads trace data into output.

`--sample` accepts 1–1000. The service selects the latest matching root per thread
plus unthreaded roots; fewer eligible traces can mean a smaller report.

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

## Shared command behavior

JSON command errors are written to stderr with a stable code, safe message, and
recovery steps. Typed diagnostics also show recovery steps in terminal output.
Shared resource helpers validate UUIDs and JSON objects; paginated workflows
preserve unknown completeness instead of claiming that a full page is the last.

Manual error check (expected nonzero exit):

```bash
bin/langsmith --format json unknown-command
```

## Online judge settings

`evaluator create-llm` supports these explicit rule settings using the existing
run-rule API (no SDK upgrade required):

| Flag | Meaning |
| --- | --- |
| `--group-by thread_id` / `--group-by none` | Whole-thread / individual-run evaluation |
| `--filter`, `--trace-filter`, `--tree-filter` | Existing service filter expressions; these select data, not prompt variables |
| `--sampling-rate 0.1` | Sample 10% of eligible evaluation units; not an exact count or cost ceiling |
| `--enabled=false` | Create or replace a paused evaluator |
| `--extend-trace-retention=false` | Explicit project evaluator retention override |
| `--trace-evaluator-runs=false` | Disable tracing of judge execution |
| `--include-extended-stats` | Include feedback/cost/token stats; incompatible with thread mode |
| `--backfill-from TIMESTAMP` | Explicit historical evaluation, potentially billable; server permissions/feature gates apply |
| `--spend-limit 1` | Weekly USD rule limit; service enforcement applies, not a per-request hard cap |
| `--dry-run` | Validate local inputs and preview requested settings without saving or inference |

```bash
langsmith evaluator create-llm --name conversation-quality --project-id PROJECT_ID \
  --prompt judge-prompt.json --schema judge-schema.json --model-config model.json \
  --group-by thread_id --sampling-rate 0.1 --enabled=false --spend-limit 1 --dry-run
```

Remove `--dry-run` to save. With `--enabled=false`, saving does not activate the
judge. Inspect saved settings using `evaluator get --session-id PROJECT_ID`.
The preview excludes prompt/model configuration to avoid leaking credentials.
It shows requested settings, not resolved defaults, and does not validate model
credentials, filter semantics, inherited settings, or permission to backfill.
It is not an inference test or frozen apply plan.

`--replace` updates a matching rule. Omitted advanced settings, sampling rate and
enabled state are preserved. Explicit empty filter values clear those filters;
`--group-by none` clears thread grouping. JSON replacement never prompts and
requires explicit `--yes` approval. Switching modes can require explicitly disabling
incompatible existing settings. Judge prompts must consume the appropriate thread
content; grouping alone does not rewrite a run-oriented prompt.

Thread idle time is project-wide, not per judge. Creating the first thread rule
may initialize the service default when no project idle time exists. Inspect or
explicitly change it separately:

```bash
langsmith project configure --project-id PROJECT_ID --format json
langsmith project configure --project-id PROJECT_ID --thread-idle-seconds 600 --dry-run
langsmith project configure --project-id PROJECT_ID --thread-idle-seconds 600 --apply
```

The CLI requires at least 120 seconds; deployment-specific server limits may differ.
Updates preserve other project extra settings and verify the idle time by reading it
back, but concurrent project configuration edits can race the read-modify-write.
Missing idle time is returned as null rather than an invented effective default.

UI inference testing, application associations, and resolved organization spend
defaults are not exposed by these additions.

## Dataset versions, splits, and reviewed edits

These commands use the existing Go SDK and API. Dataset versions are created by
the service when examples change; tags name snapshots, while splits select subsets.
Use explicit `--profile` and `--workspace` settings for your environment.

```bash
# Read-only: inspect history and resolve a version tag.
langsmith dataset version list --dataset DATASET_ID --limit 20 --format json
langsmith dataset version get --dataset DATASET_ID --as-of prod --format json
langsmith dataset version diff --dataset DATASET_ID --from prod --to latest --format json
langsmith dataset split list --dataset DATASET_ID --as-of prod --format json

# Read examples from a snapshot; split names are arbitrary, not a fixed enum.
langsmith example list --dataset DATASET_ID --as-of prod \
  --split test --split refunds --metadata '{"reviewed":true}' --limit 20 --format json
langsmith example list --dataset DATASET_ID --filter 'exists(metadata,"source")' \
  --search refund --limit 20 --format json
langsmith dataset export DATASET_ID ./examples.json --as-of prod --split test --limit 100

# Writes: replace one example's memberships, or clear them explicitly.
langsmith example update EXAMPLE_ID --split test --split refunds --format json
langsmith example update EXAMPLE_ID --clear-splits --format json
```

Version diffs return added/modified/removed example IDs, not full before/after IO.
Use historical example reads to inspect content. List/export remain bounded by
`--limit`; these are not automatically complete dataset snapshots. Use `--offset`
to page example lists. Export retains its existing inputs/outputs-only file shape.
Example list includes `dataset_id` and `modified_at` for preparing reviewed edits.

### Multimodal attachments: Go SDK limitation

LangSmith supports dataset attachments, including PDFs, images, and audio, through
the existing multipart examples API. However, the CLI's pinned Go SDK (`v0.26.2`)
does not expose the multipart methods needed to upload local attachment files.
This is an SDK coverage gap, not a missing platform API. Local attachment upload
is therefore not supported by these CLI commands yet; a JSON file path does not
upload the referenced file.

Follow-up: expose the existing multipart API in the generated Go SDK, then add
CLI attachment upload and safe update support. For now, use the UI or documented
Python/TypeScript SDK workflows in the
[multimodal evaluation guide](https://docs.langchain.com/langsmith/evaluate-with-attachments).

### Bulk edits

Prepare an array in `edits.json`, using IDs and exact `modified_at` timestamps from
the **latest** example list:

```json
[
  {
    "id": "22222222-2222-4222-8222-222222222222",
    "expected_modified_at": "2026-09-16T00:00:00Z",
    "outputs": {"answer": "Reviewed reference answer"},
    "splits": ["test", "refunds"]
  }
]
```

```bash
langsmith example update-bulk --dataset DATASET_ID --file edits.json --dry-run --format json
langsmith example update-bulk --dataset DATASET_ID --file edits.json --apply --format json
```

The same file supplies the exact edit IDs and payloads in both commands; discovery
is not rerun. Files are limited to 100 edits and 8 MiB. Every example's dataset and
timestamp are checked before any writes. Updates are sequential, **not atomic**:
another writer can still change an example after preflight. Each result is
`updated` (API acknowledged) or `unverified`; unverified outcomes exit nonzero.
There are no automatic write retries. Read back affected examples before retrying,
and prepare a new file with current timestamps. An unchanged replay normally fails
the stale-timestamp check rather than writing again.

Omitted fields are unchanged. Inputs, outputs, and metadata are replacement
objects, not key-level patches. **Metadata includes `dataset_split`: preserve that
key or explicitly supply split memberships when replacing metadata.** Use `splits:
[]` to clear memberships. Keep local edit files private; they contain evaluation IO.

### Tag a reviewed version

```bash
# Resolve the current timestamp, inspect a diff, then use that fixed timestamp.
langsmith dataset version get --dataset DATASET_ID --format json
langsmith dataset version tag --dataset DATASET_ID --as-of TIMESTAMP --tag prod --dry-run --format json
langsmith dataset version tag --dataset DATASET_ID --as-of TIMESTAMP --tag prod --format json
```

Tagging can move an existing tag. Prefer a fixed timestamp over `latest` when
applying a reviewed change. No separate version-creation step is needed.
Omitting `--as-of` from `version get` resolves `latest`. Version timestamps retain
subsecond precision. A tag write returns `updated` only after reading the tag back
and matching the resolved snapshot; a failed or mismatched read-back returns
`dataset_write_unverified`. Inspect the tag before retrying because the write may
have applied.

### Configure schemas and transformations

Read the current configuration with `dataset get DATASET_ID`. Put only the fields
you intend to replace in `dataset-config.json`:

```json
{
  "inputs_schema_definition": {
    "type": "object",
    "properties": {"question": {"type": "string"}},
    "required": ["question"]
  },
  "outputs_schema_definition": {"type": "object"},
  "transformations": []
}
```

```bash
langsmith dataset configure --dataset DATASET_ID --file dataset-config.json --dry-run --format json
langsmith dataset configure --dataset DATASET_ID --file dataset-config.json --apply --format json
```

Omitted settings are unchanged. `{}` allows an unrestricted schema; `[]` clears
transformations. Transformation entries use API fields `path` and
`transformation_type`; see the [transformation reference](https://docs.langchain.com/langsmith/dataset-transformations).
Dry-run checks local structure and reads the dataset, but does not validate server
permissions or the full JSON schema. Apply invokes the service's validation.

## License

MIT
