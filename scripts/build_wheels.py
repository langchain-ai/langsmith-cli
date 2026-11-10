# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Repackage GoReleaser archives as PyPI platform wheels.

Each `langsmith_{os}_{arch}` archive from a release becomes one wheel that
installs the prebuilt binary into the environment's scripts directory, the way
ruff and uv ship on PyPI. Nothing is compiled, and no sdist is produced, so pip
on an unsupported platform reports that no wheel matches instead of trying to
build from source.

    uv run scripts/build_wheels.py --dist <dir with archives> --version v0.3.13

The wheel is written by hand (a zip with WHEEL, METADATA and RECORD) so the
script needs only the standard library.
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import io
import re
import sys
import tarfile
import zipfile
from dataclasses import dataclass
from pathlib import Path

# langsmith-cli on PyPI belongs to an unrelated project. Keep this in sync with
# pypiPackageName in internal/cmd/install_method.go, which self-update uses to
# find the installed wheel.
PACKAGE_NAME = "langchain-langsmith-cli"

BINARY = "langsmith"
DOCS_URL = "https://docs.langchain.com/langsmith/langsmith-cli"
SOURCE_URL = "https://github.com/langchain-ai/langsmith-cli"

# GoReleaser platform -> wheel platform tags. Go binaries are static
# (CGO_ENABLED=0), so the Linux wheels claim both glibc and musl. The macOS
# tags follow Go's minimum macOS for the toolchain in go.mod: macOS 12 since
# Go 1.25, and Go 1.26 is the last release that supports macOS 12, so raise
# these tags when go.mod moves past 1.26.
PLATFORM_TAGS: dict[tuple[str, str], list[str]] = {
    ("linux", "amd64"): [
        "manylinux_2_17_x86_64",
        "manylinux2014_x86_64",
        "musllinux_1_2_x86_64",
    ],
    ("linux", "arm64"): [
        "manylinux_2_17_aarch64",
        "manylinux2014_aarch64",
        "musllinux_1_2_aarch64",
    ],
    ("darwin", "amd64"): ["macosx_12_0_x86_64"],
    ("darwin", "arm64"): ["macosx_12_0_arm64"],
    ("windows", "amd64"): ["win_amd64"],
    ("windows", "arm64"): ["win_arm64"],
}

# Fixed entry timestamps keep wheels for the same inputs byte-identical.
ZIP_DATE = (2000, 1, 1, 0, 0, 0)

# PEP 440 public versions in normalized form, which is what PyPI accepts.
VERSION_RE = re.compile(r"^\d+(\.\d+)*((a|b|rc)\d+)?(\.post\d+)?(\.dev\d+)?$")

README_TEMPLATE = """\
# {name}

The LangSmith CLI (`langsmith`): an agent-first command line for querying and
managing [LangSmith](https://smith.langchain.com) projects, traces, runs,
datasets, evaluators, experiments, and threads.

This package ships the prebuilt `langsmith` binary for your platform. Install
it with any Python package manager:

```bash
uv tool install {name}
pipx install {name}
pip install {name}
```

Then run `langsmith --help`.

- Documentation: {docs}
- Source and releases: {source}
"""


@dataclass(frozen=True)
class Platform:
    goos: str
    goarch: str

    @property
    def archive(self) -> str:
        ext = "zip" if self.goos == "windows" else "tar.gz"
        return f"{BINARY}_{self.goos}_{self.goarch}.{ext}"

    @property
    def binary(self) -> str:
        return f"{BINARY}.exe" if self.goos == "windows" else BINARY

    @property
    def tags(self) -> list[str]:
        return PLATFORM_TAGS[(self.goos, self.goarch)]


PLATFORMS = [Platform(goos, goarch) for goos, goarch in PLATFORM_TAGS]


def normalize_version(tag: str) -> str:
    version = tag.removeprefix("v")
    if not VERSION_RE.match(version):
        raise SystemExit(f"error: {tag!r} is not a release version (expected vX.Y.Z)")
    return version


def wheel_dist_name(name: str) -> str:
    """Escape a project name for wheel and dist-info file names."""
    return re.sub(r"[-_.]+", "_", name).lower()


def read_checksums(path: Path) -> dict[str, str]:
    sums = {}
    for line in path.read_text().splitlines():
        if not line.strip():
            continue
        digest, name = line.split(maxsplit=1)
        sums[name.lstrip("*")] = digest.lower()
    return sums


def verify_archive(path: Path, checksums: dict[str, str]) -> bytes:
    expected = checksums.get(path.name)
    if expected is None:
        raise SystemExit(f"error: {path.name} is not listed in checksums.txt")
    data = path.read_bytes()
    actual = hashlib.sha256(data).hexdigest()
    if actual != expected:
        raise SystemExit(
            f"error: {path.name} sha256 is {actual}, checksums.txt says {expected}"
        )
    return data


