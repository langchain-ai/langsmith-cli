"""The agent loop against a scripted model and the real CLI binary (no network)."""

from types import SimpleNamespace as NS

from agent_evals import agent


class FakeMessages:
    def __init__(self, script):
        self.script = list(script)
        self.requests = []

    def create(self, **kw):
        self.requests.append({**kw, "messages": list(kw["messages"])})
        content = self.script.pop(0)
        stop = "tool_use" if any(b.type == "tool_use" for b in content) else "end_turn"
        usage = NS(
            input_tokens=100,
            output_tokens=20,
            cache_read_input_tokens=50,
            cache_creation_input_tokens=0,
        )
        return NS(content=content, stop_reason=stop, usage=usage)


def tool(i, args):
    return NS(type="tool_use", id=f"tu{i}", name="langsmith", input={"args": args})


def text(t):
    return NS(type="text", text=t)


def test_loop_records_calls_tokens_and_made_up_commands(binary, tree):
    script = [
        [tool(1, ["datasets", "--help"])],
        [
            tool(2, ["feedback", "create", "--key", "x"]),
            tool(3, ["runs", "list", "--errors-only"]),
        ],
        [text('Done. {"count": 2}')],
    ]
    fake = NS(messages=FakeMessages(script))
    cfg = agent.AgentConfig(model="claude-haiku-5-5", binary=binary, tree=tree)
    tr = agent.run_task(fake, cfg, "t", "do it")
    assert tr.error is None and tr.turns == 3
    assert [c.command for c in tr.calls] == ["datasets", "feedback", "runs list"]
    assert [c.made_up for c in tr.calls] == [False, True, True]
    assert tr.calls[0].is_help
    assert (
        tr.input_tokens == 450
        and tr.output_tokens == 60
        and tr.cache_read_tokens == 150
    )
    assert tr.final_text.endswith('{"count": 2}')
    # Tool results for parallel calls go back in one user message, in order.
    results = fake.messages.requests[2]["messages"][-1]["content"]
    assert [r["tool_use_id"] for r in results] == ["tu2", "tu3"]


def test_api_only_surface_blocks_other_commands(binary, tree):
    script = [[tool(1, ["datasets", "list"])], [text("ok")]]
    fake = NS(messages=FakeMessages(script))
    cfg = agent.AgentConfig(model="m", binary=binary, tree=tree, api_only=True)
    tr = agent.run_task(fake, cfg, "t", "do it")
    assert tr.calls[0].blocked and tr.calls[0].exit != 0


def test_cli_env_drops_model_keys(monkeypatch):
    monkeypatch.setenv("ANTHROPIC_API_KEY", "secret")
    monkeypatch.setenv("LANGSMITH_API_KEY", "ls")
    env = agent.cli_env()
    assert "ANTHROPIC_API_KEY" not in env and env["LANGSMITH_API_KEY"] == "ls"


def test_turn_limit_is_reported(binary, tree):
    script = [[tool(i, ["datasets", "--help"])] for i in range(3)]
    fake = NS(messages=FakeMessages(script))
    cfg = agent.AgentConfig(model="m", binary=binary, tree=tree, max_turns=3)
    tr = agent.run_task(fake, cfg, "t", "do it")
    assert tr.error == "stopped after 3 turns"


