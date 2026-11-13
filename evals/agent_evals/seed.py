"""Seed a LangSmith workspace with the fixed data the tasks ask about.

Every resource is named with a per-run prefix (`cli-eval-<run id>`), so concurrent
runs never see each other's data and `cleanup` can remove everything a run made,
including what the agent created. Facts the tasks check are recorded in `Seed`
when the data is written, not hard-coded in the checks.
"""

from __future__ import annotations

import datetime as dt
import time
import uuid
from dataclasses import dataclass, field

from langsmith import Client

UTC = dt.UTC


@dataclass
class Seed:
    prefix: str
    project: str
    project_id: str = ""
    qa_dataset: str = ""
    errors_dataset: str = ""
    scratch_dataset: str = ""
    thread_id: str = ""
    # Facts the checks compare against.
    failed_this_week: int = 0
    top_error: str = ""
    slowest_trace_id: str = ""
    heaviest_llm_tokens: int = 0
    pro_failures: int = 0
    thread_turns: int = 0
    thread_last_question: str = ""
    thumbs_down_trace_ids: list[str] = field(default_factory=list)
    needs_review_questions: list[str] = field(default_factory=list)
    refund_trace_id: str = ""
    better_experiment: str = ""
    better_experiment_score: float = 0.0
    worse_experiment: str = ""
    # IDs of everything seeded, so the scope guard can allow writes that target them.
    known_ids: list[str] = field(default_factory=list)

    def names(self) -> dict[str, str]:
        """Values substituted into task prompts."""
        return {
            "prefix": self.prefix,
            "project": self.project,
            "qa_dataset": self.qa_dataset,
            "errors_dataset": self.errors_dataset,
            "scratch_dataset": self.scratch_dataset,
            "thread_id": self.thread_id,
        }


# Questions sent to the seeded support agent: (question, error, user_tier, latency_s,
# tags, llm_total_tokens, hours_ago). The error column decides which traces failed.
# LangSmith only accepts runs that start within 24 hours of now, so every seeded
# trace is from the last day; "recent" means the last 12 hours.
_TRACES = [
    ("How do I reset my password?", None, "free", 2.1, [], 900, 2),
    ("Can I get a refund for order 7731?", None, "pro", 41.5, [], 2400, 4),
    (
        "Why was I charged twice this month?",
        "RateLimitError: 429 Too Many Requests from model provider",
        "pro",
        3.2,
        ["needs-review"],
        1100,
        2,
    ),
    ("Change the email on my account", None, "free", 1.4, [], 700, 6),
    (
        "My invoice shows the wrong company name",
        "RateLimitError: 429 Too Many Requests from model provider",
        "free",
        2.8,
        [],
        1000,
        4,
    ),
    ("Summarize my last five support tickets", None, "pro", 7.9, [], 18000, 7),
    (
        "Cancel my subscription",
        "RateLimitError: 429 Too Many Requests from model provider",
        "pro",
        3.0,
        ["needs-review"],
        950,
        9,
    ),
    ("What plans do you offer?", None, "free", 1.1, [], 600, 2),
    (
        "Look up order status",
        "ValueError: missing order_id in tool call lookup_order",
        "free",
        0.9,
        ["needs-review"],
        500,
        6,
    ),
    ("Do you ship to Canada?", None, "free", 1.6, [], 650, 10),
    ("Export my data", None, "pro", 5.5, [], 1500, 4),
    ("How do I add a teammate?", None, "free", 1.3, [], 800, 10),
]
# Failures from before the 12-hour window, which "recent" questions must leave out.
_OLD_TRACES = [
    (
        "Translate this FAQ",
        "TimeoutError: tool translate timed out after 30s",
        "free",
        30.0,
        [],
        1200,
        20,
    ),
    (
        "Translate the docs",
        "TimeoutError: tool translate timed out after 30s",
        "free",
        30.0,
        [],
        1200,
        20,
    ),
    (
        "Translate the blog",
        "TimeoutError: tool translate timed out after 30s",
        "pro",
        30.0,
        [],
        1200,
        19,
    ),
    (
        "Translate the terms",
        "TimeoutError: tool translate timed out after 30s",
        "pro",
        30.0,
        [],
        1200,
        19,
    ),
]
RECENT_HOURS = 12
# A three-turn conversation sharing one thread_id.
_THREAD = [
    "I ordered a desk last week",
    "It still says processing",
    "Where is my package now?",
]
_QA = [
    ("What is the capital of France?", "Paris"),
    ("What is 2 + 2?", "4"),
    ("Who wrote Hamlet?", "William Shakespeare"),
    ("What color is the sky on a clear day?", "Blue"),
    ("How many days are in a leap year?", "366"),
]


def _dotted(start: dt.datetime, run_id: str) -> str:
    return start.strftime("%Y%m%dT%H%M%S%fZ") + run_id


