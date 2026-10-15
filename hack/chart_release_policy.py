#!/usr/bin/env python3
"""Pure validation rules used by the official chart-only publisher."""

from __future__ import annotations

import re
import sys

from release_policy import release_branch_for_version, release_branches_from_git, supported_release_branches


SOURCE_BRANCH_PATTERN = re.compile(r"^release/v[0-9]+\.[0-9]+$")
STABLE_VERSION_PATTERN = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")


def validate_chart_release(
    source_ref: str,
    chart_version: str,
    app_version: str,
    emergency_override: bool,
    emergency_reason: str,
    release_branches: list[str],
) -> str:
    if source_ref != "main" and SOURCE_BRANCH_PATTERN.fullmatch(source_ref) is None:
        raise ValueError("source_ref must be main or an exact release/vX.Y branch")
    if STABLE_VERSION_PATTERN.fullmatch(chart_version) is None:
        raise ValueError("chart_version must be a stable X.Y.Z version")
    if STABLE_VERSION_PATTERN.fullmatch(app_version) is None:
        raise ValueError(f"Chart.yaml appVersion must select a stable X.Y.Z operator image, got: {app_version}")

    reason = emergency_reason.strip()
    if emergency_override and not reason:
        raise ValueError("emergency_override requires a non-empty emergency_reason")
    if not emergency_override and reason:
        raise ValueError("emergency_reason requires emergency_override=true")

    supported = supported_release_branches(release_branches)
    if source_ref != "main" and source_ref not in supported and not emergency_override:
        raise ValueError(
            f"source release branch {source_ref} is outside the supported maintenance window; "
            "use an emergency override and approval"
        )

    operator_branch = release_branch_for_version(app_version)
    if operator_branch not in supported and not emergency_override:
        raise ValueError(
            f"operator line {operator_branch.removeprefix('release/')} is outside the supported maintenance window; "
            "use an emergency override and approval"
        )

    return f"chart-v{chart_version}"


def validate_chart_tag_target(chart_tag: str, source_sha: str, existing_target_sha: str) -> bool:
    if existing_target_sha and existing_target_sha != source_sha:
        raise ValueError(
            f"immutable chart tag {chart_tag} already points to {existing_target_sha}, selected source is {source_sha}"
        )
    return bool(existing_target_sha)


def main(argv: list[str]) -> int:
    if len(argv) == 7 and argv[1] == "metadata":
        _, _, source_ref, chart_version, app_version, override_text, reason = argv
        if override_text not in {"true", "false"}:
            print("emergency_override must be true or false", file=sys.stderr)
            return 2
        try:
            chart_tag = validate_chart_release(
                source_ref,
                chart_version,
                app_version,
                override_text == "true",
                reason,
                release_branches_from_git(),
            )
        except (RuntimeError, ValueError) as err:
            print(str(err), file=sys.stderr)
            return 1
        print(chart_tag)
        return 0

    if len(argv) == 5 and argv[1] == "check-tag":
        _, _, chart_tag, source_sha, existing_target_sha = argv
        try:
            exists = validate_chart_tag_target(chart_tag, source_sha, existing_target_sha)
        except ValueError as err:
            print(str(err), file=sys.stderr)
            return 1
        print("exists" if exists else "missing")
        return 0

    print(
        "usage: chart_release_policy.py metadata SOURCE_REF CHART_VERSION APP_VERSION OVERRIDE REASON | "
        "check-tag TAG SOURCE_SHA EXISTING_TARGET_SHA",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
