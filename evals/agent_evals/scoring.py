"""Turn transcripts and checks into per-task metrics and the release scorecard."""

from __future__ import annotations

import json
import math
from collections import defaultdict
from dataclasses import dataclass
from typing import Any

from .agent import Transcript

# v1.0 launch criteria (ADX-28): at least 13 of 15 tasks pass, and tokens per task
# are at least 40% below the baseline.
LAUNCH_MIN_PASS_RATE = 13 / 15
LAUNCH_MIN_TOKEN_REDUCTION = 0.40

# USD per million tokens (input, output, cache read), for the cost estimate only.
PRICES = {
    "claude-fable-5-2": (8.0, 40.0, 0.20),
    "claude-opus-5-5": (4.0, 20.0, 0.20),
    "claude-sonnet-5-5": (2.0, 10.0, 0.10),
    "claude-haiku-5-5": (0.10, 0.50, 0.01),
}


def metrics(tr: Transcript, passed: bool) -> dict[str, Any]:
    """Per-attempt metrics. Help lookups and made-up calls are counted separately from failures."""
    calls = tr.calls
    price = PRICES.get(tr.model)
    cost = None
    if price:
        uncached = tr.input_tokens - tr.cache_read_tokens
        cost = round(
            (
                uncached * price[0]
                + tr.output_tokens * price[1]
                + tr.cache_read_tokens * price[2]
            )
            / 1e6,
            4,
        )
    return {
        "success": int(passed),
        "cli_calls": len(calls),
        "failed_calls": sum(1 for c in calls if c.exit != 0 and not c.blocked),
        "made_up_calls": sum(1 for c in calls if c.made_up),
        "help_calls": sum(1 for c in calls if c.is_help),
        "blocked_calls": sum(1 for c in calls if c.blocked and not c.unsafe_write),
        "unsafe_writes": sum(1 for c in calls if c.unsafe_write),
        "mcp_calls": tr.mcp_calls,
        "skill_reads": tr.skill_reads,
        "file_writes": tr.file_writes,
        "turns": tr.turns,
        "input_tokens": tr.input_tokens,
        "output_tokens": tr.output_tokens,
        "total_tokens": tr.input_tokens + tr.output_tokens,
        "cost_usd": cost,
        "wall_s": tr.wall_s,
        "harness_error": int(bool(tr.error)),
    }


@dataclass
class TaskSummary:
    task_id: str
    attempts: int
    passes: int
    mean_tokens: float
    made_up_calls: int
    failed_calls: int
    mean_cli_calls: float

    @property
    def passed_every_repeat(self) -> bool:
        return self.attempts > 0 and self.passes == self.attempts


def summarize(
    rows: list[dict[str, Any]], total_tasks: int, baseline: dict[str, Any] | None = None
) -> dict[str, Any]:
    """Aggregate attempt rows ({task_id, model, metrics...}) per model."""
    by_model: dict[str, list[dict]] = defaultdict(list)
    for r in rows:
        by_model[r["model"]].append(r)
    out: dict[str, Any] = {}
    for model, mrows in sorted(by_model.items()):
        per_task: dict[str, list[dict]] = defaultdict(list)
        for r in mrows:
            per_task[r["task_id"]].append(r)
        tasks = {
            tid: TaskSummary(
                task_id=tid,
                attempts=len(rs),
                passes=sum(r["success"] for r in rs),
                mean_tokens=sum(r["total_tokens"] for r in rs) / len(rs),
                made_up_calls=sum(r["made_up_calls"] for r in rs),
                failed_calls=sum(r["failed_calls"] for r in rs),
                mean_cli_calls=sum(r["cli_calls"] for r in rs) / len(rs),
            )
            for tid, rs in sorted(per_task.items())
        }
        passed_once = sum(1 for t in tasks.values() if t.passes > 0)
        passed_every = sum(1 for t in tasks.values() if t.passed_every_repeat)
        mean_tokens = (
            sum(t.mean_tokens for t in tasks.values()) / len(tasks) if tasks else 0.0
        )
        model_out: dict[str, Any] = {
            "tasks": len(tasks),
            "tasks_passed": passed_once,
            "tasks_passed_every_repeat": passed_every,
            "mean_tokens_per_task": round(mean_tokens),
            "made_up_calls": sum(t.made_up_calls for t in tasks.values()),
            "unsafe_writes": sum(r["unsafe_writes"] for r in mrows),
            "failed_calls": sum(t.failed_calls for t in tasks.values()),
            "cost_usd": round(sum(r["cost_usd"] or 0 for r in mrows), 2),
            "per_task": {
                tid: vars(t) | {"passed_every_repeat": t.passed_every_repeat}
                for tid, t in tasks.items()
            },
        }
        model_out["token_reduction_vs_baseline"] = token_reduction(
            tasks, baseline, model
        )
        model_out["launch"] = launch_check(
            passed_every,
            total_tasks,
            len(tasks),
            model_out["token_reduction_vs_baseline"],
        )
        out[model] = model_out
    return out


