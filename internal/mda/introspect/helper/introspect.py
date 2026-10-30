# The Python introspection helper of `langsmith deploy`.
#
# Run with the project interpreter, isolated (`-I`), from the project root.
# Reads one JSON request on stdin, imports the project's modules, and writes one
# JSON document describing the values they export to the file the request names
# in `output`.
#
# The document goes to a file, not stdout: a project module may write to file
# descriptor 1 directly, which nothing in this process can keep out of stdout.
# What it prints through `sys.stdout` goes to stderr. A module that fails to
# import is reported in its section, never read as "not configured".
import importlib
import importlib.metadata
import json
import math
import os
import sys
import traceback

VERSION = 1

UNSUPPORTED_AGENT_KEYS = (
    "backend",
    "store",
    "checkpointer",
    "instructions",
    "mcp",
    "memory",
    "sandbox",
    "skills",
    "system_prompt",
    "state_schema",
)
SANDBOX_KEYS = (
    "snapshot_name",
    "snapshot_id",
    "docker_image",
    "registry",
    "fs_capacity_bytes",
    "scope",
)
INGRESS_PROVIDERS = frozenset({"slack", "github", "linear"})
MISSING = object()


class NotJson(Exception):
    pass


def main():
    sys.stdout = sys.stderr
    code = 1
    try:
        request = json.loads(sys.stdin.buffer.read())
        output = request["output"]
        try:
            document = introspect_project(request)
        except BaseException as error:  # noqa: BLE001 - reported in the document
            document = {"error": {"kind": "helper", "message": error_text(error)}}
        with open(output, "w", encoding="utf-8") as out:
            out.write(encode(document))
        code = 0
    except BaseException:  # noqa: BLE001 - the helper's own failure, for the CLI to show
        traceback.print_exc()
    finally:
        sys.stderr.flush()
        # Threads a project module started must not keep the helper alive.
        os._exit(code)


def encode(document):
    try:
        return json.dumps({"version": VERSION, **document}) + "\n"
    except (TypeError, ValueError) as error:
        failure = {"kind": "helper", "message": f"the document is not JSON: {error_text(error)}"}
        return json.dumps({"version": VERSION, "error": failure}) + "\n"


def introspect_project(request):
    root = request["root"]
    sys.path.insert(0, root)
    import managed_deepagents as mda

    kinds = {
        "factory": getattr(mda, "DeepAgentFactory", None),
        "static": getattr(mda, "DeepAgentDefinition", None),
    }

    def load(rel):
        if rel is None:
            return None
        try:
            return {"rel": rel, "module": importlib.import_module(module_name(rel))}
        except SyntaxError as error:
            if error.filename and os.path.abspath(error.filename) == os.path.join(root, rel):
                return {
                    "rel": rel,
                    "error": {
                        "kind": "syntax",
                        "message": error.msg,
                        "line": error.lineno or 1,
                        "column": error.offset or 1,
                    },
                }
            return {"rel": rel, "error": import_error(error)}
        except BaseException as error:  # noqa: BLE001 - reported in the document
            return {"rel": rel, "error": import_error(error)}

    def section(loaded, read):
        if loaded is None:
            return None
        if "error" in loaded:
            return failed(loaded["rel"], loaded["error"])
        try:
            return succeeded(loaded["rel"], read(loaded["module"]))
        except BaseException as error:  # noqa: BLE001 - reported in the document
            return failed(loaded["rel"], evaluate_error(error))

    document = {"sdk": installed_sdk(kinds)}
    document["agent"] = section(load(request["agent"]), lambda m: read_agent(m, kinds))
    document["identity"] = section(
        load(request.get("identity")), lambda m: read_identity(getattr(m, "identity", None))
    )
    document["memory"] = section(
        load(request.get("memory")), lambda m: read_memory(getattr(m, "memory", None))
    )
    document["sandbox"] = section(load(request.get("sandbox")), read_sandbox)
    mcp = load(request.get("mcp"))
    document["mcp"] = section(mcp, lambda m: exports_of(m, "mcp"))
    document["legacy_connectors"] = [
        section(load(rel), lambda m: exports_of(m, "connector"))
        for rel in request["legacy_connectors"]
    ]
    channel_modules = []

    def read_channel(rel, module):
        exported = exports_of(module, "channel")
        if exported["has_export"]:
            channel_modules.append((rel, module))
        channel = getattr(module, "channel", None)
        return {**exported, "ingress": channel_ingress(channel) if exported["has_export"] else None}

    document["channels"] = []
    for rel in request["channels"]:
        document["channels"].append(section(load(rel), lambda m, rel=rel: read_channel(rel, m)))
    document["schedules"] = [section(load(rel), read_schedule) for rel in request["schedules"]]

    if document["mcp"] is not None and ("error" in mcp or hasattr(mcp["module"], "mcp")):
        if "error" in mcp:
            connectors = {"file": mcp["rel"], "error": mcp["error"]}
        else:
            from types import SimpleNamespace

            connectors = collected(
                mcp["rel"],
                "managed_deepagents._connector",
                lambda sdk: sdk.collect_connectors(
                    [(mcp["rel"], SimpleNamespace(connector=mcp["module"].mcp))]
                ),
            )
        document["connector_requirements"] = derive(
            connectors, lambda sdk, items: plain_json(sdk.requirements_from_connectors(items))
        )
        document["mcp_connections"] = derive(connectors, lambda _sdk, items: mcp_connections(items))
    failure = next((entry for entry in document["channels"] if entry["error"] is not None), None)
    if failure is not None:
        document["channel_requirements"] = failed(failure["file"], failure["error"])
    elif channel_modules:
        channels = collected(
            channel_modules[0][0],
            "managed_deepagents._channel",
            lambda sdk: sdk.collect_channels(channel_modules),
        )
        document["channel_requirements"] = derive(
            channels, lambda sdk, items: plain_json(sdk.requirements_from_channels(items))
        )
    return document


