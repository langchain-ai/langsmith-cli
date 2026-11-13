"""The fixed task set.

Each task is a prompt over the seeded workspace plus a deterministic check. Answer
tasks end with one line of JSON that the check compares with facts recorded at seed
time; state tasks are checked by reading the workspace afterwards, never by trusting
what the agent says it did.
"""

from __future__ import annotations

import json
import re
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

from langsmith import Client

from .seed import Seed


@dataclass
class Check:
    passed: bool
    detail: str


@dataclass
class Task:
    id: str
    title: str
    prompt: str
    check: Callable[[Client, Seed, str], Check]
    kind: str  # "answer" (read-only) or "state" (writes)

    def render(self, seed: Seed) -> str:
        return self.prompt.format(**seed.names()).strip()


def final_json(text: str) -> dict[str, Any] | None:
    """The last top-level JSON object in the agent's final message."""
    decoder = json.JSONDecoder()
    best: tuple[int, int, dict] | None = None
    for m in re.finditer(r"\{", text):
        try:
            value, end = decoder.raw_decode(text, m.start())
        except ValueError:
            continue
        # Prefer the object that ends last; among those, the outermost one.
        if isinstance(value, dict) and (
            best is None or end > best[0] or (end == best[0] and m.start() < best[1])
        ):
            best = (end, m.start(), value)
    return best[2] if best else None


def _answer(
    fn: Callable[[dict[str, Any], Seed], Check],
) -> Callable[[Client, Seed, str], Check]:
    def check(_client: Client, seed: Seed, text: str) -> Check:
        data = final_json(text)
        if data is None:
            return Check(False, "no JSON answer in the final message")
        try:
            return fn(data, seed)
        except (KeyError, TypeError, ValueError) as e:
            return Check(False, f"answer {data!r} is malformed: {e}")

    return check


def _num(v: Any) -> float:
    return float(str(v).strip())


def _ok(cond: bool, got: Any, want: Any) -> Check:
    return Check(bool(cond), f"got {got!r}, want {want!r}")


# ---- state helpers -------------------------------------------------------------


def _rules(client: Client, project_id: str) -> list[dict]:
    resp = client.request_with_retries(
        "GET", "/runs/rules", request_kwargs={"params": {"session_id": project_id}}
    )
    return resp.json()


def _dataset(client: Client, name: str):
    return (
        client.read_dataset(dataset_name=name)
        if client.has_dataset(dataset_name=name)
        else None
    )


def _queue_run_ids(client: Client, queue_id: str) -> set[str]:
    return {str(r.id) for r in client.list_runs_from_annotation_queue(queue_id)}


# ---- checks --------------------------------------------------------------------


def _what_broke(a: dict, s: Seed) -> Check:
    ok = (
        int(_num(a["failed_traces"])) == s.failed_this_week
        and s.top_error.lower() in str(a["top_error"]).lower()
    )
    return _ok(ok, a, {"failed_traces": s.failed_this_week, "top_error": s.top_error})


def _slowest(a: dict, s: Seed) -> Check:
    return _ok(
        str(a["trace_id"]).strip() == s.slowest_trace_id,
        a["trace_id"],
        s.slowest_trace_id,
    )


def _heaviest(a: dict, s: Seed) -> Check:
    return _ok(
        int(_num(a["total_tokens"])) == s.heaviest_llm_tokens,
        a["total_tokens"],
        s.heaviest_llm_tokens,
    )


def _pro_failures(a: dict, s: Seed) -> Check:
    return _ok(int(_num(a["count"])) == s.pro_failures, a["count"], s.pro_failures)


def _thread(a: dict, s: Seed) -> Check:
    ok = (
        int(_num(a["turns"])) == s.thread_turns
        and "package" in str(a["last_question"]).lower()
    )
    return _ok(
        ok, a, {"turns": s.thread_turns, "last_question": s.thread_last_question}
    )


def _thumbs_down(a: dict, s: Seed) -> Check:
    got = sorted(str(x).strip() for x in a["trace_ids"])
    return _ok(got == s.thumbs_down_trace_ids, got, s.thumbs_down_trace_ids)


def _compare(a: dict, s: Seed) -> Check:
    ok = (
        str(a["experiment"]).strip() == s.better_experiment
        and abs(_num(a["mean_correctness"]) - s.better_experiment_score) < 0.01
    )
    return _ok(
        ok,
        a,
        {
            "experiment": s.better_experiment,
            "mean_correctness": s.better_experiment_score,
        },
    )


def _runs_to_dataset(client: Client, s: Seed, _text: str) -> Check:
    ds = _dataset(client, f"{s.prefix}-review-set")
    if not ds:
        return Check(False, "dataset not created")
    got = sorted(
        str((e.inputs or {}).get("question", e.inputs))
        for e in client.list_examples(dataset_id=ds.id)
    )
    return _ok(
        got == sorted(s.needs_review_questions), got, sorted(s.needs_review_questions)
    )


