"""The CLI's command surface, read from `langsmith --help`, and checks of agent calls against it.

The tree is built by walking `--help` output, so it always matches the binary under
test: a command or flag the agent uses that the binary does not document counts as
made up.
"""

from __future__ import annotations

import json
import re
import subprocess
from dataclasses import dataclass, field

_SECTION = re.compile(r"^([A-Za-z][A-Za-z ]+):\s*$")
# cobra:   "  -p, --project string   desc" / "      --yes   desc"
# urfave:  "   --name string  desc" / "   --help, -h  show help" / "   --x string=any [ ... ]"
_COBRA_FLAG = re.compile(
    r"^\s+(?:-(?P<short>\w), )?--(?P<long>[\w-]+)(?: (?P<type>[\w.\[\]=]+))?(?:\s{2,}|$)"
)
_URFAVE_FLAG = re.compile(
    r"^\s+--(?P<long>[\w-]+)(?:, -(?P<short>\w))?(?: (?P<type>[\w.=]+))?(?:\s|$)"
)
_GROUP_SECTIONS = {"available commands", "additional commands", "commands"}
_FLAG_SECTIONS = {"flags", "global flags", "options", "global options"}
_USAGE_NOISE = {
    "[flags]",
    "[command]",
    "[options]",
    "[command",
    "options]]",
    "[command options]]",
}


@dataclass
class Command:
    path: list[str]
    children: dict[str, str] = field(
        default_factory=dict
    )  # name or alias -> canonical name
    flags: dict[str, bool] = field(
        default_factory=dict
    )  # "--name" / "-n" -> takes a value
    runnable: bool = True
    accepts_args: bool = False
    # Flags whose help says they read or write local files, run commands or read
    # stdin, and whether a positional argument is a local path (OUTPUT_FILE, DIR).
    file_flags: list[str] = field(default_factory=list)
    file_args: bool = False

    def to_json(self) -> dict:
        return {
            "path": self.path,
            "children": self.children,
            "flags": self.flags,
            "runnable": self.runnable,
            "accepts_args": self.accepts_args,
            "file_flags": self.file_flags,
            "file_args": self.file_args,
        }


def parse_help(text: str) -> tuple[list[str], list[str], dict[str, bool], bool, bool]:
    """Parse one `--help` page (cobra or the generated commands' format).

    Returns (subcommand names, aliases, flags, runnable, accepts_args); flags map
    each spelling to whether it takes a value.
    """
    section = None
    children: list[str] = []
    aliases: list[str] = []
    flags: dict[str, bool] = {}
    usage: list[str] = []
    for line in text.splitlines():
        m = _SECTION.match(line)
        if m:
            section = m.group(1).strip().lower()
            continue
        if not line.strip() or section is None:
            continue
        if section == "usage":
            usage.append(line.strip())
        elif section == "aliases":
            aliases += [a.strip() for a in line.split(",") if a.strip()]
        elif section in _GROUP_SECTIONS or section.endswith(" commands"):
            name = line.split()[0]
            # Wrapped description lines are indented further than command names.
            indent = len(line) - len(line.lstrip())
            if 1 <= indent <= 3 and re.fullmatch(r"[\w-]+", name):
                children.append(name.rstrip(":"))
        elif section in _FLAG_SECTIONS:
            fm = (
                _COBRA_FLAG.match(line)
                if not re.match(r"^\s+--[\w-]+, -\w", line)
                else None
            )
            fm = fm or _URFAVE_FLAG.match(line)
            if fm:
                takes_value = bool(fm.group("type"))
                flags["--" + fm.group("long")] = takes_value
                if fm.group("short"):
                    flags["-" + fm.group("short")] = takes_value
    groupish = [
        u
        for u in usage
        if u.endswith("[command]") or "[command [command options]]" in u
    ]
    runnable = not usage or len(groupish) < len(usage)
    accepts_args = False
    for u in usage:
        if u in groupish:
            continue
        # Tokens after the command path (which is all lowercase words) that are not
        # flag placeholders mean the command takes positional arguments.
        extra = [
            t
            for t in u.split()[1:]
            if not re.fullmatch(r"[a-z][\w-]*", t) and t not in _USAGE_NOISE
        ]
        if extra:
            accepts_args = True
    return children, aliases, flags, runnable, accepts_args


_FILE_HELP = re.compile(
    r"\bfiles?\b|\bpath\b|\bdirectory\b|\bdir\b|stdin|keychain|\bcommand\b|@<path>|@file",
    re.IGNORECASE,
)
_FILE_USAGE = re.compile(r"FILE|DIR\b|PATH|<path>")