def installed_sdk(kinds):
    """The installed ``managed-deepagents``: its version, and whether it has named factories."""
    try:
        version = importlib.metadata.version("managed-deepagents")
    except importlib.metadata.PackageNotFoundError:
        version = None
    return {"version": version, "named_factories": kinds["factory"] is not None}


def succeeded(file, value):
    return {"file": file, "error": None, "value": value}


def failed(file, error):
    return {"file": file, "error": error, "value": None}


def module_name(rel):
    parts = rel[: -len(".py")].split("/") if rel.endswith(".py") else rel.split("/")
    if parts[-1] == "__init__":
        parts = parts[:-1]
    return ".".join(parts)


def collected(file, sdk_module, collect):
    try:
        sdk = importlib.import_module(sdk_module)
    except BaseException as error:  # noqa: BLE001 - reported in the document
        return {
            "file": file,
            "error": {
                "kind": "helper",
                "message": f"cannot import {sdk_module}: {error_text(error)}. "
                "Install the project's dependencies (`uv sync`).",
            },
        }
    try:
        return {"file": file, "sdk": sdk, "items": collect(sdk)}
    except BaseException as error:  # noqa: BLE001 - reported in the document
        return {"file": file, "error": evaluate_error(error)}


def derive(collection, read):
    if "error" in collection:
        return failed(collection["file"], collection["error"])
    try:
        return succeeded(collection["file"], read(collection["sdk"], collection["items"]))
    except BaseException as error:  # noqa: BLE001 - reported in the document
        return failed(collection["file"], evaluate_error(error))


def name_value(name):
    return {"kind": "STATIC", "value": name} if isinstance(name, str) else {"kind": "DYNAMIC"}