def _trace(
    project: str,
    question: str,
    error: str | None,
    tier: str,
    latency: float,
    tags: list[str],
    tokens: int,
    start: dt.datetime,
    extra_meta: dict | None = None,
) -> tuple[str, list[dict]]:
    """Build one support-agent trace: a chain root with an LLM child."""
    root_id = str(uuid.uuid4())
    llm_id = str(uuid.uuid4())
    end = start + dt.timedelta(seconds=latency)
    llm_start = start + dt.timedelta(milliseconds=50)
    llm_end = end - dt.timedelta(milliseconds=50)
    answer = None if error else f"Here is how I can help with: {question}"
    root_dotted = _dotted(start, root_id)
    meta = {"user_tier": tier, **(extra_meta or {})}
    root = {
        "id": root_id,
        "trace_id": root_id,
        "dotted_order": root_dotted,
        "name": "support_agent",
        "run_type": "chain",
        "inputs": {"question": question},
        "outputs": {"answer": answer} if answer else None,
        "error": error,
        "start_time": start,
        "end_time": end,
        "session_name": project,
        "tags": tags,
        "extra": {"metadata": meta},
    }
    input_tokens = int(tokens * 0.8)
    llm = {
        "id": llm_id,
        "trace_id": root_id,
        "parent_run_id": root_id,
        "dotted_order": root_dotted + "." + _dotted(llm_start, llm_id),
        "name": "ChatAnthropic",
        "run_type": "llm",
        "inputs": {"messages": [{"role": "user", "content": question}]},
        "outputs": {
            "content": answer or "",
            "usage_metadata": {
                "input_tokens": input_tokens,
                "output_tokens": tokens - input_tokens,
                "total_tokens": tokens,
            },
        },
        "error": error,
        "start_time": llm_start,
        "end_time": llm_end,
        "session_name": project,
        "extra": {
            "metadata": {
                **meta,
                "ls_provider": "anthropic",
                "ls_model_name": "claude-haiku-5-5",
            }
        },
    }
    return root_id, [root, llm]


def prefix_for(run_id: str) -> str:
    return f"cli-eval-{run_id}"


def seed(client: Client, run_id: str, now: dt.datetime | None = None) -> Seed:
    now = now or dt.datetime.now(UTC)
    prefix = prefix_for(run_id)
    s = Seed(prefix=prefix, project=f"{prefix}-support-bot")
    s.project_id = str(
        client.create_project(s.project, description="Seeded by the CLI agent evals").id
    )

    runs: list[dict] = []
    root_ids: dict[str, str] = {}
    for q, err, tier, lat, tags, tok, hours_ago in _TRACES + _OLD_TRACES:
        start = now - dt.timedelta(hours=hours_ago)
        rid, rs = _trace(s.project, q, err, tier, lat, tags, tok, start)
        root_ids[q] = rid
        runs += rs
    s.thread_id = f"{prefix}-thread-1"
    for i, q in enumerate(_THREAD):
        start = now - dt.timedelta(hours=5) + dt.timedelta(minutes=3 * i)
        _, rs = _trace(
            s.project, q, None, "free", 1.2, [], 400, start, {"thread_id": s.thread_id}
        )
        runs += rs
    for i in range(0, len(runs), 50):
        client.batch_ingest_runs(create=runs[i : i + 50])
    s.known_ids += [s.project_id, *(r["id"] for r in runs)]

    this_week = [t for t in _TRACES if t[1]]
    s.failed_this_week = len(this_week)
    s.top_error = "RateLimitError"
    s.slowest_trace_id = root_ids[max(_TRACES, key=lambda t: t[3])[0]]
    s.heaviest_llm_tokens = max(t[5] for t in _TRACES + _OLD_TRACES)
    s.pro_failures = sum(1 for t in this_week if t[2] == "pro")
    s.thread_turns = len(_THREAD)
    s.thread_last_question = _THREAD[-1]
    s.needs_review_questions = [t[0] for t in _TRACES if "needs-review" in t[4]]
    s.refund_trace_id = root_ids["Can I get a refund for order 7731?"]

    _wait_for_runs(
        client, s.project, expected_roots=len(_TRACES) + len(_OLD_TRACES) + len(_THREAD)
    )

    # User ratings: two thumbs-down, the rest of the recent successes thumbs-up.
    down = ["Export my data", "How do I reset my password?"]
    s.thumbs_down_trace_ids = sorted(root_ids[q] for q in down)
    for q, err, *_ in _TRACES:
        if not err:
            client.create_feedback(
                root_ids[q],
                key="user_score",
                score=0 if q in down else 1,
                trace_id=root_ids[q],
            )

    # Datasets: a QA set with two experiments, an empty target for a rule, a scratch set to delete.
    s.qa_dataset = f"{prefix}-qa"
    qa = client.create_dataset(s.qa_dataset, description="Seeded QA set")
    client.create_examples(
        dataset_id=qa.id,
        examples=[
            {"inputs": {"question": q}, "outputs": {"answer": a}} for q, a in _QA
        ],
    )
    s.errors_dataset = f"{prefix}-errors"
    errors = client.create_dataset(
        s.errors_dataset, description="Errored runs land here"
    )
    s.scratch_dataset = f"{prefix}-scratch"
    scratch = client.create_dataset(s.scratch_dataset, description="Safe to delete")
    client.create_examples(
        dataset_id=scratch.id, examples=[{"inputs": {"q": "x"}, "outputs": {"a": "y"}}]
    )

    examples = sorted(
        client.list_examples(dataset_id=qa.id), key=lambda e: e.inputs["question"]
    )
    s.known_ids += [
        str(qa.id),
        str(errors.id),
        str(scratch.id),
        *(str(e.id) for e in examples),
    ]
    scores = {"baseline": [1, 0, 0, 1, 0], "candidate": [1, 1, 0, 1, 1]}
    exp_names = {}
    for label, marks in scores.items():
        name = f"{prefix}-exp-{label}"
        exp_names[label] = name
        exp = client.create_project(name, reference_dataset_id=qa.id)
        exp_runs = []
        for ex, mark in zip(examples, marks):
            rid = str(uuid.uuid4())
            start = now - dt.timedelta(hours=2)
            exp_runs.append(
                {
                    "id": rid,
                    "trace_id": rid,
                    "dotted_order": _dotted(start, rid),
                    "name": "qa_target",
                    "run_type": "chain",
                    "inputs": ex.inputs,
                    "outputs": {"answer": ex.outputs["answer"] if mark else "unsure"},
                    "start_time": start,
                    "end_time": start + dt.timedelta(seconds=1),
                    "session_name": name,
                    "reference_example_id": str(ex.id),
                }
            )
        client.batch_ingest_runs(create=exp_runs)
        s.known_ids += [str(exp.id), *(r["id"] for r in exp_runs)]
        _wait_for_runs(client, name, expected_roots=len(exp_runs))
        for r, mark in zip(exp_runs, marks):
            client.create_feedback(
                r["id"],
                key="correctness",
                score=mark,
                trace_id=r["id"],
                project_id=exp.id,
            )
    s.better_experiment = exp_names["candidate"]
    s.better_experiment_score = sum(scores["candidate"]) / len(scores["candidate"])
    s.worse_experiment = exp_names["baseline"]
    client.flush()
    return s


