"""Run one task with Claude driving the `langsmith` CLI, and record what it did.

The model gets one tool that runs `langsmith` with an argument list. There is no
shell, so the agent cannot pipe, read files, or call the API with curl; it can only
use what the CLI offers. Every call is checked against the help tree, so commands
and flags the binary does not document are counted as made up even when the CLI
happens to print help and exit 0.
"""

from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import tempfile
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any

import anthropic

from . import surface

MAX_TOOL_OUTPUT = (
    30_000  # characters returned to the model, like a coding agent's shell tool
)
CALL_TIMEOUT_S = 120

SYSTEM = (
    "You are a coding agent. Your tools run the `langsmith` command-line tool with the arguments you give, "
    "and write files into the directory it runs in, for flags that take a file. "
    "There is no shell, so pipes and other programs are unavailable; use the CLI's own options instead. "
    "Work step by step, verify your results, and do not change anything the task does not ask for."
)
API_ONLY_NOTE = " For this run you may only use `langsmith api`, which makes raw authenticated requests to the LangSmith REST API."
TOOL = {
    "name": "langsmith",
    "description": (
        'Run `langsmith` with these arguments, e.g. ["datasets", "list", "--help"]. '
        "Returns stdout, stderr, and the exit code."
    ),
    "input_schema": {
        "type": "object",
        "properties": {
            "args": {
                "type": "array",
                "items": {"type": "string"},
                "description": "Arguments after `langsmith`",
            }
        },
        "required": ["args"],
        "additionalProperties": False,
    },
}
WRITE_TOOL = {
    "name": "write_file",
    "description": (
        "Write a text file in the working directory `langsmith` runs in, e.g. a JSON file "
        "for a flag that takes a path. The path is relative to that directory."
    ),
    "input_schema": {
        "type": "object",
        "properties": {"path": {"type": "string"}, "content": {"type": "string"}},
        "required": ["path", "content"],
        "additionalProperties": False,
    },
}
MAX_FILE_BYTES = 1_000_000
SKILL_TOOL = {
    "name": "read_skill",
    "description": "Read the full instructions of one of the skills listed in the system prompt.",
    "input_schema": {
        "type": "object",
        "properties": {"name": {"type": "string"}},
        "required": ["name"],
        "additionalProperties": False,
    },
}
_UNKNOWN = re.compile(
    r"unknown (command|flag|shorthand flag)|flag provided but not defined|No help topic",
    re.IGNORECASE,
)


@dataclass
class Call:
    argv: list[str]
    exit: int
    ms: int
    stdout_bytes: int
    stderr: str
    command: str
    made_up: bool
    unknown: list[str]
    is_help: bool
    blocked: bool = False
    unsafe_write: bool = (
        False  # a write or delete outside this run's resources, refused
    )


@dataclass
class Transcript:
    task_id: str
    model: str
    final_text: str = ""
    calls: list[Call] = field(default_factory=list)
    mcp_calls: int = 0
    skill_reads: int = 0
    file_writes: int = 0
    turns: int = 0
    input_tokens: int = 0  # uncached + cache reads + cache writes
    cache_read_tokens: int = 0
    output_tokens: int = 0
    wall_s: float = 0.0
    stop_reason: str = ""
    error: str | None = None

    def to_json(self) -> dict[str, Any]:
        return asdict(self)


@dataclass
class AgentConfig:
    model: str
    binary: str
    tree: dict[str, surface.Command]
    effort: str = "medium"
    max_turns: int = 40
    api_only: bool = False
    skills_dir: str | None = None
    mcp_url: str | None = None
    mcp_token: str | None = None


def cli_env() -> dict[str, str]:
    """Environment for CLI calls: LangSmith credentials only, no model-provider keys."""
    env = {
        k: v
        for k, v in os.environ.items()
        if not k.startswith(("ANTHROPIC_", "OPENAI_", "EVAL_"))
    }
    env["NO_COLOR"] = "1"
    env.pop("LANGSMITH_PROJECT", None)  # tasks name their project explicitly
    return env


_WRITE_WORD = re.compile(
    r"delete|remove|update|replace|clear|purge|secrets", re.IGNORECASE
)
_API_BODY_FLAGS = {"--body", "--input", "-F", "--field", "-f", "--raw-field"}


_API_VALUE_FLAGS = {
    "-X",
    "--method",
    "-H",
    "--header",
    "--jq",
    "--format",
    *_API_BODY_FLAGS,
}


