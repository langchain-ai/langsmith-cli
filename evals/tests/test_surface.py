from agent_evals import surface

COBRA = """Manage feedback

Usage:
  langsmith feedback [flags]
  langsmith feedback [command]

Aliases:
  feedback, fb

Available Commands:
  delete      Delete a feedback.
  resolve     Returns the project an address names. An address is an AGENT
(`id` and `environment`), an EXPERIMENT (`id`).

Flags:
  -h, --help           help for feedback
  -l, --limit int      max rows
      --yes            skip confirmation

Global Flags:
      --format string   Output format
"""

URFAVE = """NAME:
   langsmith online-evaluators create - Create a new evaluator.

USAGE:
   langsmith online-evaluators create [options]

OPTIONS:
   --code-evaluator string=any [ --code-evaluator string=any ]  (default: {}
      )
   --name string
   --help, -h     show help

GLOBAL OPTIONS:
   --raw-output, -r          If the result is a string, print it without quotes.
"""


def test_parse_cobra_page():
    children, aliases, flags, runnable, accepts_args = surface.parse_help(COBRA)
    # Wrapped description lines start at column 0 and must not become commands.
    assert children == ["delete", "resolve"]
    assert aliases == ["feedback", "fb"]
    assert flags["--limit"] is True and flags["-l"] is True
    assert flags["--yes"] is False and flags["--format"] is True
    assert runnable and not accepts_args


def test_parse_generated_page():
    children, _, flags, runnable, _ = surface.parse_help(URFAVE)
    assert children == []
    assert flags["--code-evaluator"] is True and flags["--name"] is True
    assert flags["--help"] is False and flags["-h"] is False
    assert flags["--raw-output"] is False and flags["-r"] is False
    assert runnable


def test_real_commands_are_known(tree):
    for argv in (
        ["datasets", "list", "--limit", "5"],
        ["dataset", "list"],  # alias
        ["runs", "list", "--project", "x", "--error", "--format", "json"],
        ["traces", "list", "--project=x", "--last-n-minutes", "60", "--jq", ".[0]"],
        ["online-evaluators", "create", "--name", "x", "--type", "llm"],
        [
            "api",
            "runs/rules?session_id=x",
            "-X",
            "GET",
        ],  # raw API: the path is positional
        ["datasets", "--help"],
    ):
        chk = surface.check_call(tree, argv)
        assert not chk.made_up, (argv, chk)


def test_made_up_commands_and_flags(tree):
    chk = surface.check_call(tree, ["datasets", "lst"])
    assert chk.unknown_command == "datasets lst"
    chk = surface.check_call(tree, ["feedback", "create", "--run-id", "x"])
    assert chk.unknown_command == "feedback create"
    chk = surface.check_call(tree, ["runs", "list", "--errors-only"])
    assert chk.unknown_flags == ["--errors-only"] and chk.command == "runs list"
    chk = surface.check_call(tree, ["bogus"])
    assert chk.unknown_command == "bogus"


def test_flag_values_are_not_read_as_commands(tree):
    # "list" is the value of --name, not a subcommand.
    chk = surface.check_call(tree, ["datasets", "create", "--name", "list"])
    assert chk.command == "datasets create" and not chk.made_up


def test_help_detection(tree):
    assert surface.check_call(tree, ["runs", "list", "-h"]).is_help
    assert not surface.check_call(tree, ["runs", "list"]).is_help


def test_tree_round_trips(tree, tmp_path):
    path = tmp_path / "tree.json"
    surface.save(tree, str(path))
    loaded = surface.load(str(path))
    assert surface.check_call(
        loaded, ["runs", "list", "--error"]
    ) == surface.check_call(tree, ["runs", "list", "--error"])
