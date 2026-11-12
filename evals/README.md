# CLI agent evals

A fixed set of 15 tasks that an agent (Claude) does through the `langsmith` CLI on every release. Each run is scored for task success, made-up commands or flags, and tokens, and logged to LangSmith as an experiment so releases can be compared. This is the success metric for the CLI roadmap (ADX-28).

## Run it

You need `ANTHROPIC_API_KEY`, `LANGSMITH_API_KEY` (and `LANGSMITH_ENDPOINT` if you are not on US prod), Go and uv. From `cli/`:

```bash
make agent-evals ARGS="--model claude-haiku-5-5 --tasks what-broke,add-example --no-upload"
```

That builds this checkout's CLI and runs it. Leave out `ARGS` to run all 15 tasks with `claude-opus-5-5` and log an experiment. To evaluate an installed release instead, run `uv run python -m agent_evals run --binary "$(which langsmith)"` from this directory. Other options are in `uv run python -m agent_evals run --help`; `make test-agent-evals` runs the offline tests.

Each run seeds a fresh set of resources named `cli-eval-<run id>-…` (a tracing project with traces, errors, feedback and a thread; datasets; two experiments), runs the tasks against them, and deletes them afterwards. If a run is killed, remove what it left with `uv run python -m agent_evals cleanup cli-eval-<run id>-m0r1`.

## What it measures

| Metric | How |
|---|---|
| Task success | Answer tasks end with a line of JSON compared with facts recorded at seed time. State tasks are checked by reading the workspace afterwards, never from the agent's claim. |
| Made-up commands or flags | Every call is resolved against a command tree crawled from the binary's own `--help`, plus `unknown command/flag` errors. A group given a word that isn't a subcommand counts, even when the CLI prints help and exits 0. |
| Failed calls, help lookups, CLI calls, turns | From the call log. |
| Tokens | Input (including cache reads and writes) plus output, summed over the task. Cost is estimated from list prices. |
| Unsafe writes | Creates, updates and deletes must target only the run's prefix, its seeded IDs, or IDs the agent created in that task (output flags like `--jq` don't count). Others are refused before they reach the API and counted; this should stay 0. |

The agent runs `langsmith` with an argument list and can write files into an empty per-attempt directory, which the CLI runs in, for flags that take a file (such as `evaluators create-llm --model-config`). There is no shell, so it can't pipe or fall back to curl. Every path a call names, through a file flag (classified from the CLI's own help), an `@file` value or a file argument, must resolve inside that directory. Flags that pick credentials or run commands (`--api-key-cmd`, `--profile`, …) and commands that act on the runner (`auth`, `self-update`, `sandboxes`, …) are refused. `--skills-dir` adds the SKILL.md files in a directory (for example a langsmith-skills checkout) through a `read_skill` tool, and `--mcp-url` adds an MCP server through the Messages API MCP connector. MCP tool calls run server-side, so the write guard does not cover them.

## Launch criteria

v1.0 needs at least 13 of 15 tasks passing in every repeat, and tokens per task at least 40% below the baseline. The baseline is the same task set done with only raw `langsmith api` calls (`--surface api`), which is how agents used LangSmith before the generated commands. Record it once and commit it as `baseline.json`:

```bash
uv run python -m agent_evals run --surface api --repetitions 3 \
  --model claude-haiku-5-5 --model claude-opus-5-5 --write-baseline baseline.json
```

Token savings are compared per model, so the baseline needs every model you score: Haiku for the scheduled runs, Opus for dispatched ones. A baseline dispatched from CI (`surface: api`) records both by default.

Until `baseline.json` exists the token criterion reports "no baseline". `--enforce` makes the run exit 1 when a model misses either criterion.

## In CI

`.github/workflows/cli_agent_evals.yml` runs daily on `main`: it installs the latest CLI release and skips versions that already have a finished experiment (a crashed run is retried the next day), so each release is evaluated once with Haiku. Opus runs only when dispatched (dispatch defaults to Opus and Haiku), for example before a launch. Dispatch it also to evaluate a specific tag, this checkout (`source`), or the baseline. It reads the dev LangSmith key and the dev-vault Anthropic key the SDK nightly tests use (set the `CLI_AGENT_EVALS_WORKSPACE_ID` repository variable to run in a dedicated workspace), and never runs on pull requests; PRs that touch this directory run only the offline tests.

Results are in the `langsmith-cli-agent-evals` dataset in LangSmith (one experiment per CLI version and model, with experiment metadata `cli_version`, `model`, `surface`), the job summary, and the `cli-agent-evals` artifact.

## Adding a task

Add a `Task` to `agent_evals/tasks.py` with a prompt over the seeded names and a check. If it needs new data, seed it in `agent_evals/seed.py` and record the fact the check compares against on `Seed`. Keep checks deterministic: compare with seeded facts or read state back through the SDK.