def read_member(archive: bytes, platform: Platform, member: str) -> bytes:
    """Read a top-level file from an archive without extracting it to disk."""
    if platform.goos == "windows":
        with zipfile.ZipFile(io.BytesIO(archive)) as zf:
            try:
                return zf.read(member)
            except KeyError:
                pass
    else:
        with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as tf:
            for info in tf:
                if info.isfile() and info.name.removeprefix("./") == member:
                    f = tf.extractfile(info)
                    assert f is not None
                    return f.read()
    raise SystemExit(f"error: {platform.archive} has no {member}")


def record_hash(data: bytes) -> str:
    digest = base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=")
    return "sha256=" + digest.decode()


def metadata(name: str, version: str) -> str:
    readme = README_TEMPLATE.format(name=name, docs=DOCS_URL, source=SOURCE_URL)
    headers = [
        ("Metadata-Version", "2.4"),
        ("Name", name),
        ("Version", version),
        ("Summary", "The LangSmith CLI, an agent-first command line for LangSmith"),
        ("Author", "LangChain"),
        ("License-Expression", "MIT"),
        ("License-File", "LICENSE"),
        ("Project-URL", f"Documentation, {DOCS_URL}"),
        ("Project-URL", f"Source, {SOURCE_URL}"),
        ("Project-URL", f"Changelog, {SOURCE_URL}/releases"),
        ("Project-URL", f"Issues, {SOURCE_URL}/issues"),
        ("Classifier", "Environment :: Console"),
        ("Classifier", "Programming Language :: Go"),
        ("Classifier", "Topic :: Software Development"),
        ("Description-Content-Type", "text/markdown"),
    ]
    return "".join(f"{k}: {v}\n" for k, v in headers) + "\n" + readme


def wheel_file(platform: Platform) -> str:
    lines = [
        "Wheel-Version: 1.0",
        f"Generator: {Path(__file__).name}",
        "Root-Is-Purelib: false",
    ]
    lines += [f"Tag: py3-none-{tag}" for tag in platform.tags]
    return "\n".join(lines) + "\n"


def build_wheel(
    platform: Platform, archive: bytes, name: str, version: str, out: Path
) -> Path:
    dist = wheel_dist_name(name)
    data_dir = f"{dist}-{version}.data"
    info_dir = f"{dist}-{version}.dist-info"

    binary = read_member(archive, platform, platform.binary)
    license_text = read_member(archive, platform, "LICENSE")

    # (path, contents, unix mode); RECORD is appended last.
    files = [
        (f"{data_dir}/scripts/{platform.binary}", binary, 0o755),
        (f"{info_dir}/licenses/LICENSE", license_text, 0o644),
        (f"{info_dir}/METADATA", metadata(name, version).encode(), 0o644),
        (f"{info_dir}/WHEEL", wheel_file(platform).encode(), 0o644),
    ]
    record_path = f"{info_dir}/RECORD"
    record = "".join(
        f"{path},{record_hash(data)},{len(data)}\n" for path, data, _ in files
    )
    record += f"{record_path},,\n"
    files.append((record_path, record.encode(), 0o644))

    out.mkdir(parents=True, exist_ok=True)
    wheel_path = out / f"{dist}-{version}-py3-none-{'.'.join(platform.tags)}.whl"
    with zipfile.ZipFile(wheel_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        for path, data, mode in files:
            info = zipfile.ZipInfo(path, date_time=ZIP_DATE)
            info.compress_type = zipfile.ZIP_DEFLATED
            info.create_system = 3  # unix, so installers honor the mode bits
            info.external_attr = (0o100000 | mode) << 16
            zf.writestr(info, data)
    return wheel_path


def build_all(
    dist: Path, version_tag: str, out: Path, name: str = PACKAGE_NAME
) -> list[Path]:
    version = normalize_version(version_tag)
    checksums = read_checksums(dist / "checksums.txt")
    missing = [p.archive for p in PLATFORMS if not (dist / p.archive).is_file()]
    if missing:
        raise SystemExit(f"error: missing archives in {dist}: {', '.join(missing)}")
    wheels = []
    for platform in PLATFORMS:
        archive = verify_archive(dist / platform.archive, checksums)
        wheels.append(build_wheel(platform, archive, name, version, out))
    return wheels


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "--dist",
        type=Path,
        required=True,
        help="directory with the GoReleaser archives and checksums.txt",
    )
    parser.add_argument(
        "--version", required=True, help="release tag, for example v0.3.13"
    )
    parser.add_argument(
        "--out",
        type=Path,
        default=Path("wheels"),
        help="output directory (default: wheels)",
    )
    parser.add_argument(
        "--name",
        default=PACKAGE_NAME,
        help=f"PyPI distribution name (default: {PACKAGE_NAME})",
    )
    args = parser.parse_args(argv)
    for wheel in build_all(args.dist, args.version, args.out, args.name):
        print(wheel)
    return 0


if __name__ == "__main__":
    sys.exit(main())