def read_agent(module, kinds):
    fields = {
        "kind": "missing",
        "name": {"kind": "ABSENT"},
        "model": None,
        "declares_disable_memory": False,
        "unsupported_keys": [],
        "async_subagents": [],
        "unreadable_subagents": 0,
    }
    agent = getattr(module, "agent", MISSING)
    if agent is MISSING:
        return fields
    if kinds["factory"] is not None and isinstance(agent, kinds["factory"]):
        names = getattr(agent, "async_subagents", ())
        return {
            **fields,
            "kind": "factory",
            "name": name_value(getattr(agent, "name", None)),
            "async_subagents": [name_value(name) for name in names],
        }
    if kinds["static"] is not None and isinstance(agent, kinds["static"]):
        config = agent.config if isinstance(agent.config, dict) else {}
        subagents = config.get("subagents") or ()
        return {
            **fields,
            "kind": "static",
            "name": name_value(config["name"]) if "name" in config else {"kind": "ABSENT"},
            "model": config.get("model") if isinstance(config.get("model"), str) else None,
            "declares_disable_memory": "disable_memory" in config,
            "unsupported_keys": [key for key in UNSUPPORTED_AGENT_KEYS if key in config],
            "async_subagents": [
                name_value(spec.get("name"))
                for spec in subagents
                if isinstance(spec, dict) and spec.get("async") is True
            ],
        }
    if callable(agent):
        return {**fields, "kind": "bare_function"}
    return {**fields, "kind": "other"}


def read_identity(identity):
    config = getattr(identity, "config", None)
    if getattr(identity, "kind", None) != "identity" or not isinstance(config, dict):
        return {
            "declared": False,
            "auth": None,
            "auth_omitted": False,
            "scope": None,
            "declares_memory": False,
        }
    ingress = config.get("ingress")
    http = ingress.get("http") if isinstance(ingress, dict) else None
    mode = http.get("mode") if isinstance(http, dict) else None
    if http == "backend":
        auth = "backend"
    elif mode in ("langsmith_api_key", "validated_token"):
        auth = mode
    else:
        auth = None
    scope = config.get("scope") if isinstance(config.get("scope"), dict) else None
    credentials = scope.get("credentials") if scope is not None else None
    return {
        "declared": True,
        "auth": auth,
        "auth_omitted": False,
        "scope": {"credentials": credentials} if isinstance(credentials, str) else None,
        "declares_memory": scope is not None and "memory" in scope,
    }


def read_memory(memory):
    """Classify the root ``memory`` declaration the way ``memory_layers.py`` did."""
    config = getattr(memory, "config", None)
    if getattr(memory, "kind", None) != "memory" or not isinstance(config, dict):
        return {"kind": "absent", "detail": None}
    scope = config.get("scope")
    if not isinstance(scope, (list, tuple)):
        return {"kind": "dynamic", "detail": "a computed value"}
    return reduce_layers([layer_entry(layer) for layer in scope])


def layer_entry(layer):
    if not isinstance(layer, dict):
        if isinstance(layer, str):
            return {"kind": "shorthand", "detail": json.dumps(layer)}
        return {"kind": "dynamic", "detail": "a dynamic list entry"}
    layer_type = None
    has_id = False
    for key, value in layer.items():
        if not isinstance(key, str):
            return {"kind": "reserved", "detail": "a computed key"}
        if key == "type":
            if not isinstance(value, str):
                return {"kind": "dynamic", "detail": "a computed layer `type`"}
            layer_type = value
        elif key == "id":
            has_id = True
            if not isinstance(value, str):
                return {"kind": "dynamic", "detail": "a computed layer `id`"}
        elif key != "allow":
            return {"kind": "reserved", "detail": f"the key `{key}`"}
    if layer_type is None:
        return {"kind": "unknown", "detail": "a layer with no `type`"}
    if layer_type == "agent":
        if has_id:
            return {"kind": "unknown", "detail": "an `id` on the agent layer"}
        return {"kind": "agent", "detail": None}
    if layer_type == "user":
        return {"kind": "user", "detail": None}
    return {"kind": "unknown", "detail": f"type {layer_type!r}"}


def reduce_layers(entries):
    agent = False
    user = False
    for entry in entries:
        if entry["kind"] == "agent":
            if agent:
                return {"kind": "duplicate_agent", "detail": None}
            agent = True
        elif entry["kind"] == "user":
            if user:
                return {"kind": "duplicate_user", "detail": None}
            user = True
        else:
            return entry
    if agent and user:
        return {"kind": "agent_user", "detail": None}
    if agent:
        return {"kind": "agent", "detail": None}
    if user:
        return {"kind": "user", "detail": None}
    return {"kind": "none", "detail": None}