def _wait_for_runs(
    client: Client, project: str, expected_roots: int, timeout_s: float = 180
) -> None:
    deadline = time.time() + timeout_s
    seen = 0
    while time.time() < deadline:
        seen = sum(
            1
            for _ in client.list_runs(project_name=project, is_root=True, select=["id"])
        )
        if seen >= expected_roots:
            return
        time.sleep(3)
    raise TimeoutError(
        f"only {seen}/{expected_roots} root runs visible in {project} after {timeout_s}s"
    )


def cleanup(client: Client, prefix: str) -> list[str]:
    """Delete every project, dataset, prompt, queue and rule whose name starts with prefix."""
    removed: list[str] = []
    assert prefix.startswith("cli-eval-"), prefix  # never sweep anything else
    for ds in client.list_datasets(dataset_name_contains=prefix):
        if ds.name.startswith(prefix):
            client.delete_dataset(dataset_id=ds.id)
            removed.append(f"dataset {ds.name}")
    for q in client.list_annotation_queues(name_contains=prefix):
        if q.name.startswith(prefix):
            client.delete_annotation_queue(q.id)
            removed.append(f"queue {q.name}")
    try:
        prompts = client.list_prompts(query=prefix, limit=100).repos
    except Exception:  # noqa: BLE001 - prompt hub may be disabled on self-hosted
        prompts = []
    for p in prompts:
        if p.repo_handle.startswith(prefix):
            client.delete_prompt(p.repo_handle)
            removed.append(f"prompt {p.repo_handle}")
    for proj in client.list_projects(name_contains=prefix):
        if proj.name.startswith(prefix):
            for rule in client.request_with_retries(
                "GET",
                "/runs/rules",
                request_kwargs={"params": {"session_id": str(proj.id)}},
            ).json():
                client.request_with_retries("DELETE", f"/runs/rules/{rule['id']}")
                removed.append(f"rule {rule.get('display_name')}")
            client.delete_project(project_id=proj.id)
            removed.append(f"project {proj.name}")
    # Evaluators are workspace-level; the tasks ask the agent to name them with the prefix.
    evaluators = (
        client.request_with_retries(
            "GET",
            "/api/v1/platform/evaluators",
            request_kwargs={"params": {"name_contains": prefix, "limit": 100}},
        )
        .json()
        .get("evaluators")
        or []
    )
    for ev in evaluators:
        if ev.get("name", "").startswith(prefix):
            client.request_with_retries(
                "DELETE",
                f"/api/v1/platform/evaluators/{ev['id']}",
                request_kwargs={"params": {"delete_run_rules": "true"}},
            )
            removed.append(f"evaluator {ev['name']}")
    return removed
