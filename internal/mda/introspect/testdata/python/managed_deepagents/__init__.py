"""Test stub of the ``managed_deepagents`` package.

The authoring functions return values shaped like the real package's, which is
all the introspection helper reads.
"""

from dataclasses import dataclass, field
from types import SimpleNamespace

from managed_deepagents._channel import CHANNEL_BRAND
from managed_deepagents._connector import CONNECTOR_BRAND


@dataclass(frozen=True)
class DeepAgentDefinition:
    config: dict
    kind = "deep-agent"


@dataclass(frozen=True, eq=False)
class NamedAgentFactory:
    name: str
    factory: object
    async_subagents: tuple = ()

    def __call__(self, runtime):
        return self.factory(runtime)


def _definition(caller, name, config):
    if "disable_memory" in config:
        raise TypeError(f"disable_memory is no longer supported in {caller}(...).")
    for key in ("backend", "store", "checkpointer", "system_prompt", "state_schema"):
        if key in config:
            raise TypeError(f"Remove `{key}` from {caller}(...).")
    return DeepAgentDefinition({**({} if name is None else {"name": name}), **config})


def define_deep_agent(*, name, **config):
    return _definition("define_deep_agent", name, config)


def define_agent(name=None, factory=None, /, **config):
    if factory is not None:
        return NamedAgentFactory(name, factory, tuple(config.get("async_subagents", ())))
    return _definition("define_agent", name, config)


class IdentityDefinition:
    kind = "identity"

    def __init__(self, config):
        self.config = config


def define_identity(*, auth):
    if auth == "backend":
        http = "backend"
    elif isinstance(auth, dict) and auth.get("mode") == "langsmith_api_key":
        http = auth
    else:
        http = {"mode": "validated_token", "providers": list(auth) if isinstance(auth, list) else [auth]}
    return IdentityDefinition({"ingress": {"http": http}, "scope": {"credentials": "user"}})


auth = SimpleNamespace(
    langsmith_api_key=lambda: {"mode": "langsmith_api_key"},
    supabase=lambda project_ref: {"id": "supabase", "claims": {"user": "sub"}},
)


@dataclass(frozen=True)
class MemoryLayer:
    allow: object = None


class MemoryDefinition:
    kind = "memory"

    def __init__(self, config):
        self.config = config


def define_memory(*, agent=None, user=None, scope=None):
    if scope is not None and (agent is not None or user is not None):
        raise ValueError("define_memory(...) cannot mix `scope` with `agent` or `user`.")
    if scope is None:
        scope = [{"type": name} for name, layer in (("agent", agent), ("user", user)) if layer]
    elif isinstance(scope, dict):
        scope = [scope]
    return MemoryDefinition({"version": 2, "scope": list(scope)})


class SandboxDefinition:
    kind = "sandbox"

    def __init__(self, options):
        self.options = options


def define_sandbox(**options):
    if "scope" in options:
        raise TypeError("sandbox scope was removed: remove scope from the sandbox declaration.")
    return SandboxDefinition({key: value for key, value in options.items() if value is not None})


class ScheduleDefinition:
    kind = "schedule"

    def __init__(self, config):
        self.config = config


def define_schedule(*, cron, **options):
    if len(cron.split()) != 5:
        raise ValueError("define_schedule(...) requires `cron` to be a standard 5-field cron expression.")
    return ScheduleDefinition({"cron": cron, **{k: v for k, v in options.items() if v is not None}})


@dataclass(frozen=True)
class Connection:
    slug: str
    owner: dict = field(default_factory=lambda: {"type": "user"})


class _Mcp:
    kind = "mcp_servers"

    def __init__(self, servers):
        self.config = {"servers": servers}
        setattr(self, CONNECTOR_BRAND, True)

    def tools(self, ctx):
        return []

    def requirements(self, name):
        slugs = sorted(
            server["connection"].slug
            for server in self.config["servers"].values()
            if server.get("connection") is not None
        )
        return {"name": name, "provider": "mcp", "requiredEnv": [], "userConnections": slugs}


def define_mcp(*, servers):
    return _Mcp(servers)


class _Channel:
    def __init__(self, kind):
        self.kind = kind
        setattr(self, CHANNEL_BRAND, True)

    def events(self, ctx):
        return None

    def requirements(self, name):
        return {"name": name, "provider": self.kind, "eventsPath": f"/channels/{name}/events"}


channels = SimpleNamespace(
    slack=lambda **options: _Channel("slack"),
    github=lambda **options: _Channel("github"),
    linear=lambda **options: _Channel("linear"),
    http=lambda *, provider, **options: _Channel(provider),
)