def parse_file_access(text: str) -> tuple[list[str], bool]:
    """Flags whose description touches the local machine, and whether the usage line
    takes a local path as a positional argument, from one `--help` page."""
    section = None
    flags: list[str] = []
    file_args = False
    for line in text.splitlines():
        m = _SECTION.match(line)
        if m:
            section = m.group(1).strip().lower()
            continue
        if section == "usage" and line.strip():
            usage = line.strip()
            if not usage.startswith("langsmith api ") and _FILE_USAGE.search(
                re.sub(r"--[\w-]+ \S+", "", usage)
            ):
                file_args = True
        elif section in _FLAG_SECTIONS:
            fm = re.match(
                r"^\s+(?:-(\w), )?--([\w-]+)(?:, -(\w))?(?: \S+)?\s{2,}(.*)$", line
            )
            # A flag whose help offers `@file` takes inline values; its `@` form is
            # checked wherever it appears, so only plain file flags are listed here.
            if fm and _FILE_HELP.search(fm.group(4)) and "@" not in fm.group(4):
                flags.append("--" + fm.group(2))
                flags += ["-" + x for x in (fm.group(1), fm.group(3)) if x]
    return flags, file_args


def crawl(binary: str = "langsmith", max_depth: int = 4) -> dict[str, Command]:
    """Walk `<binary> ... --help` and return commands keyed by space-joined path."""
    tree: dict[str, Command] = {}

    def visit(path: list[str]) -> list[str]:
        out = subprocess.run(
            [binary, *path, "--help"],
            capture_output=True,
            text=True,
            timeout=30,
            stdin=subprocess.DEVNULL,
            check=False,
        )
        children, aliases, flags, runnable, accepts_args = parse_help(out.stdout)
        flags.setdefault("--help", False)
        flags.setdefault("-h", False)
        file_flags, file_args = parse_file_access(out.stdout)
        cmd = Command(
            path=path,
            flags=flags,
            runnable=runnable,
            accepts_args=accepts_args,
            file_flags=file_flags,
            file_args=file_args,
        )
        tree[" ".join(path)] = cmd
        if len(path) < max_depth:
            for child in children:
                if child in ("completion",) or (child == "help" and path == []):
                    cmd.children[child] = child
                    continue
                for alias in visit([*path, child]):
                    cmd.children[alias] = child
                cmd.children[child] = child
        return aliases

    visit([])
    return tree


def save(tree: dict[str, Command], path: str) -> None:
    with open(path, "w") as f:
        json.dump(
            {k: v.to_json() for k, v in tree.items()}, f, indent=1, sort_keys=True
        )


def load(path: str) -> dict[str, Command]:
    with open(path) as f:
        raw = json.load(f)
    return {k: Command(**v) for k, v in raw.items()}


@dataclass
class CallCheck:
    command: str  # resolved command path, e.g. "datasets create"
    unknown_command: str | None = None
    unknown_flags: list[str] = field(default_factory=list)
    is_help: bool = False

    @property
    def made_up(self) -> bool:
        return bool(self.unknown_command or self.unknown_flags)


def check_call(tree: dict[str, Command], argv: list[str]) -> CallCheck:
    """Resolve argv (without the leading `langsmith`) against the tree.

    Follows cobra's rules closely enough for scoring: subcommands are matched until
    the first positional argument, and every flag must exist on the resolved command
    or as a global flag. `langsmith api` takes an endpoint path, so only its subcommands
    and flags are checked.
    """
    node = tree[""]
    path: list[str] = []
    check = CallCheck(command="")
    i = 0
    positional_seen = False
    flags_to_check: list[str] = []
    while i < len(argv):
        tok = argv[i]
        if tok == "--":
            break
        if tok.startswith("-") and tok != "-":
            name = tok.split("=", 1)[0]
            flags_to_check.append(name)
            if "=" not in tok and _takes_value(tree, path, name):
                i += 1
            i += 1
            continue
        if not positional_seen and tok in node.children:
            path.append(node.children[tok])
            node = tree.get(" ".join(path), Command(path=list(path)))
            i += 1
            continue
        if (
            not positional_seen
            and node.children
            and not node.accepts_args
            and path != ["api"]
        ):
            # A group given a word that is not one of its subcommands: cobra either
            # errors or prints help and exits 0. Either way the agent made it up.
            check.unknown_command = " ".join([*path, tok])
        positional_seen = True
        i += 1
    check.command = " ".join(path)
    check.is_help = any(f in ("--help", "-h") for f in flags_to_check) or (
        path[:1] == ["help"]
    )
    if path[:1] == ["help"]:
        return check
    for name in flags_to_check:
        if not _known_flag(tree, path, name):
            check.unknown_flags.append(name)
    return check


def _scopes(tree: dict[str, Command], path: list[str]):
    # Flags of the command itself, then persistent flags shown as "Global Flags" on it.
    cmd = tree.get(" ".join(path))
    if cmd:
        yield cmd.flags
    yield tree[""].flags


def _known_flag(tree, path, name) -> bool:
    return any(name in flags for flags in _scopes(tree, path))


def _takes_value(tree, path, name) -> bool:
    for flags in _scopes(tree, path):
        if name in flags:
            return flags[name]
    return False
