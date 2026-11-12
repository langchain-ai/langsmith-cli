from agent_evals import scoring
from agent_evals.agent import Call, Transcript
from agent_evals.seed import Seed
from agent_evals.tasks import TASKS, final_json


def test_final_json_takes_last_outermost_object():
    text = (
        'I ran {"a": 1}. Result:\n{"trace_ids": ["x", "y"], "nested": {"k": 2}}\nDone.'
    )
    assert final_json(text) == {"trace_ids": ["x", "y"], "nested": {"k": 2}}
    assert final_json("no json here") is None
    assert final_json('```json\n{"count": 2}\n```') == {"count": 2}


def test_task_set_shape():
    ids = [t.id for t in TASKS]
    assert len(ids) == 15 and len(set(ids)) == 15
    seed = Seed(
        prefix="cli-eval-x",
        project="cli-eval-x-support-bot",
        qa_dataset="q",
        errors_dataset="e",
        scratch_dataset="s",
        thread_id="t",
    )
    for t in TASKS:
        prompt = t.render(seed)
        # Every placeholder is filled in; `{ticket}` is literal prompt-template text.
        assert not any("{" + k + "}" in prompt for k in seed.names()), t.id
        assert t.kind in ("answer", "state")


def _seed() -> Seed:
    return Seed(
        prefix="p",
        project="p-proj",
        failed_this_week=4,
        top_error="RateLimitError",
        slowest_trace_id="abc",
        thumbs_down_trace_ids=["a", "b"],
        better_experiment="p-exp-candidate",
        better_experiment_score=0.8,
    )


def test_answer_checks():
    checks = {t.id: t.check for t in TASKS}
    s = _seed()
    assert checks["what-broke"](
        None, s, '{"failed_traces": 4, "top_error": "RateLimitError: 429 Too Many"}'
    ).passed
    assert not checks["what-broke"](
        None, s, '{"failed_traces": 8, "top_error": "RateLimitError"}'
    ).passed
    assert checks["negative-feedback"](None, s, '{"trace_ids": ["b", "a"]}').passed
    assert not checks["negative-feedback"](None, s, '{"trace_ids": ["a"]}').passed
    assert checks["compare-experiments"](
        None, s, '{"experiment": "p-exp-candidate", "mean_correctness": "0.80"}'
    ).passed
    assert not checks["slowest-trace"](None, s, "I could not find it").passed
    assert not checks["slowest-trace"](
        None, s, '{"id": "abc"}'
    ).passed  # wrong key is a malformed answer


def _transcript(model, tokens, made_up=0, failed=0):
    calls = [Call(["x"], 1, 1, 0, "", "x", True, ["x"], False) for _ in range(made_up)]
    calls += [Call(["y"], 1, 1, 0, "", "y", False, [], False) for _ in range(failed)]
    calls += [Call(["runs", "list"], 0, 1, 10, "", "runs list", False, [], False)]
    return Transcript(
        task_id="t", model=model, calls=calls, input_tokens=tokens, output_tokens=0
    )


def _rows(model, results):
    rows = []
    for i, (task_id, passed, tokens) in enumerate(results):
        tr = _transcript(model, tokens, made_up=0 if passed else 1)
        rows.append(
            {
                "task_id": task_id,
                "model": model,
                "repetition": 1,
                "check": "",
                "calls": [],
                **scoring.metrics(tr, passed),
            }
        )
    return rows


def test_metrics_counts():
    m = scoring.metrics(_transcript("claude-opus-5-5", 1000, made_up=2, failed=1), True)
    assert m["made_up_calls"] == 2 and m["failed_calls"] == 3 and m["cli_calls"] == 4
    assert m["total_tokens"] == 1000 and m["cost_usd"] == 0.004


def test_launch_criteria():
    model = "claude-opus-5-5"
    base = scoring.summarize(
        _rows(model, [(f"t{i}", True, 10_000) for i in range(15)]), total_tasks=15
    )
    baseline = {"models": base}
    good = scoring.summarize(
        _rows(model, [(f"t{i}", i < 13, 5_000) for i in range(15)]), 15, baseline
    )[model]
    assert good["launch"]["met"] and good["token_reduction_vs_baseline"] == 0.5
    few = scoring.summarize(
        _rows(model, [(f"t{i}", i < 12, 5_000) for i in range(15)]), 15, baseline
    )[model]
    assert not few["launch"]["tasks_ok"] and few["launch"]["tokens_ok"]
    heavy = scoring.summarize(
        _rows(model, [(f"t{i}", True, 7_000) for i in range(15)]), 15, baseline
    )[model]
    assert heavy["launch"]["tasks_ok"] and not heavy["launch"]["tokens_ok"]
    assert not scoring.summarize(_rows(model, [("t0", True, 1)]), 15, baseline)[model][
        "launch"
    ]["tasks_ok"]


def test_every_repeat_is_required_for_launch():
    model = "claude-opus-5-5"
    rows = _rows(model, [(f"t{i}", True, 1) for i in range(15)])
    flaky = _rows(model, [(f"t{i}", i >= 3, 1) for i in range(15)])
    for r in flaky:
        r["repetition"] = 2
    s = scoring.summarize(rows + flaky, 15)[model]
    assert s["tasks_passed"] == 15 and s["tasks_passed_every_repeat"] == 12
    assert not s["launch"]["tasks_ok"]
    assert "t0" in scoring.markdown({model: s}, {"cli_version": "v1"}, rows + flaky)


def test_baseline_only_compares_the_same_model():
    opus = scoring.summarize(
        _rows("claude-opus-5-5", [(f"t{i}", True, 10_000) for i in range(15)]), 15
    )
    haiku = scoring.summarize(
        _rows("claude-haiku-5-5", [(f"t{i}", True, 1_000) for i in range(15)]),
        15,
        {"models": opus},
    )
    assert haiku["claude-haiku-5-5"]["token_reduction_vs_baseline"] is None
