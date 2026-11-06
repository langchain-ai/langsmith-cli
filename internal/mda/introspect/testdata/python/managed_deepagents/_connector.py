"""Test stub of ``managed_deepagents._connector``."""

CONNECTOR_BRAND = "__mda_connector__"


def _stem(source, kind):
    base = source.replace("\\", "/").rsplit("/", 1)[-1].rsplit(".", 1)[0]
    if not base or base == kind:
        raise ValueError(f'invalid {kind} path "{source}"')
    return base


def collect(modules, attribute, brand, kind):
    collected = []
    names = set()
    for source, module in modules:
        candidate = getattr(module, attribute, None)
        if candidate is None or not getattr(candidate, brand, False):
            continue
        name = _stem(source, kind)
        if name in names:
            raise ValueError(f"duplicate {kind} name '{name}'")
        names.add(name)
        object.__setattr__(candidate, "name", name)
        collected.append(candidate)
    return collected


def requirements_from(items):
    out = {}
    for item in items:
        hook = getattr(item, "requirements", None)
        name = getattr(item, "name", None)
        if callable(hook) and isinstance(name, str) and name:
            out[name] = hook(name)
    return out


def collect_connectors(modules):
    return collect(modules, "connector", CONNECTOR_BRAND, "connector")


def requirements_from_connectors(connectors):
    return requirements_from(connectors)
