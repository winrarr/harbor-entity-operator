#!/usr/bin/env python3
"""Compare Helm chart archives by their safe, normalized file contents."""

from __future__ import annotations

import hashlib
import sys
import tarfile
from pathlib import PurePosixPath


MAX_ARCHIVE_FILES = 10_000
MAX_UNCOMPRESSED_BYTES = 100 * 1024 * 1024


def archive_digest(filename: str) -> str:
    digest = hashlib.sha256()
    file_count = 0
    total_size = 0
    archive_root: str | None = None

    with tarfile.open(filename, mode="r:gz") as archive:
        members = archive.getmembers()
        if len(members) > MAX_ARCHIVE_FILES:
            raise ValueError(f"chart archive contains more than {MAX_ARCHIVE_FILES} entries")

        normalized: list[tuple[str, tarfile.TarInfo]] = []
        for member in members:
            path = PurePosixPath(member.name)
            if path.is_absolute() or ".." in path.parts:
                raise ValueError(f"chart archive contains an unsafe path: {member.name!r}")
            if not member.isfile():
                if member.isdir():
                    continue
                raise ValueError(f"chart archive contains a non-regular entry: {member.name!r}")
            if len(path.parts) < 2:
                raise ValueError(f"chart archive entry is missing its chart root: {member.name!r}")
            if archive_root is None:
                archive_root = path.parts[0]
            if path.parts[0] != archive_root:
                raise ValueError("chart archive contains more than one root directory")
            relative = "/".join(path.parts[1:])
            total_size += member.size
            if total_size > MAX_UNCOMPRESSED_BYTES:
                raise ValueError(f"chart archive expands beyond {MAX_UNCOMPRESSED_BYTES} bytes")
            normalized.append((relative, member))

        for relative, member in sorted(normalized, key=lambda item: item[0]):
            stream = archive.extractfile(member)
            if stream is None:
                raise ValueError(f"chart archive entry cannot be read: {member.name!r}")
            content = stream.read()
            if len(content) != member.size:
                raise ValueError(f"chart archive entry has an unexpected size: {member.name!r}")
            name_bytes = relative.encode("utf-8")
            digest.update(len(name_bytes).to_bytes(8, "big"))
            digest.update(name_bytes)
            digest.update(len(content).to_bytes(8, "big"))
            digest.update(content)
            file_count += 1

    if file_count == 0:
        raise ValueError("chart archive is empty")
    return digest.hexdigest()


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print("usage: compare_chart_archives.py LOCAL.tgz REMOTE.tgz", file=sys.stderr)
        return 2
    try:
        local_digest = archive_digest(argv[1])
        remote_digest = archive_digest(argv[2])
    except (OSError, tarfile.TarError, ValueError) as err:
        print(f"cannot compare chart archives: {err}", file=sys.stderr)
        return 2
    if local_digest != remote_digest:
        print("existing chart contents differ from the selected source ref", file=sys.stderr)
        return 1
    print(f"chart contents match (sha256:{local_digest})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
