# langsmith

A coding agent-first CLI for querying and managing [LangSmith](https://smith.langchain.com) resources.

Built for AI coding agents (Claude Code, Cursor, etc.) and developers who need fast, scriptable access to projects, traces, runs, datasets, evaluators, experiments, and threads. All commands output JSON by default for easy parsing.

## Installation

```bash
# With uv (recommended)
uv tool install langsmith-tools

# With pipx
pipx install langsmith-tools

# With pip
pip install langsmith-tools

# From source
uv tool install git+https://github.com/langchain-ai/langsmith
```

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

All commands default to **JSON** output for machine consumption:

```bash
langsmith trace list --project my-app  # JSON array to stdout
```

Use `--format pretty` for human-readable Rich tables and trees:

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

```bash
# List tracing projects (default limit: 20)
langsmith project list
langsmith project list --limit 50

# Filter by name
langsmith project list --name-contains chatbot

# Human-readable table
langsmith --format pretty project list
```

### `trace` — Query and export traces

A trace is a tree of runs representing one end-to-end invocation of your application.

```bash
# List recent traces (default limit: 20)
langsmith trace list --project my-app
langsmith trace list --project my-app --limit 50 --last-n-minutes 60

# Filter traces
langsmith trace list --project my-app --error           # Only errors
langsmith trace list --project my-app --min-latency 5   # Slow traces (>5s)
langsmith trace list --project my-app --tags production  # By tag
langsmith trace list --project my-app --name "agent"     # By name

# Include additional fields
langsmith trace list --project my-app --include-metadata  # + status, duration, tokens, costs
langsmith trace list --project my-app --include-io        # + inputs, outputs, error
langsmith trace list --project my-app --full              # All fields

# Show trace hierarchy (fetches full run tree for each trace)
langsmith trace list --project my-app --show-hierarchy --limit 3

# Get a specific trace
langsmith trace get <trace-id> --project my-app --full

# Export traces to JSONL files (one per trace)
langsmith trace export ./traces --project my-app --limit 20 --full
```

### `run` — Query individual runs

A run is a single step within a trace (LLM call, tool call, chain step, etc.).

```bash
# List LLM calls (default limit: 50)
langsmith run list --project my-app --run-type llm
langsmith run list --project my-app --run-type tool --name search

# Find expensive calls
langsmith run list --project my-app --run-type llm --min-tokens 1000 --include-metadata

# Get a specific run
langsmith run get <run-id> --full

# Export to JSONL (default limit: 100)
langsmith run export llm_calls.jsonl --project my-app --run-type llm --full
```

### `thread` — Query conversation threads

A thread groups multiple root runs sharing a thread_id (multi-turn conversations).

```bash
# List threads (requires --project)
langsmith thread list --project my-chatbot
langsmith thread list --project my-chatbot --last-n-minutes 120

# Get all turns in a thread
langsmith thread get <thread-id> --project my-chatbot --full
```

### `dataset` — Manage evaluation datasets

```bash
# List datasets
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

# Inspect local files without uploading
langsmith dataset view-file data.json
langsmith dataset structure data.json
```

#### Dataset Generation from Traces

Generate evaluation datasets from exported trace files:

```bash
# Step 1: Export traces
langsmith trace export ./traces --project my-app --full --limit 50

# Step 2: Generate dataset
langsmith dataset generate -i ./traces -o eval.json --type final_response

# Dataset types:
#   final_response  - Root input -> root output pairs
#   single_step     - Individual node I/O (use --run-name to target specific nodes)
#   trajectory      - Input -> tool call sequence
#   rag             - Question -> retrieved chunks -> answer

# Generate and upload to LangSmith in one step
langsmith dataset generate -i ./traces -o eval.json --type rag --upload my-rag-eval

# Advanced options
langsmith dataset generate -i ./traces -o eval.json --type single_step --run-name ChatOpenAI
langsmith dataset generate -i ./traces -o eval.json --type trajectory --depth 2
langsmith dataset generate -i ./traces -o eval.json --type final_response --input-fields query --output-fields answer
```

### `example` — Manage dataset examples

```bash
# List examples
langsmith example list --dataset my-dataset
langsmith example list --dataset my-dataset --split test --limit 50

# Create an example
langsmith example create --dataset my-dataset \
  --inputs '{"question": "What is LangSmith?"}' \
  --outputs '{"answer": "A platform for LLM observability"}'

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

# Replace an existing evaluator
langsmith evaluator upload evals.py \
  --name accuracy --function check_accuracy_v2 --dataset my-eval-set --replace --yes

# Delete an evaluator
langsmith evaluator delete accuracy --yes
```

### `experiment` — Query experiment results

```bash
# List experiments
langsmith experiment list
langsmith experiment list --dataset my-eval-set

# Get experiment results (feedback stats, run stats)
langsmith experiment get my-experiment-2024-01-15
```

## Filter Options

Most `trace` and `run` commands share these filter options:

| Flag | Description | Example |
|------|-------------|---------|
| `--project` | Project name | `--project my-app` |
| `--limit, -n` | Max results | `-n 10` |
| `--last-n-minutes` | Time window | `--last-n-minutes 60` |
| `--since` | After ISO timestamp | `--since 2024-01-15T00:00:00Z` |
| `--error / --no-error` | Error status | `--error` |
| `--name` | Name search (case-insensitive) | `--name ChatOpenAI` |
| `--run-type` | Run type (run commands only) | `--run-type llm` |
| `--min-latency` | Min latency (seconds) | `--min-latency 2.5` |
| `--max-latency` | Max latency (seconds) | `--max-latency 10` |
| `--min-tokens` | Min total tokens | `--min-tokens 1000` |
| `--tags` | Tags (comma-separated, OR logic) | `--tags prod,v2` |
| `--filter` | Raw LangSmith filter DSL | `--filter 'eq(status, "error")'` |
| `--trace-ids` | Specific trace IDs | `--trace-ids abc123,def456` |

## JSON Output Schemas

All commands produce predictable JSON:

- **List commands**: `[{...}, {...}, ...]` (array)
- **Get commands**: `{...}` (single object)
- **Mutating commands**: `{"status": "created|deleted|uploaded", ...}`
- **Errors**: `{"error": "message"}` (written to stderr)

### Run fields

Base fields (always included):

```json
{"run_id", "trace_id", "name", "run_type", "parent_run_id", "start_time", "end_time"}
```

With `--include-metadata`:

```json
{"status", "duration_ms", "custom_metadata", "token_usage", "costs", "tags"}
```

With `--include-io`:

```json
{"inputs", "outputs", "error"}
```

## Usage with AI Agents

This CLI is designed to be called by AI coding agents. Key design decisions:

1. **JSON by default** — all output is valid JSON, parseable without regex
2. **Predictable schemas** — list commands return arrays, get commands return objects
3. **`--yes` flags** — all destructive operations have `--yes` to skip interactive prompts
4. **Errors to stderr** — error JSON goes to stderr so stdout is always clean data
5. **Env var auth** — no interactive login flows; set `LANGSMITH_API_KEY` and go

Example agent workflow:

```bash
# Agent investigates a production issue
langsmith trace list --project prod-app --error --last-n-minutes 30 --full

# Agent examines a specific failing trace
langsmith trace get <trace-id> --project prod-app --full

# Agent checks the LLM call that failed
langsmith run get <run-id> --full
```

## Development

```bash
git clone https://github.com/langchain-ai/langsmith
cd langsmith

# Install with dev dependencies
uv sync --extra dev

# Run tests
uv run pytest tests/ -v

# Lint
uv run ruff check src/ tests/
```

## License

MIT
