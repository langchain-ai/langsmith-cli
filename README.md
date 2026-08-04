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

Or save the credentials in a profile, so the key is not passed on the command
line — process arguments are visible to other users on the machine, so prefer
`auth login` or the environment variable over `--api-key`:

```bash
langsmith auth login                    # OAuth
langsmith profile create prod           # or an API key, taken from $LANGSMITH_API_KEY
```

Other settings can be passed as flags:

```bash
langsmith --workspace <workspace-id> trace list --project my-app
```

The config file (`~/.langsmith/config.json`) holds these credentials and is
written owner-only; the CLI warns if it is readable by other users.

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

## Command Reference

### `project` — List tracing projects

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