def _api_path(args: list[str]) -> str:
    """The endpoint path of a `langsmith api` call: its first positional argument."""
    i = 1
    while i < len(args):
        a = args[i]
        if a in _API_VALUE_FLAGS:
            i += 2
            continue
        if not a.startswith("-"):
            return a
        i += 1
    return ""


def is_write(chk: surface.CallCheck, args: list[str]) -> bool:
    """Whether a call deletes or changes existing resources (creating new ones is fine).

    Some endpoints delete or update through POST (bulk deletes, `.../delete`), so a
    command or API path that names such an action counts whatever its method.
    """
    words = chk.command.split(" ")
    if words[0] == "api":
        method = _method(args)
        if method in ("DELETE", "PATCH", "PUT"):
            return True
        if method == "GET":
            return False
        return bool(_WRITE_WORD.search(_api_path(args).split("?", 1)[0]))
    return any(_WRITE_WORD.search(w) for w in words)


_UUID = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")


def _created_id(stdout: str) -> list[str]:
    """The created object's ID: the top-level `id` of a JSON response, or the first
    ID a create command prints in its table output."""
    try:
        value = json.loads(stdout)
    except ValueError:
        return _UUID.findall(stdout)[:1]
    if (
        isinstance(value, dict)
        and isinstance(value.get("id"), str)
        and _UUID.fullmatch(value["id"])
    ):
        return [value["id"]]
    return []


_READ_PATH = re.compile(
    r"query|search|stats|group|aggregate|count|filter|export|compare|preview|diff",
    re.IGNORECASE,
)


def _method(args: list[str]) -> str | None:
    """The HTTP method of a `langsmith api` call, in any of its spellings."""
    for i, a in enumerate(args):
        if a in ("-X", "--method") and i + 1 < len(args):
            return args[i + 1].upper()
        if a.startswith(("--method=", "-X=")):
            return a.split("=", 1)[1].upper()
        if a.startswith("-X") and len(a) > 2:
            return a[2:].upper()
    return None


def _creates(chk: surface.CallCheck, args: list[str]) -> bool:
    """Whether a call creates something (a POST or a create command), not a POST read."""
    words = chk.command.split(" ")
    if words[0] == "api":
        method = _method(args)
        posts = (method or "").upper() == "POST" or (
            method is None and any(a.split("=", 1)[0] in _API_BODY_FLAGS for a in args)
        )
        return posts and not _READ_PATH.search(_api_path(args).split("?", 1)[0])
    return any(w.startswith("create") or w in ("push", "upload") for w in words)


# Flags that only shape output or pick credentials; they never name the target of a
# write, so they cannot vouch for one.
_OUTPUT_FLAGS = {
    "--jq",
    "--format",
    "--output",
    "-o",
    "--transform",
    "--transform-error",
    "--format-error",
    "-H",
    "--header",
    "--limit",
    "--workspace",
    "--api-url",
    "--profile",
}


def _target_args(args: list[str]) -> list[str]:
    out, i = [], 0
    while i < len(args):
        name = args[i].split("=", 1)[0]
        if name in _OUTPUT_FLAGS:
            i += 1 if "=" in args[i] else 2
            continue
        out.append(args[i])
        i += 1
    return out


# Commands that act on the runner itself rather than on LangSmith resources.
_HOST_COMMANDS = {
    "self-update",
    "auth",
    "profiles",
    "connector",
    "relay",
    "tunnel",
    "sandboxes",
    "apps",
    "completion",
    "traces setup",
}
# Flags that pick credentials, where they are sent, or which workspace they act in,
# or that run commands: always refused, so the key only ever goes to the configured
# endpoint and workspace.
_CREDENTIAL_FLAGS = {
    "--api-key-cmd",
    "--api-key-env",
    "--keychain",
    "--profile",
    "--api-url",
    "--base-url",
    "--workspace",
}
# Flags that read or write local files. The crawl adds every flag whose help says so
# (per command, so a short flag like `-f` only counts where it means a file); this
# list is a backstop of long flags the agent is most likely to reach for.
_FILE_FLAGS = {
    "--input",
    "--file",
    "--function",
    "--input-attachments",
    "--output-attachments",
    "--dir",
    "--path",
    "--output",
    "-o",
    "--config",
    "--prompt",
    "--schema",
    "--model-config",
    "--readme",
    "--filename-pattern",
}


