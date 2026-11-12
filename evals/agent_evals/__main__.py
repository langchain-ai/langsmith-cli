"""Entry point: `uv run python -m agent_evals run` (see README.md)."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import shutil
import subprocess
import sys
import uuid
from concurrent.futures import ThreadPoolExecutor
from dataclasses import asdict
from pathlib import Path
from typing import Any

from . import scoring, seed, surface
from .agent import AgentConfig, run_task
from .tasks import TASKS, by_id

DATASET = "langsmith-cli-agent-evals"
HERE = Path(__file__).resolve().parent.parent


def cli_version(binary: str) -> str:
    out = subprocess.run(
        [binary, "--version"], capture_output=True, text=True, timeout=30, check=False
    ).stdout
    # "langsmith version v0.9.0 (commit: …)" -> "v0.9.0"
    parts = out.split()
    return parts[2] if len(parts) > 2 else out.strip() or "unknown"


def ensure_dataset(client) -> dict[str, str]:
    """Keep the task-set dataset in sync with TASKS; return task id -> example id."""
    if client.has_dataset(dataset_name=DATASET):
        ds = client.read_dataset(dataset_name=DATASET)
    else:
        ds = client.create_dataset(
            DATASET,
            description="Fixed tasks an agent runs through the LangSmith CLI on every release (cli/evals).",
        )
    existing = {
        e.inputs.get("task_id"): e for e in client.list_examples(dataset_id=ds.id)
    }
    ids = {}
    for t in TASKS:
        inputs = {"task_id": t.id, "title": t.title, "kind": t.kind, "prompt": t.prompt}
        ex = existing.get(t.id)
        if ex is None:
            ex = client.create_example(dataset_id=ds.id, inputs=inputs)
        elif ex.inputs != inputs:
            client.update_example(ex.id, inputs=inputs)
        ids[t.id] = str(ex.id)
    return ids


def already_ran(client, meta: dict[str, Any]) -> bool:
    """Whether this CLI version, model and surface already have a finished experiment.

    Only experiments marked complete count, so a run that crashed partway is retried.
    """
    keys = ("cli_version", "model", "surface")
    found = client.list_projects(
        reference_dataset_name=DATASET,
        metadata={**{k: meta[k] for k in keys}, "complete": True},
        limit=1,
    )
    return any(True for _ in found)


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(prog="agent_evals", description=__doc__)
    sub = ap.add_subparsers(dest="cmd", required=True)

    run = sub.add_parser(
        "run", help="seed a workspace, run the tasks, score them, and log an experiment"
    )
    run.add_argument(
        "--binary",
        default=os.environ.get("LANGSMITH_CLI", "langsmith"),
        help="CLI under test (default: langsmith on PATH)",
    )
    run.add_argument(
        "--model",
        action="append",
        help="model under test; repeat for several (default: claude-opus-5-5)",
    )
    run.add_argument(
        "--effort", default="medium", choices=["low", "medium", "high", "xhigh", "max"]
    )
    run.add_argument("--tasks", help="comma-separated task ids (default: all)")
    run.add_argument("--repetitions", type=int, default=1)
    run.add_argument(
        "--concurrency",
        type=int,
        default=5,
        help="tasks run at once within a repetition",
    )
    run.add_argument("--max-turns", type=int, default=40)
    run.add_argument(
        "--surface",
        choices=["cli", "api"],
        default="cli",
        help="cli: the full CLI; api: only `langsmith api` (raw REST), used to record the baseline",
    )
    run.add_argument(
        "--skills-dir",
        help="directory of SKILL.md files the agent can read (e.g. a langsmith-skills checkout)",
    )
    run.add_argument(
        "--mcp-url",
        help="MCP server the agent can use alongside the CLI (token from EVAL_MCP_TOKEN)",
    )
    run.add_argument(
        "--baseline",
        default=str(HERE / "baseline.json"),
        help="baseline JSON for the token criterion",
    )
    run.add_argument(
        "--write-baseline", help="write this run's summary as a baseline file"
    )
    run.add_argument(
        "--no-upload", action="store_true", help="do not log an experiment to LangSmith"
    )
    run.add_argument(
        "--skip-if-done",
        action="store_true",
        help="exit if this CLI version, model and surface already have an experiment",
    )
    run.add_argument(
        "--enforce",
        action="store_true",
        help="exit 1 when a model misses the launch criteria",
    )
    run.add_argument(
        "--keep", action="store_true", help="keep the seeded workspace for debugging"
    )
    run.add_argument("--out", default=str(HERE / "results"))

    sub.add_parser("list", help="print the task set")
    surf = sub.add_parser(
        "check",
        help="check a command line against the CLI's help, e.g. check -- runs list --bogus",
    )
    surf.add_argument("--binary", default=os.environ.get("LANGSMITH_CLI", "langsmith"))
    surf.add_argument("args", nargs=argparse.REMAINDER)
    clean = sub.add_parser("cleanup", help="delete everything a run seeded or created")
    clean.add_argument("prefix", help="e.g. cli-eval-1a2b3c4d")

    args = ap.parse_args(argv)
    if args.cmd == "list":
        for t in TASKS:
            print(f"{t.id:24} {t.kind:7} {t.title}")
        return 0
    if args.cmd == "check":
        tree = surface.crawl(args.binary)
        argv_ = args.args[1:] if args.args[:1] == ["--"] else args.args
        print(
            json.dumps(
                asdict(surface.check_call(tree, argv_))
                | {"made_up": surface.check_call(tree, argv_).made_up}
            )
        )
        return 0

    from langsmith import Client

    client = Client()
    if args.cmd == "cleanup":
        print("\n".join(seed.cleanup(client, args.prefix)))
        return 0
    return run_evals(client, args)


def run_evals(client, args) -> int:
    import anthropic

    binary = shutil.which(args.binary) or args.binary
    models = args.model or ["claude-opus-5-5"]
    tasks = by_id(args.tasks.split(",") if args.tasks else None)
    version = cli_version(binary)
    tree = surface.crawl(binary)
    run_id = uuid.uuid4().hex[:8]
    out_dir = Path(args.out) / f"{dt.datetime.now(dt.UTC):%Y%m%dT%H%M%S}-{run_id}"
    out_dir.mkdir(parents=True, exist_ok=True)
    surface.save(tree, str(out_dir / "cli_surface.json"))
    upload = not args.no_upload
    common = {
        "cli_version": version,
        "surface": args.surface
        + ("+skills" if args.skills_dir else "")
        + ("+mcp" if args.mcp_url else ""),
        "effort": args.effort,
        "repetitions": args.repetitions,
        "git_sha": os.environ.get("GITHUB_SHA", ""),
        "run_id": run_id,
    }
    print(
        f"CLI {version} ({len(tree)} commands), models {', '.join(models)}, {len(tasks)} tasks × {args.repetitions}",
        flush=True,
    )

    anthropic_client = anthropic.Anthropic()
    if upload:
        from langsmith.wrappers import wrap_anthropic

        anthropic_client = wrap_anthropic(anthropic_client)
        example_ids = ensure_dataset(client)

    rows: list[dict[str, Any]] = []
    for mi, model in enumerate(models):
        meta = {**common, "model": model}
        if args.skip_if_done and upload and already_ran(client, meta):
            print(
                f"{model}: an experiment for {version} already exists, skipping",
                flush=True,
            )
            continue
        experiment = None
        if upload:
            ds = client.read_dataset(dataset_name=DATASET)
            name = f"cli-{version}-{model}-{meta['surface']}-{run_id}"
            experiment = client.create_project(
                name,
                reference_dataset_id=ds.id,
                metadata=meta,
                description=f"LangSmith CLI {version} driven by {model}",
            )
            print(f"{model}: experiment {name}", flush=True)
        cfg = AgentConfig(
            model=model,
            binary=binary,
            tree=tree,
            effort=args.effort,
            max_turns=args.max_turns,
            api_only=args.surface == "api",
            skills_dir=args.skills_dir,
            mcp_url=args.mcp_url,
            mcp_token=os.environ.get("EVAL_MCP_TOKEN"),
        )
        for rep in range(1, args.repetitions + 1):
            # A fresh workspace per repetition, so a state task never sees an earlier attempt's writes.
            seed_id = f"{run_id}-m{mi}r{rep}"
            prefix = seed.prefix_for(seed_id)
            try:
                s = seed.seed(client, seed_id)
                print(f"{model} repeat {rep}: seeded {s.prefix}", flush=True)
                with ThreadPoolExecutor(max_workers=args.concurrency) as pool:
                    futures = [
                        pool.submit(
                            attempt,
                            client,
                            anthropic_client,
                            cfg,
                            t,
                            s,
                            rep,
                            experiment,
                            example_ids[t.id] if upload else None,
                        )
                        for t in tasks
                    ]
                    for f in futures:
                        row = f.result()
                        rows.append(row)
                        mark = "PASS" if row["success"] else "FAIL"
                        print(
                            f"  {mark} {row['task_id']:24} tokens={row['total_tokens']:>7,} calls={row['cli_calls']:>2} "
                            f"made_up={row['made_up_calls']} {'' if row['success'] else row['check']}",
                            flush=True,
                        )
            finally:
                if not args.keep:
                    try:
                        seed.cleanup(client, prefix)
                    except Exception as e:  # noqa: BLE001 - leftovers are named by prefix; report, keep going
                        print(
                            f"cleanup of {prefix} failed: {e}; rerun `agent_evals cleanup {prefix}`",
                            flush=True,
                        )
        if experiment is not None:
            client.update_project(
                experiment.id,
                end_time=dt.datetime.now(dt.UTC),
                # Only a run of the whole task set stands in for a release; a
                # hand-picked subset must not make the schedule skip it.
                metadata={**meta, "complete": not args.tasks},
            )

    baseline = None
    if args.baseline and Path(args.baseline).exists() and args.surface != "api":
        baseline = json.loads(Path(args.baseline).read_text())
    summary = scoring.summarize(rows, total_tasks=len(TASKS), baseline=baseline)
    report = scoring.markdown(summary, common, rows)
    (out_dir / "attempts.json").write_text(json.dumps(rows, indent=1, default=str))
    (out_dir / "summary.json").write_text(json.dumps(summary, indent=1))
    (out_dir / "summary.md").write_text(report)
    if args.write_baseline:
        Path(args.write_baseline).write_text(scoring.baseline_from(summary, common))
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as f:
            f.write(report)
    print(report)
    print(f"Results: {out_dir}")
    if args.enforce and not all(s["launch"]["met"] for s in summary.values()):
        return 1
    return 0


def attempt(
    client,
    anthropic_client,
    cfg: AgentConfig,
    task,
    s: seed.Seed,
    rep: int,
    experiment,
    example_id,
) -> dict[str, Any]:
    prompt = task.render(s)

    def go():
        tr = run_task(
            anthropic_client, cfg, task.id, prompt, scope=[s.prefix, *s.known_ids]
        )
        try:
            chk = task.check(client, s, tr.final_text)
        except Exception as e:  # noqa: BLE001 - a crashing check fails the task, not the run
            chk = type("C", (), {"passed": False, "detail": f"check error: {e}"})()
        return tr, chk

    if experiment is None:
        tr, chk = go()
        root_id = None
    else:
        import langsmith as ls

        with (
            ls.tracing_context(enabled=True),
            ls.trace(
                task.id,
                "chain",
                inputs={"task_id": task.id, "prompt": prompt},
                project_name=experiment.name,
                reference_example_id=example_id,
                client=client,
                metadata={"repetition": rep, "model": cfg.model},
            ) as run,
        ):
            tr, chk = go()
            run.end(
                outputs={
                    "final_text": tr.final_text,
                    "check": chk.detail,
                    "commands": [" ".join(c.argv) for c in tr.calls],
                },
                error=tr.error,
            )
            root_id = run.id
    row = {
        "task_id": task.id,
        "model": cfg.model,
        "repetition": rep,
        "check": chk.detail,
        "error": tr.error,
        "final_text": tr.final_text,
        "calls": [asdict(c) for c in tr.calls],
        **scoring.metrics(tr, chk.passed),
    }
    if root_id is not None:
        for key in (
            "success",
            "made_up_calls",
            "failed_calls",
            "cli_calls",
            "help_calls",
            "total_tokens",
            "input_tokens",
            "output_tokens",
        ):
            client.create_feedback(
                root_id,
                key=key,
                score=row[key],
                trace_id=root_id,
                comment=chk.detail if key == "success" else None,
                project_id=experiment.id,
            )
    return row


if __name__ == "__main__":
    sys.exit(main())