def _add_example(client: Client, s: Seed, _text: str) -> Check:
    for e in client.list_examples(dataset_name=s.qa_dataset):
        if "speed of light" in json.dumps(e.inputs).lower():
            split = (e.metadata or {}).get("dataset_split") or []
            ok = "299" in json.dumps(e.outputs) and "test" in (
                split if isinstance(split, list) else [split]
            )
            return _ok(
                ok,
                {"outputs": e.outputs, "split": split},
                "answer with 299,792 km/s in split test",
            )
    return Check(False, "example not found")


def _prompt_tag(client: Client, s: Seed, _text: str) -> Check:
    name = f"{s.prefix}-triage"
    try:
        prompt = client.pull_prompt(f"{name}:prod")
    except Exception as e:  # noqa: BLE001 - any failure to resolve the tag fails the task
        return Check(False, f"cannot pull {name}:prod: {e}")
    text = json.dumps(
        prompt.to_json() if hasattr(prompt, "to_json") else str(prompt), default=str
    )
    return _ok(
        "{ticket}" in text,
        "template with {ticket}" if "{ticket}" in text else text[:200],
        "{ticket} variable",
    )


def _online_evaluator(client: Client, s: Seed, _text: str) -> Check:
    """A 25%-sampled rule on the project whose LLM judge writes `helpfulness`."""
    for r in _rules(client, s.project_id):
        if abs(float(r.get("sampling_rate") or 0) - 0.25) > 1e-9:
            continue
        # Older rules embed the LLM evaluator; rules from `evaluators create-llm`
        # point at a workspace evaluator, whose type and feedback keys we read back.
        judge = r.get("evaluators") or []
        if r.get("evaluator_id"):
            ev = client.request_with_retries(
                "GET", f"/api/v1/platform/evaluators/{r['evaluator_id']}"
            ).json()
            if ev.get("type") == "llm" or ev.get("llm_evaluator"):
                judge = [ev]
        if judge and "helpfulness" in json.dumps(judge, default=str):
            return Check(
                True, f"rule {r.get('display_name')}: 25% LLM judge for helpfulness"
            )
    return Check(
        False, "no 25%-sampled rule on the project with an LLM judge for `helpfulness`"
    )


def _error_rule(client: Client, s: Seed, _text: str) -> Check:
    ds = _dataset(client, s.errors_dataset)
    for r in _rules(client, s.project_id):
        filt = " ".join(
            str(r.get(k) or "") for k in ("filter", "trace_filter", "tree_filter")
        )
        if (
            ds
            and str(r.get("add_to_dataset_id")) == str(ds.id)
            and "error" in filt
            and float(r.get("sampling_rate") or 0) == 1.0
        ):
            return Check(
                True,
                f"rule {r.get('display_name')} adds errored runs to {s.errors_dataset}",
            )
    return Check(False, "no rule adding all errored runs to the errors dataset")


def _queue(client: Client, s: Seed, _text: str) -> Check:
    queues = [q for q in client.list_annotation_queues(name=f"{s.prefix}-triage-queue")]
    if not queues:
        return Check(False, "queue not created")
    got = sorted(_queue_run_ids(client, str(queues[0].id)))
    return _ok(got == s.thumbs_down_trace_ids, got, s.thumbs_down_trace_ids)


def _feedback(client: Client, s: Seed, _text: str) -> Check:
    for f in client.list_feedback(
        run_ids=[s.refund_trace_id], feedback_key=["resolved"]
    ):
        return _ok(
            f.score == 1 and "verified" in (f.comment or "").lower(),
            {"score": f.score, "comment": f.comment},
            {"score": 1, "comment": "verified"},
        )
    return Check(False, "no `resolved` feedback on the refund trace")


def _delete_scratch(client: Client, s: Seed, _text: str) -> Check:
    gone = not client.has_dataset(dataset_name=s.scratch_dataset)
    kept = client.has_dataset(dataset_name=s.qa_dataset) and client.has_dataset(
        dataset_name=s.errors_dataset
    )
    return Check(gone and kept, f"scratch deleted={gone}, other datasets kept={kept}")


_PREAMBLE = """You are working in LangSmith through its command-line tool, `langsmith`, which is installed and already authenticated.
"""