def _inside(workdir: str | None, value: str) -> bool:
    """Whether a path the CLI would open resolves inside the attempt's directory."""
    if value in ("", "-"):
        return True  # stdin, which is empty
    if workdir is None:
        return False
    root = os.path.realpath(workdir)
    path = os.path.realpath(os.path.join(root, value))
    return path == root or path.startswith(root + os.sep)


def host_access(
    tree: dict[str, surface.Command],
    chk: surface.CallCheck,
    args: list[str],
    workdir: str | None,
) -> bool:
    """Whether a call would reach the runner outside the attempt's directory: run
    commands, use other credentials, or open a path such as `/proc/<pid>/environ`
    through a file flag, an `@file` value or a file argument."""
    if any(chk.command == c or chk.command.startswith(c + " ") for c in _HOST_COMMANDS):
        return True
    path = chk.command.split(" ") if chk.command else []
    cmd = tree.get(chk.command)
    file_flags = _FILE_FLAGS | set(cmd.file_flags if cmd else ())
    positional: list[str] = []
    i = 0
    while i < len(args):
        a = args[i]
        if a.startswith("-") and a != "-":
            name, eq, value = a.partition("=")
            if name in _CREDENTIAL_FLAGS:
                return True
            if not eq and surface._takes_value(tree, path, name) and i + 1 < len(args):
                value = args[i + 1]
                i += 1
            if name in file_flags and not _inside(workdir, value):
                return True
        else:
            value = a
            positional.append(a)
        # `@path` and `key=@path` values read files on any flag (--body, -F, ...).
        if value.startswith("@") and not _inside(workdir, value[1:]):
            return True
        if "=@" in value and not _inside(workdir, value.split("=@", 1)[1]):
            return True
        i += 1
    if cmd and cmd.file_args:
        return any(not _inside(workdir, p) for p in positional[len(path) :])
    return False


def _file_text(workdir: str | None, value: str) -> str | None:
    """The contents of a file in the attempt's directory that a value names, if any."""
    if workdir is None or not value or not _inside(workdir, value):
        return None
    path = os.path.join(os.path.realpath(workdir), value)
    if not os.path.isfile(path):
        return None
    with open(path, errors="replace") as f:
        return f.read(MAX_FILE_BYTES)


def _file_refs(
    tree: dict[str, surface.Command], chk: surface.CallCheck, args: list[str]
) -> dict[int, str]:
    """Where a call names a file the CLI will read: arg index -> the path. Only the
    places the CLI opens a file count (a file flag's value, `@file`, `key=@file`, a
    file argument), never an endpoint path or other positional argument."""
    path = chk.command.split(" ") if chk.command else []
    cmd = tree.get(chk.command)
    file_flags = _FILE_FLAGS | set(cmd.file_flags if cmd else ())
    refs: dict[int, str] = {}
    positional = 0
    i = 0
    while i < len(args):
        a = args[i]
        if a.startswith("-") and a != "-":
            name, eq, value = a.partition("=")
            idx = i
            if not eq and surface._takes_value(tree, path, name) and i + 1 < len(args):
                idx, value = i + 1, args[i + 1]
                i += 1
            if name in _OUTPUT_FLAGS:
                pass  # output files are written, not sent; dropped from the check
            elif value.startswith("@"):
                refs[idx] = value[1:]
            elif "=@" in value:
                refs[idx] = value.split("=@", 1)[1]
            elif name in file_flags:
                refs[idx] = value
        else:
            positional += 1
            if a.startswith("@"):
                refs[i] = a[1:]
            elif cmd and cmd.file_args and positional > len(path):
                refs[i] = a
        i += 1
    return refs


def _expand_files(
    tree: dict[str, surface.Command],
    chk: surface.CallCheck,
    args: list[str],
    workdir: str | None,
) -> list[str]:
    """Replace each file the CLI will read with its contents, so the scope check
    judges what is sent rather than the file's name."""
    out = list(args)
    for idx, ref in _file_refs(tree, chk, args).items():
        text = _file_text(workdir, ref)
        if text is not None:
            out[idx] = text
    return out


def in_scope(
    args: list[str],
    scope: list[str],
    workdir: str | None = None,
    tree: dict[str, surface.Command] | None = None,
    chk: surface.CallCheck | None = None,
) -> bool:
    """A write is in scope when every ID it names is the run's, or, if it names no
    ID, when it names the run's prefix. One known ID cannot vouch for foreign ones.
    Files the call sends are judged by their contents."""
    if tree is not None and chk is not None:
        args = _expand_files(tree, chk, args, workdir)
    joined = " ".join(_target_args(args))
    ids = set(_UUID.findall(joined.lower()))
    if ids:
        return ids <= {t.lower() for t in scope}
    return any(token in joined for token in scope)


