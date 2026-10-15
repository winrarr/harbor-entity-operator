#!/usr/bin/env python3
"""Release-line support policy shared by release automation."""

from __future__ import annotations

import re
import subprocess
import sys


SUPPORTED_RELEASE_LINE_COUNT = 3
RELEASE_BRANCH_PATTERN = re.compile(r"^release/v?(?P<major>[0-9]+)\.(?P<minor>[0-9]+)$")
STABLE_VERSION_PATTERN = re.compile(r"^(?P<major>[0-9]+)\.(?P<minor>[0-9]+)\.[0-9]+(?:-rc\.[0-9]+)?$")


def release_line_key(branch: str) -> tuple[int, int] | None:
    match = RELEASE_BRANCH_PATTERN.fullmatch(branch)
    if not match:
        return None
    return int(match.group("major")), int(match.group("minor"))


def supported_release_branches(branches: list[str]) -> list[str]:
    """Return the newest three numeric release lines, including current."""
    keyed = [(key, branch) for branch in branches if (key := release_line_key(branch)) is not None]
    keyed.sort(key=lambda item: item[0])
    return [branch for _, branch in keyed[-SUPPORTED_RELEASE_LINE_COUNT:]]


def release_branch_for_version(version: str) -> str:
    match = STABLE_VERSION_PATTERN.fullmatch(version)
    if not match:
        raise ValueError(f"invalid operator version {version!r}; expected X.Y.Z or X.Y.Z-rc.N")
    return f"release/v{match.group('major')}.{match.group('minor')}"


def release_branches_from_git() -> list[str]:
    result = subprocess.run(
        ["git", "for-each-ref", "--format=%(refname:strip=3)", "refs/remotes/origin/release/*"],
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or "failed to list release branches")
    return [branch for branch in result.stdout.splitlines() if branch]


def is_unsupported(branch: str, branches: list[str] | None = None) -> bool:
    candidates = release_branches_from_git() if branches is None else branches
    return branch not in supported_release_branches(candidates)


def main(argv: list[str]) -> int:
    if len(argv) != 3 or argv[1] not in {"check-branch", "check-version"}:
        print("usage: release_policy.py check-branch BRANCH | check-version VERSION", file=sys.stderr)
        return 2

    try:
        branch = argv[2] if argv[1] == "check-branch" else release_branch_for_version(argv[2])
        branches = release_branches_from_git()
    except (RuntimeError, ValueError) as err:
        print(str(err), file=sys.stderr)
        return 2

    supported = supported_release_branches(branches)
    if branch not in supported:
        branch_name = branch.removeprefix("release/")
        current = supported[-1].removeprefix("release/") if supported else "none"
        print(
            f"Release line {branch_name} is outside the supported maintenance window. "
            f"Supported lines: {', '.join(item.removeprefix('release/') for item in supported) or 'none'}; "
            f"current line: {current}.",
            file=sys.stderr,
        )
        return 1
    print(f"Release line {branch.removeprefix('release/')} is supported.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
