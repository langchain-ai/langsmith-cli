"""Test stub of ``managed_deepagents._channel``."""

from managed_deepagents._connector import collect, requirements_from

CHANNEL_BRAND = "__mda_channel__"


def collect_channels(modules):
    return collect(modules, "channel", CHANNEL_BRAND, "channel")


def requirements_from_channels(channels):
    return requirements_from(channels)
