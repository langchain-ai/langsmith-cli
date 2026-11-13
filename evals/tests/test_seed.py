import datetime as dt
from types import SimpleNamespace as NS
from unittest.mock import MagicMock

from agent_evals import seed


def _fake_client():
    client = MagicMock()
    ingested = []
    client.batch_ingest_runs.side_effect = lambda create: ingested.extend(create)
    client.list_runs.side_effect = lambda **kw: [NS(id=i) for i in range(100)]
    client.create_project.side_effect = lambda name, **kw: NS(
        id=f"proj-{name}", name=name
    )
    client.create_dataset.side_effect = lambda name, **kw: NS(
        id=f"ds-{name}", name=name
    )
    client.list_examples.side_effect = lambda **kw: [
        NS(id=f"ex-{i}", inputs={"question": q}, outputs={"answer": a})
        for i, (q, a) in enumerate(seed._QA)
    ]
    return client, ingested


def test_every_seeded_run_starts_within_the_ingest_window():
    # LangSmith rejects runs that start more than 24 hours from now.
    now = dt.datetime.now(dt.UTC)
    client, ingested = _fake_client()
    s = seed.seed(client, "test", now=now)
    assert ingested
    for run in ingested:
        assert now - dt.timedelta(hours=23) <= run["start_time"] <= now, run["name"]

    # The facts the "last 12 hours" tasks check match what was ingested in that
    # window, and older failures exist outside it for those tasks to leave out.
    roots = [
        r
        for r in ingested
        if r["session_name"] == s.project and not r.get("parent_run_id")
    ]
    cutoff = now - dt.timedelta(hours=12)
    recent = [r for r in roots if r["start_time"] >= cutoff]
    older = [r for r in roots if r["start_time"] < cutoff]
    recent_failed = [r for r in recent if r["error"]]
    assert len(recent_failed) == s.failed_this_week
    assert s.pro_failures == sum(
        1 for r in recent_failed if r["extra"]["metadata"]["user_tier"] == "pro"
    )
    assert any(r["error"] for r in older)
    slowest = max(recent, key=lambda r: r["end_time"] - r["start_time"])
    assert slowest["id"] == s.slowest_trace_id