def run_cli(
    cfg: AgentConfig,
    args: Any,
    scope: list[str] | None = None,
    workdir: str | None = None,
) -> tuple[str, bool, Call | None]:
    """Run one CLI call in `workdir`. Calls that would reach the runner outside it
    (other files, commands, credentials, CLI config) are refused. With `scope`, a create, change or delete must target the
    run's prefix or its IDs; others are refused before they reach the API, so a
    mistaken agent cannot damage resources that other users of the workspace own.
    """
    if not isinstance(args, list) or not all(isinstance(a, str) for a in args):
        return "error: args must be a list of strings", True, None
    chk = surface.check_call(cfg.tree, args)
    if not chk.is_help and host_access(cfg.tree, chk, args, workdir):
        msg = (
            "refused: files must be inside the working directory, and commands that run"
            " programs or change CLI config or credentials are not allowed"
        )
        call = Call(args, 2, 0, 0, msg, chk.command, False, [], False, True)
        return f"error: {msg}", True, call
    guarded = is_write(chk, args) or _creates(chk, args)
    if (
        scope
        and not chk.is_help
        and guarded
        and not in_scope(args, scope, workdir, cfg.tree, chk)
    ):
        msg = (
            "refused: this run may only create, change or delete resources under its own"
            " prefix or IDs"
        )
        call = Call(args, 2, 0, 0, msg, chk.command, False, [], False, True, True)
        return f"error: {msg}", True, call
    if cfg.api_only and chk.command.split(" ")[0] != "api" and not chk.is_help:
        call = Call(
            args,
            2,
            0,
            0,
            "blocked: only `langsmith api` is allowed in this run",
            chk.command,
            False,
            [],
            False,
            True,
        )
        return "error: only `langsmith api ...` is allowed in this run", True, call
    start = time.time()
    try:
        p = subprocess.run(
            [cfg.binary, *args],
            capture_output=True,
            text=True,
            timeout=CALL_TIMEOUT_S,
            env=cli_env(),
            stdin=subprocess.DEVNULL,
            cwd=workdir,
            check=False,
        )
        code, out, err = p.returncode, p.stdout, p.stderr
    except subprocess.TimeoutExpired:
        code, out, err = 124, "", f"timed out after {CALL_TIMEOUT_S}s"
    ms = int((time.time() - start) * 1000)
    unknown = [x for x in [chk.unknown_command, *chk.unknown_flags] if x]
    # A failed call whose error names an unknown command or flag is made up even if
    # the static check missed it (hidden commands, positional quirks).
    printed_help = "Usage:" in out or "USAGE:" in out
    made_up = bool(
        (chk.unknown_command and (code != 0 or printed_help))
        or (code != 0 and (chk.unknown_flags or _UNKNOWN.search(err)))
    )
    call = Call(
        args, code, ms, len(out), err[:500], chk.command, made_up, unknown, chk.is_help
    )
    if scope is not None and code == 0 and _creates(chk, args):
        # What the agent creates in this task is its own to change or delete. Only the
        # created object's own ID counts: a POST query returns other resources' IDs too.
        scope.extend(_created_id(out))
    text = f"exit code: {code}\nstdout:\n{out}\nstderr:\n{err}"
    if len(text) > MAX_TOOL_OUTPUT:
        text = (
            text[:MAX_TOOL_OUTPUT]
            + f"\n[output truncated: {len(text) - MAX_TOOL_OUTPUT} more characters]"
        )
    return text, code != 0, call


def write_file(workdir: str, path: Any, content: Any) -> tuple[str, bool]:
    if not isinstance(path, str) or not isinstance(content, str):
        return "error: path and content must be strings", True
    if len(content.encode()) > MAX_FILE_BYTES:
        return f"error: files are limited to {MAX_FILE_BYTES} bytes", True
    if not path or os.path.isabs(path) or not _inside(workdir, path):
        return "error: path must be relative and inside the working directory", True
    dest = os.path.join(os.path.realpath(workdir), path)
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    with open(dest, "w") as f:
        f.write(content)
    return f"wrote {len(content)} characters to {path}", False


