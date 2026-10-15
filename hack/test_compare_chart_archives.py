#!/usr/bin/env python3
"""Unit tests for immutable Helm chart content comparison."""

from __future__ import annotations

import io
import tarfile
import tempfile
import unittest
from pathlib import Path

from compare_chart_archives import archive_digest


def make_archive(path: Path, entries: dict[str, bytes], *, mtime: int) -> None:
    with tarfile.open(path, mode="w:gz") as archive:
        for name, content in entries.items():
            member = tarfile.TarInfo(name)
            member.size = len(content)
            member.mtime = mtime
            archive.addfile(member, io.BytesIO(content))


class CompareChartArchivesTests(unittest.TestCase):
    def test_digest_ignores_tar_metadata_but_covers_chart_paths_and_contents(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            first = Path(directory) / "first.tgz"
            second = Path(directory) / "second.tgz"
            entries = {"harbor-entity-operator/Chart.yaml": b"version: 1.2.3\n", "harbor-entity-operator/values.yaml": b"replicas: 1\n"}
            make_archive(first, entries, mtime=1)
            make_archive(second, entries, mtime=2)

            self.assertEqual(archive_digest(str(first)), archive_digest(str(second)))

            changed = Path(directory) / "changed.tgz"
            make_archive(changed, {**entries, "harbor-entity-operator/values.yaml": b"replicas: 2\n"}, mtime=2)
            self.assertNotEqual(archive_digest(str(first)), archive_digest(str(changed)))

    def test_digest_rejects_path_traversal(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / "unsafe.tgz"
            make_archive(archive, {"harbor-entity-operator/../outside": b"unsafe"}, mtime=1)

            with self.assertRaisesRegex(ValueError, "unsafe path"):
                archive_digest(str(archive))


if __name__ == "__main__":
    unittest.main()