def read_sandbox(module):
    sandbox = getattr(module, "sandbox", MISSING)
    if sandbox is MISSING:
        return {"declared": False, "is_sandbox": False, "options": {}, "errors": {}}
    given = getattr(sandbox, "options", None)
    if getattr(sandbox, "kind", None) != "sandbox" or not isinstance(given, dict):
        return {"declared": True, "is_sandbox": False, "options": {}, "errors": {}}
    options = {}
    errors = {}
    for key in SANDBOX_KEYS:
        if given.get(key) is None:
            continue
        try:
            options[key] = plain_json(given[key])
        except NotJson as error:
            errors[key] = f"property `{key}` is not static: {error}"
    return {"declared": True, "is_sandbox": True, "options": options, "errors": errors}


def read_schedule(module):
    schedule = getattr(module, "schedule", MISSING)
    if schedule is MISSING:
        return {"declared": False, "is_schedule": False, "config": None, "error": None}
    config = getattr(schedule, "config", None)
    if getattr(schedule, "kind", None) != "schedule" or not isinstance(config, dict):
        return {"declared": True, "is_schedule": False, "config": None, "error": None}
    try:
        return {"declared": True, "is_schedule": True, "config": plain_json(config), "error": None}
    except NotJson as error:
        return {"declared": True, "is_schedule": True, "config": None, "error": str(error)}


def exports_of(module, name):
    return {"has_export": hasattr(module, name), "has_default": False}


def channel_ingress(channel):
    kind = getattr(channel, "kind", None)
    return kind if isinstance(kind, str) and kind in INGRESS_PROVIDERS else None


def mcp_connections(connectors):
    """The connection declarations of the collected ``define_mcp(...)`` connectors."""
    entries = []
    for connector in connectors:
        if getattr(connector, "kind", None) != "mcp_servers":
            continue
        connector_name = getattr(connector, "name", "")
        if not isinstance(connector_name, str):
            connector_name = ""
        config = getattr(connector, "config", None)
        servers = config.get("servers") if isinstance(config, dict) else None
        if not isinstance(servers, dict):
            continue
        for server_name, server in servers.items():
            if not isinstance(server_name, str) or not isinstance(server, dict):
                continue
            url = server.get("url")
            connection = server.get("connection")
            slug = getattr(connection, "slug", None)
            owner = getattr(connection, "owner", None)
            owner_policy = owner.get("type") if isinstance(owner, dict) else None
            if not isinstance(url, str) or not isinstance(slug, str):
                continue
            if not isinstance(owner_policy, str):
                continue
            entries.append(
                {
                    "connector_name": connector_name,
                    "server_name": server_name,
                    "url": url,
                    "slug": slug,
                    "owner_policy": owner_policy,
                }
            )
    return entries


def plain_json(value, seen=None):
    """``value`` as plain JSON data, or ``NotJson`` naming what is not."""
    if value is None or isinstance(value, (bool, str)):
        return value
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        if not math.isfinite(value):
            raise NotJson("a non-finite number is not plain JSON data")
        return value
    if callable(value) and not isinstance(value, (list, tuple, dict)):
        raise NotJson("a function is not plain JSON data")
    seen = set() if seen is None else seen
    if id(value) in seen:
        raise NotJson("a circular reference is not plain JSON data")
    seen.add(id(value))
    try:
        if isinstance(value, (list, tuple)):
            return [plain_json(item, seen) for item in value]
        if isinstance(value, dict):
            out = {}
            for key, item in value.items():
                if not isinstance(key, str):
                    raise NotJson(f"a key of type {type(key).__name__} is not plain JSON data")
                try:
                    out[key] = plain_json(item, seen)
                except NotJson as error:
                    raise NotJson(f"property `{key}` is not static: {error}") from None
            return out
        raise NotJson(f"an instance of {type(value).__name__} is not plain JSON data")
    finally:
        seen.discard(id(value))


def import_error(error):
    return {"kind": "import", "message": error_text(error), "detail": traceback.format_exc()}


def evaluate_error(error):
    return {"kind": "evaluate", "message": error_text(error), "detail": traceback.format_exc()}


def error_text(error):
    return f"{type(error).__name__}: {error}"


main()