def load_skills(skills_dir: str) -> dict[str, tuple[str, str]]:
    """name -> (description, full text) for every SKILL.md under skills_dir."""
    skills = {}
    for path in sorted(Path(skills_dir).rglob("SKILL.md")):
        text = path.read_text()
        name = re.search(r"^name:\s*(.+)$", text, re.MULTILINE)
        desc = re.search(r"^description:\s*(.+)$", text, re.MULTILINE)
        key = (name.group(1) if name else path.parent.name).strip().strip("'\"")
        skills[key] = ((desc.group(1) if desc else "").strip().strip("'\""), text)
    return skills


def run_task(
    client: anthropic.Anthropic,
    cfg: AgentConfig,
    task_id: str,
    prompt: str,
    scope: list[str] | None = None,
) -> Transcript:
    tr = Transcript(task_id=task_id, model=cfg.model)
    system = SYSTEM + (API_ONLY_NOTE if cfg.api_only else "")
    tools: list[dict[str, Any]] = [TOOL, WRITE_TOOL]
    # Each attempt gets an empty directory: the CLI runs in it, and every file a call
    # names must resolve inside it.
    workdir = tempfile.mkdtemp(prefix="cli-eval-work-")
    skills = load_skills(cfg.skills_dir) if cfg.skills_dir else {}
    if skills:
        system += "\n\nSkills you can read with `read_skill`:\n" + "\n".join(
            f"- {n}: {d}" for n, (d, _) in skills.items()
        )
        tools.append(SKILL_TOOL)
    extra: dict[str, Any] = {}
    if cfg.mcp_url:
        server = {"type": "url", "url": cfg.mcp_url, "name": "langsmith"}
        if cfg.mcp_token:
            server["authorization_token"] = cfg.mcp_token
        tools.append({"type": "mcp_toolset", "mcp_server_name": "langsmith"})
        extra = {"mcp_servers": [server], "betas": ["mcp-client-2025-11-20"]}
    create = client.beta.messages.create if cfg.mcp_url else client.messages.create

    messages: list[dict[str, Any]] = [{"role": "user", "content": prompt}]
    start = time.time()
    try:
        for turn in range(1, cfg.max_turns + 1):
            tr.turns = turn
            resp = create(
                model=cfg.model,
                max_tokens=16000,
                system=system,
                tools=tools,
                messages=messages,
                output_config={"effort": cfg.effort},
                cache_control={
                    "type": "ephemeral"
                },  # each turn reads the previous turns from cache
                **extra,
            )
            u = resp.usage
            cache_read = getattr(u, "cache_read_input_tokens", 0) or 0
            cache_write = getattr(u, "cache_creation_input_tokens", 0) or 0
            tr.input_tokens += (u.input_tokens or 0) + cache_read + cache_write
            tr.cache_read_tokens += cache_read
            tr.output_tokens += u.output_tokens or 0
            tr.stop_reason = resp.stop_reason or ""
            # Pass the content back unchanged: thinking blocks must round-trip as received.
            messages.append({"role": "assistant", "content": resp.content})
            tr.mcp_calls += sum(1 for b in resp.content if b.type == "mcp_tool_use")
            text = "".join(b.text for b in resp.content if b.type == "text")
            if text:
                tr.final_text = text
            if resp.stop_reason == "refusal":
                tr.error = "model refused"
                break
            uses = [b for b in resp.content if b.type == "tool_use"]
            if not uses:
                break
            results = []
            for b in uses:
                if b.name == "write_file":
                    tr.file_writes += 1
                    out, is_err = write_file(
                        workdir,
                        (b.input or {}).get("path"),
                        (b.input or {}).get("content"),
                    )
                elif b.name == "read_skill":
                    tr.skill_reads += 1
                    name = (b.input or {}).get("name", "")
                    out, is_err = (
                        (skills[name][1], False)
                        if name in skills
                        else (f"no skill named {name!r}", True)
                    )
                else:
                    out, is_err, call = run_cli(
                        cfg, (b.input or {}).get("args"), scope, workdir
                    )
                    if call:
                        tr.calls.append(call)
                results.append(
                    {
                        "type": "tool_result",
                        "tool_use_id": b.id,
                        "content": out,
                        "is_error": is_err,
                    }
                )
            messages.append({"role": "user", "content": results})
        else:
            tr.error = f"stopped after {cfg.max_turns} turns"
    except anthropic.APIError as e:  # recorded, not raised, so the task is still scored
        tr.error = f"{type(e).__name__}: {e}"
    finally:
        shutil.rmtree(workdir, ignore_errors=True)
    tr.wall_s = round(time.time() - start, 1)
    return tr