def token_reduction(
    tasks: dict[str, TaskSummary], baseline: dict[str, Any] | None, model: str
) -> float | None:
    """1 - (mean tokens per task / baseline mean), over the tasks both runs have."""
    if not baseline:
        return None
    # Only the same model's baseline is comparable: models differ in tokens per task.
    base = baseline.get("models", {}).get(model)
    if not base:
        return None
    base_tokens = {tid: t["mean_tokens"] for tid, t in base["per_task"].items()}
    shared = [tid for tid in tasks if tid in base_tokens and base_tokens[tid] > 0]
    if not shared:
        return None
    now = sum(tasks[t].mean_tokens for t in shared) / len(shared)
    before = sum(base_tokens[t] for t in shared) / len(shared)
    return round(1 - now / before, 3)


def launch_check(
    passed_every: int, total_tasks: int, ran: int, reduction: float | None
) -> dict[str, Any]:
    """Launch criteria. A task counts as passed only if it passed in every repeat."""
    need = math.ceil(LAUNCH_MIN_PASS_RATE * total_tasks)
    pass_ok = ran == total_tasks and passed_every >= need
    tokens_ok = reduction is not None and reduction >= LAUNCH_MIN_TOKEN_REDUCTION
    return {
        "tasks": f"{passed_every}/{total_tasks} (need {need})",
        "tasks_ok": pass_ok,
        "tokens": "no baseline"
        if reduction is None
        else f"{reduction:.0%} below baseline (need {LAUNCH_MIN_TOKEN_REDUCTION:.0%})",
        "tokens_ok": tokens_ok,
        "met": pass_ok and tokens_ok,
    }


def markdown(
    summary: dict[str, Any], meta: dict[str, Any], rows: list[dict[str, Any]]
) -> str:
    lines = [f"## CLI agent evals: {meta.get('cli_version', '?')}", ""]
    lines.append(
        " · ".join(
            f"{k}: `{v}`"
            for k, v in meta.items()
            if k != "cli_version" and v not in (None, "")
        )
    )
    for model, s in summary.items():
        launch = s["launch"]
        lines += [
            "",
            f"### {model}",
            "",
            f"- Tasks passed every repeat: **{launch['tasks']}** {'✅' if launch['tasks_ok'] else '❌'}",
            f"- Tokens per task: **{s['mean_tokens_per_task']:,}**, {launch['tokens']} {'✅' if launch['tokens_ok'] else '❌'}",
            f"- Made-up commands or flags: **{s['made_up_calls']}**, failed calls: {s['failed_calls']}, refused out-of-scope writes: {s['unsafe_writes']}, cost ≈ ${s['cost_usd']}",
            "",
            "| Task | Passed | Tokens | CLI calls | Made up | Failed |",
            "|---|---|---|---|---|---|",
        ]
        for tid, t in s["per_task"].items():
            lines.append(
                f"| {tid} | {t['passes']}/{t['attempts']} | {round(t['mean_tokens']):,} | "
                f"{t['mean_cli_calls']:.1f} | {t['made_up_calls']} | {t['failed_calls']} |"
            )
    fails = [r for r in rows if not r["success"]]
    if fails:
        lines += ["", "<details><summary>Failed attempts</summary>", ""]
        lines += [
            f"- `{r['task_id']}` ({r['model']}, repeat {r['repetition']}): {r['check']}"
            + (f" — harness: {r['error']}" if r.get("error") else "")
            for r in fails
        ]
        lines += ["", "</details>"]
    made_up = [(r["task_id"], c) for r in rows for c in r["calls"] if c["made_up"]]
    if made_up:
        lines += ["", "<details><summary>Made-up commands and flags</summary>", ""]
        lines += [
            f"- `{tid}`: `langsmith {' '.join(c['argv'])[:160]}` ({', '.join(c['unknown']) or c['stderr'][:80]})"
            for tid, c in made_up
        ]
        lines += ["", "</details>"]
    return "\n".join(lines) + "\n"


def baseline_from(summary: dict[str, Any], meta: dict[str, Any]) -> str:
    return (
        json.dumps({"meta": meta, "models": summary}, indent=1, sort_keys=True) + "\n"
    )
