# langsmith-cli

> **Alpha** — This CLI is under active development. Commands, flags, and output schemas may change between releases. Feedback and bug reports welcome via [GitHub Issues](https://github.com/langchain-ai/langsmith-cli/issues).

An agent-first CLI for querying and managing [LangSmith](https://smith.langchain.com) resources.

Built for AI coding agents (deepagents, Claude Code, Cursor, etc.) and developers who need fast, scriptable access to projects, traces, runs, datasets, evaluators, experiments, and threads.

## Installation

### Install script (recommended)

```bash
curl -fsSL https://cli.langsmith.com/install.sh | sh
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
export LANGSMITH_PROJECT="my-default-project"                 # Default project for queries
```

Or pass them as flags:

```bash
langsmith --api-key lsv2_pt_... trace list --project my-app
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

All commands default to **JSON** output for agent consumption:

```bash
langsmith trace list --project my-app  # JSON array to stdout
```

Use `--format pretty` for human-readable tables and trees:

```bash
langsmith --format pretty trace list --project my-app
```

Write to a file with `-o`:

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

# Human-readable table
langsmith --format pretty project list
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

### `self-update` — Update langsmith to the latest version

```bash
# Check if an update is available
langsmith self-update --dry-run

# Update to the latest version
langsmith self-update
```

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

### Requirements

- Go 1.23+
- golangci-lint (for linting)

## License

MIT