TASKS: list[Task] = [
    Task(
        "what-broke",
        "Find what broke recently",
        """
In the tracing project `{project}`, find what broke in the last 12 hours: how many traces failed, and which error message was the most common.
Finish with one line of JSON: {{"failed_traces": <number>, "top_error": "<most common error message>"}}
""",
        _answer(_what_broke),
        "answer",
    ),
    Task(
        "slowest-trace",
        "Find the slowest trace",
        """
Which trace in the tracing project `{project}` took the longest in the last 12 hours?
Finish with one line of JSON: {{"trace_id": "<id>"}}
""",
        _answer(_slowest),
        "answer",
    ),
    Task(
        "heaviest-llm-call",
        "Find the most expensive LLM call",
        """
In the tracing project `{project}`, find the LLM call that used the most tokens.
Finish with one line of JSON: {{"total_tokens": <number>}}
""",
        _answer(_heaviest),
        "answer",
    ),
    Task(
        "failures-by-metadata",
        "Filter failures by metadata",
        """
In the tracing project `{project}`, how many traces from the last 12 hours failed for users whose metadata has `user_tier` set to `pro`?
Finish with one line of JSON: {{"count": <number>}}
""",
        _answer(_pro_failures),
        "answer",
    ),
    Task(
        "thread-summary",
        "Read a conversation thread",
        """
In the tracing project `{project}`, the conversation with thread ID `{thread_id}` has several turns. How many turns does it have, and what was the user's last question?
Finish with one line of JSON: {{"turns": <number>, "last_question": "<text>"}}
""",
        _answer(_thread),
        "answer",
    ),
    Task(
        "negative-feedback",
        "Find traces users rated badly",
        """
Users rate traces in the tracing project `{project}` with the feedback key `user_score` (1 is good, 0 is bad). Which traces got a 0?
Finish with one line of JSON: {{"trace_ids": ["<id>", ...]}}
""",
        _answer(_thumbs_down),
        "answer",
    ),
    Task(
        "compare-experiments",
        "Compare two experiments",
        """
The dataset `{qa_dataset}` has two experiments. Which one has the higher average `correctness` score, and what is that average?
Finish with one line of JSON: {{"experiment": "<experiment name>", "mean_correctness": <number>}}
""",
        _answer(_compare),
        "answer",
    ),
    Task(
        "runs-to-dataset",
        "Add runs to a dataset",
        """
In the tracing project `{project}`, some traces are tagged `needs-review`. Create a dataset named `{prefix}-review-set` and add one example per tagged trace, using the trace's inputs as the example inputs and its outputs (if any) as the example outputs.
""",
        _runs_to_dataset,
        "state",
    ),
    Task(
        "add-example",
        "Add an example to a dataset",
        """
Add an example to the dataset `{qa_dataset}` with inputs {{"question": "What is the speed of light?"}} and outputs {{"answer": "299,792 km/s"}}, in the `test` split.
""",
        _add_example,
        "state",
    ),
    Task(
        "prompt-commit-tag",
        "Create a prompt and tag a commit",
        """
Create a prompt named `{prefix}-triage` whose template is `Classify this support ticket: {{ticket}}`, then tag its commit `prod`.
""",
        _prompt_tag,
        "state",
    ),
    Task(
        "online-evaluator",
        "Set up an online evaluator",
        """
Set up an online LLM-as-judge evaluator named `{prefix}-helpfulness` on the tracing project `{project}` that scores whether each answer is helpful (feedback key `helpfulness`). It should run on 25% of traces.
""",
        _online_evaluator,
        "state",
    ),
    Task(
        "errors-to-dataset-rule",
        "Automate collecting errors",
        """
Make sure every run that errors in the tracing project `{project}` is added automatically to the dataset `{errors_dataset}` from now on. All errored runs should be collected, not a sample.
""",
        _error_rule,
        "state",
    ),
    Task(
        "annotation-queue",
        "Send traces for human review",
        """
Create an annotation queue named `{prefix}-triage-queue` and add to it every trace in the tracing project `{project}` that has `user_score` feedback of 0.
""",
        _queue,
        "state",
    ),
    Task(
        "leave-feedback",
        "Leave feedback on a trace",
        """
In the tracing project `{project}`, find the trace where the user asked about a refund for order 7731 and record feedback on it with key `resolved`, score 1, and the comment `verified`.
""",
        _feedback,
        "state",
    ),
    Task(
        "delete-dataset",
        "Delete a dataset safely",
        """
Delete the dataset `{scratch_dataset}`. It is no longer needed. Do not change anything else.
""",
        _delete_scratch,
        "state",
    ),
]

for _t in TASKS:
    _t.prompt = _PREAMBLE + _t.prompt


def by_id(ids: list[str] | None) -> list[Task]:
    if not ids:
        return TASKS
    known = {t.id: t for t in TASKS}
    missing = [i for i in ids if i not in known]
    if missing:
        raise SystemExit(
            f"unknown task ids: {', '.join(missing)}; known: {', '.join(known)}"
        )
    return [known[i] for i in ids]
