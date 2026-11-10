# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Tests for build_wheels.py.

    uv run scripts/test_build_wheels.py

Set LANGSMITH_WHEEL_TEST_BINARY to a built langsmith binary for this machine to
also install its wheel into a fresh venv and run `langsmith --version`. The
twine and install checks need uv on PATH and are skipped without it.
"""

from __future__ import annotations

import base64
import csv
import hashlib
import io
import os
import platform as host
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest
import zipfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import build_wheels as bw

# Pinned so a new twine release can't change what passes.
TWINE = "twine==7.0.0"

LICENSE = b"MIT License\n\nCopyright (c) 2026 LangChain\n"
FAKE_SCRIPT = b"#!/bin/sh\necho fake-langsmith\n"


def host_platform() -> bw.Platform | None:
    goos = {"Linux": "linux", "Darwin": "darwin", "Windows": "windows"}.get(
        host.system()
    )
    goarch = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "arm64": "arm64",
        "aarch64": "arm64",
    }.get(host.machine().lower())
    if (goos, goarch) not in bw.PLATFORM_TAGS:
        return None
    return bw.Platform(goos, goarch)


def make_release(dist: Path, binaries: dict[bw.Platform, bytes]) -> None:
    """Write GoReleaser-shaped archives and checksums.txt into dist."""
    dist.mkdir(parents=True, exist_ok=True)
    sums = []
    for plat in bw.PLATFORMS:
        binary = binaries.get(plat, f"binary for {plat.goos}/{plat.goarch}".encode())
        members = [
            ("LICENSE", LICENSE, 0o644),
            ("README.md", b"# readme\n", 0o644),
            (plat.binary, binary, 0o755),
        ]
        buf = io.BytesIO()
        if plat.goos == "windows":
            with zipfile.ZipFile(buf, "w") as zf:
                for name, data, _ in members:
                    zf.writestr(name, data)
        else:
            with tarfile.open(fileobj=buf, mode="w:gz") as tf:
                for name, data, mode in members:
                    info = tarfile.TarInfo(name)
                    info.size, info.mode = len(data), mode
                    tf.addfile(info, io.BytesIO(data))
        (dist / plat.archive).write_bytes(buf.getvalue())
        sums.append(f"{hashlib.sha256(buf.getvalue()).hexdigest()}  {plat.archive}\n")
    (dist / "checksums.txt").write_text("".join(sums))


def wheel_tags(wheel: zipfile.ZipFile, info_dir: str) -> list[str]:
    text = wheel.read(f"{info_dir}/WHEEL").decode()
    return [
        line.removeprefix("Tag: ")
        for line in text.splitlines()
        if line.startswith("Tag: ")
    ]


def install_and_run(
    test: unittest.TestCase, wheel: Path, tmp: Path, binary: str
) -> str:
    """Install wheel into a fresh venv and return `langsmith --version` output."""
    venv = tmp / "venv"
    subprocess.run(["uv", "venv", "--quiet", str(venv)], check=True)
    python = venv / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
    subprocess.run(
        ["uv", "pip", "install", "--quiet", "--python", str(python), str(wheel)],
        check=True,
    )
    exe = venv / ("Scripts" if os.name == "nt" else "bin") / binary
    test.assertTrue(exe.is_file(), f"{exe} was not installed")
    return subprocess.run(
        [str(exe), "--version"], check=True, capture_output=True, text=True
    ).stdout


class BuildWheelsTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)
        self.dist = self.tmp / "dist"
        self.out = self.tmp / "wheels"

    def test_builds_one_wheel_per_platform(self) -> None:
        make_release(self.dist, {})
        wheels = bw.build_all(self.dist, "v1.2.3", self.out)

        self.assertEqual(
            sorted(w.name for w in wheels),
            sorted(
                [
                    "langchain_langsmith_cli-1.2.3-py3-none-macosx_12_0_x86_64.whl",
                    "langchain_langsmith_cli-1.2.3-py3-none-macosx_12_0_arm64.whl",
                    "langchain_langsmith_cli-1.2.3-py3-none-manylinux_2_17_aarch64.manylinux2014_aarch64.musllinux_1_2_aarch64.whl",
                    "langchain_langsmith_cli-1.2.3-py3-none-manylinux_2_17_x86_64.manylinux2014_x86_64.musllinux_1_2_x86_64.whl",
                    "langchain_langsmith_cli-1.2.3-py3-none-win_amd64.whl",
                    "langchain_langsmith_cli-1.2.3-py3-none-win_arm64.whl",
                ]
            ),
        )
        self.assertEqual(
            sorted(p.name for p in self.out.iterdir()), sorted(w.name for w in wheels)
        )

    def test_wheel_contents(self) -> None:
        make_release(self.dist, {})
        wheels = {
            w.name.split("-py3-none-")[1]: w
            for w in bw.build_all(self.dist, "v1.2.3", self.out)
        }

        for tagset, plat in [
            (
                "manylinux_2_17_x86_64.manylinux2014_x86_64.musllinux_1_2_x86_64.whl",
                bw.Platform("linux", "amd64"),
            ),
            ("win_arm64.whl", bw.Platform("windows", "arm64")),
        ]:
            with self.subTest(plat=plat), zipfile.ZipFile(wheels[tagset]) as zf:
                info_dir = "langchain_langsmith_cli-1.2.3.dist-info"
                script = f"langchain_langsmith_cli-1.2.3.data/scripts/{plat.binary}"

                self.assertEqual(
                    wheel_tags(zf, info_dir), [f"py3-none-{t}" for t in plat.tags]
                )
                self.assertEqual(
                    zf.read(script), f"binary for {plat.goos}/{plat.goarch}".encode()
                )
                self.assertEqual(zf.getinfo(script).external_attr >> 16 & 0o777, 0o755)
                self.assertEqual(zf.read(f"{info_dir}/licenses/LICENSE"), LICENSE)

                meta = zf.read(f"{info_dir}/METADATA").decode()
                self.assertIn("Name: langchain-langsmith-cli\n", meta)
                self.assertIn("Version: 1.2.3\n", meta)
                self.assertIn("License-Expression: MIT\n", meta)
                self.assertIn(f"Project-URL: Documentation, {bw.DOCS_URL}\n", meta)
                self.assertIn(bw.DOCS_URL, meta.split("\n\n", 1)[1])

                # RECORD lists every other file with a matching hash and size.
                rows = list(
                    csv.reader(zf.read(f"{info_dir}/RECORD").decode().splitlines())
                )
                self.assertEqual({r[0] for r in rows}, set(zf.namelist()))
                for path, digest, size in rows:
                    if path.endswith("/RECORD"):
                        self.assertEqual((digest, size), ("", ""))
                        continue
                    data = zf.read(path)
                    want = (
                        base64.urlsafe_b64encode(hashlib.sha256(data).digest())
                        .rstrip(b"=")
                        .decode()
                    )
                    self.assertEqual(digest, f"sha256={want}", path)
                    self.assertEqual(int(size), len(data), path)

    def test_name_override_and_reproducible(self) -> None:
        make_release(self.dist, {})
        first = bw.build_all(
            self.dist, "v0.4.0rc1", self.tmp / "a", name="LangSmith.Bin"
        )
        second = bw.build_all(
            self.dist, "v0.4.0rc1", self.tmp / "b", name="LangSmith.Bin"
        )
        self.assertTrue(first[0].name.startswith("langsmith_bin-0.4.0rc1-py3-none-"))
        for a, b in zip(first, second):
            self.assertEqual(a.read_bytes(), b.read_bytes())

    def test_rejects_checksum_mismatch(self) -> None:
        make_release(self.dist, {})
        archive = self.dist / bw.PLATFORMS[0].archive
        archive.write_bytes(archive.read_bytes() + b"tampered")
        with self.assertRaisesRegex(SystemExit, "sha256"):
            bw.build_all(self.dist, "v1.2.3", self.out)

    def test_rejects_missing_archive(self) -> None:
        make_release(self.dist, {})
        (self.dist / "langsmith_windows_arm64.zip").unlink()
        with self.assertRaisesRegex(SystemExit, "langsmith_windows_arm64.zip"):
            bw.build_all(self.dist, "v1.2.3", self.out)

    def test_rejects_non_release_version(self) -> None:
        make_release(self.dist, {})
        for tag in ["v1.2.3-internal-abc123", "main", "v1.2.3-rc.1"]:
            with self.subTest(tag=tag), self.assertRaises(SystemExit):
                bw.build_all(self.dist, tag, self.out)

    @unittest.skipUnless(shutil.which("uvx"), "needs uvx")
    def test_twine_check(self) -> None:
        make_release(self.dist, {})
        wheels = bw.build_all(self.dist, "v1.2.3", self.out)
        subprocess.run(
            ["uvx", TWINE, "check", "--strict", *map(str, wheels)], check=True
        )

    @unittest.skipUnless(
        shutil.which("uv") and os.name != "nt" and host_platform(),
        "needs uv on a unix host",
    )
    def test_installs_executable_script(self) -> None:
        plat = host_platform()
        make_release(self.dist, {plat: FAKE_SCRIPT})
        wheel = next(
            w
            for w in bw.build_all(self.dist, "v1.2.3", self.out)
            if w.name.endswith(f"{plat.tags[-1]}.whl")
        )
        self.assertEqual(
            install_and_run(self, wheel, self.tmp, plat.binary).strip(),
            "fake-langsmith",
        )

    @unittest.skipUnless(
        os.environ.get("LANGSMITH_WHEEL_TEST_BINARY"), "set LANGSMITH_WHEEL_TEST_BINARY"
    )
    def test_installs_real_binary(self) -> None:
        plat = host_platform()
        self.assertIsNotNone(plat, f"no wheel for {host.system()}/{host.machine()}")
        binary = Path(os.environ["LANGSMITH_WHEEL_TEST_BINARY"]).read_bytes()
        make_release(self.dist, {plat: binary})
        wheel = next(
            w
            for w in bw.build_all(self.dist, "v1.2.3", self.out)
            if w.name.endswith(f"{plat.tags[-1]}.whl")
        )
        self.assertIn(
            "langsmith", install_and_run(self, wheel, self.tmp, plat.binary).lower()
        )


if __name__ == "__main__":
    unittest.main()