def test_scope_guard_refuses_writes_outside_the_run(tree, tmp_path):
    # A stand-in binary that prints a new ID, as a create command would.
    fake = tmp_path / "langsmith"
    fake.write_text(
        '#!/bin/sh\necho \'{"id": "11111111-2222-3333-4444-555555555555"}\'\n'
    )
    fake.chmod(0o755)
    cfg = agent.AgentConfig(model="m", binary=str(fake), tree=tree)
    scope = ["cli-eval-abc", "aaaaaaaa-0000-0000-0000-000000000000"]

    def refused(args):
        _, _, call = agent.run_cli(cfg, args, scope)
        return call.unsafe_write

    assert refused(["datasets", "delete", "someone-elses-dataset", "--yes"])
    assert refused(
        ["api", "datasets/bbbbbbbb-0000-0000-0000-000000000000", "-X", "DELETE"]
    )
    assert refused(
        ["api", "runs/rules/bbbbbbbb-0000-0000-0000-000000000000", "--method", "PATCH"]
    )
    assert not refused(["datasets", "delete", "cli-eval-abc-scratch", "--yes"])
    assert not refused(
        ["api", "datasets/aaaaaaaa-0000-0000-0000-000000000000", "-X", "DELETE"]
    )
    assert refused(
        ["api", "runs/delete", "--body", '{"trace_ids": ["x"]}']
    )  # POST that deletes
    assert refused(["online-evaluators", "bulk-delete", "--evaluator-id", "x"])
    seeded_run = (
        '{"run_id": "aaaaaaaa-0000-0000-0000-000000000000", "comment": "please update"}'
    )
    assert not refused(
        ["api", "feedback", "--body", seeded_run]
    )  # write words in a body don't count
    assert not refused(["datasets", "list"])
    # Creating is fine inside the run, refused against someone else's resources.
    assert not refused(["datasets", "create", "--name", "cli-eval-abc-review-set"])
    assert refused(
        ["examples", "create", "--dataset", "someone-elses-dataset", "--inputs", "{}"]
    )
    assert refused(
        [
            "api",
            "runs/rules",
            "--body",
            '{"session_id": "bbbbbbbb-0000-0000-0000-000000000000"}',
        ]
    )
    assert not refused(
        ["api", "runs/query", "--body", "{}"]
    )  # a POST read is not a write
    # Output-shaping flags can't vouch for a write's target.
    assert refused(
        ["datasets", "delete", "someone-elses-dataset", "--yes", "--jq", "cli-eval-abc"]
    )
    assert not refused(["datasets", "delete", "--help"])
    assert refused(["api", "v1/fleet/secrets", "--body", "{}"])
    mixed = '{"trace_ids": ["aaaaaaaa-0000-0000-0000-000000000000", "bbbbbbbb-0000-0000-0000-000000000000"]}'
    assert refused(
        ["api", "runs/delete", "--body", mixed]
    )  # one known ID can't vouch for a foreign one
    # IDs the agent's own in-scope create calls return join the scope; an
    # out-of-scope create is refused before it runs, so its ID never does.
    fresh = ["cli-eval-abc"]
    delete_new = [
        "api",
        "datasets/11111111-2222-3333-4444-555555555555",
        "-X",
        "DELETE",
    ]
    _, _, call = agent.run_cli(cfg, ["datasets", "create", "--name", "x"], fresh)
    assert call.unsafe_write and agent.run_cli(cfg, delete_new, fresh)[2].unsafe_write
    agent.run_cli(cfg, ["datasets", "create", "--name", "cli-eval-abc-new"], fresh)
    assert not agent.run_cli(cfg, delete_new, fresh)[2].unsafe_write


def test_runner_access_is_refused(tree):
    cfg = agent.AgentConfig(model="m", binary="/bin/false", tree=tree)
    for args in (
        ["api", "datasets", "--input", "/proc/1/environ"],
        ["api", "datasets", "--body", "@/etc/passwd"],
        ["api", "datasets", "-F", "name=@/etc/hostname"],
        ["prompt", "push", "x", "--file", "/etc/passwd"],
        ["datasets", "list", "--api-key-cmd", "cat ~/.ssh/id_rsa"],
        ["datasets", "list", "--api-url", "https://example.com"],
        ["online-evaluators", "create", "--base-url=https://example.com"],
        ["datasets", "list", "--workspace", "00000000-0000-0000-0000-000000000000"],
        ["self-update"],
        ["auth", "logout"],
        ["traces", "setup", "--yes"],
    ):
        _, is_err, call = agent.run_cli(cfg, args)
        assert is_err and call.blocked and not call.unsafe_write, args
    _, _, call = agent.run_cli(cfg, ["api", "--help"])
    assert not call.blocked


