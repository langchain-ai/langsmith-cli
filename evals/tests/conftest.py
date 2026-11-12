import os
import shutil
from pathlib import Path

import pytest
from agent_evals import surface

REPO_BINARY = Path(__file__).resolve().parents[2] / "bin" / "langsmith"


@pytest.fixture(scope="session")
def binary() -> str:
    path = os.environ.get("LANGSMITH_CLI") or (
        str(REPO_BINARY) if REPO_BINARY.exists() else shutil.which("langsmith")
    )
    if not path:
        pytest.skip("no langsmith binary; run `make build` in cli/")
    return path


@pytest.fixture(scope="session")
def tree(binary):
    return surface.crawl(binary)