def test_files_must_stay_in_the_attempt_directory(tree, tmp_path):
    cfg = agent.AgentConfig(model="m", binary="/bin/true", tree=tree)
    work = str(tmp_path / "work")
    (tmp_path / "work").mkdir()

    def blocked(args):
        _, _, call = agent.run_cli(cfg, args, None, work)
        return call.blocked

    # Reading or writing outside it is refused, through any flag or argument form.
    assert blocked(["datasets", "list", "--output", "/tmp/overwrite.json"])
    assert blocked(["datasets", "list", "-o", "../escape.json"])
    assert blocked(["insights", "create", "--config", "/etc/hostname"])
    assert blocked(["api", "datasets", "--input", "/proc/1/environ"])
    assert blocked(["datasets", "export", "cli-eval-x-qa", "/tmp/out.json"])
    assert blocked(["api", "datasets", "--body", "@../secret.json"])
    # Inside it, files are fine: the agent writes a config and passes it.
    assert agent.write_file(work, "model.json", "{}") == (
        "wrote 2 characters to model.json",
        False,
    )
    assert not blocked(
        ["evaluators", "create-llm", "--model-config", "model.json", "--name", "x"]
    )
    assert not blocked(["datasets", "list", "--output", "out/list.json"])
    assert not blocked(["datasets", "export", "cli-eval-x-qa", "out.json"])
    assert not blocked(["api", "datasets", "--body", '{"name": "a/b"}'])
    # write_file can't escape either.
    assert agent.write_file(work, "../x", "y")[1]
    assert agent.write_file(work, "/etc/x", "y")[1]


def test_method_spellings_are_all_guarded(tree, tmp_path):
    cfg = agent.AgentConfig(model="m", binary="/bin/true", tree=tree)
    foreign = '{"run_id": "bbbbbbbb-0000-0000-0000-000000000000", "key": "x"}'
    for args in (
        ["api", "feedback", "--method=POST", "--body", foreign],
        ["api", "feedback", "-X", "POST", "--body", foreign],
        ["api", "feedback", "-XPOST", "--body", foreign],
        ["api", "feedback", "--body", foreign],
    ):
        _, _, call = agent.run_cli(cfg, args, ["cli-eval-abc"], str(tmp_path))
        assert call.unsafe_write, args


def test_files_sent_are_judged_by_their_contents(tree, tmp_path):
    cfg = agent.AgentConfig(model="m", binary="/bin/true", tree=tree)
    work = str(tmp_path)
    scope = ["cli-eval-abc", "aaaaaaaa-0000-0000-0000-000000000000"]
    foreign = '{"run_id": "bbbbbbbb-0000-0000-0000-000000000000", "key": "x"}'
    seeded = '{"run_id": "aaaaaaaa-0000-0000-0000-000000000000", "key": "x"}'
    agent.write_file(work, "cli-eval-abc-foreign.json", foreign)
    agent.write_file(work, "cli-eval-abc-seeded.json", seeded)

    def unsafe(args):
        return agent.run_cli(cfg, args, scope, work)[2].unsafe_write

    # A prefixed file name can't vouch for a foreign ID inside the file.
    assert unsafe(["api", "feedback", "--input", "cli-eval-abc-foreign.json"])
    assert unsafe(["api", "feedback", "--body", "@cli-eval-abc-foreign.json"])
    assert unsafe(["api", "feedback", "--input=cli-eval-abc-foreign.json"])
    assert unsafe(["api", "feedback", "--body=@cli-eval-abc-foreign.json"])
    assert not unsafe(["api", "feedback", "--input", "cli-eval-abc-seeded.json"])
    # A file named like an endpoint is not read: the endpoint itself is judged.
    agent.write_file(
        work, "datasets/bbbbbbbb-0000-0000-0000-000000000000", "cli-eval-abc"
    )
    assert unsafe(
        ["api", "datasets/bbbbbbbb-0000-0000-0000-000000000000", "-X", "DELETE"]
    )
    # `api -f` is a raw field, not a file, so a file named like the field is ignored.
    field = "session_id=bbbbbbbb-0000-0000-0000-000000000000"
    agent.write_file(work, field, "cli-eval-abc")
    assert unsafe(["api", "runs/rules", "-X", "POST", "-f", field])
    # An output file is written, not sent, so its contents can't vouch either.
    agent.write_file(work, "out.json", "cli-eval-abc")
    assert unsafe(
        ["datasets", "delete", "someone-elses-dataset", "--yes", "--output=out.json"]
    )
    assert unsafe(
        ["datasets", "delete", "someone-elses-dataset", "--yes", "-o", "out.json"]
    )
    agent.write_file(work, "someone-elses-dataset", "cli-eval-abc")
    assert unsafe(["datasets", "delete", "someone-elses-dataset", "--yes"])
