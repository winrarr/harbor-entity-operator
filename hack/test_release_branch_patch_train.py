#!/usr/bin/env python3
"""Local tests for release_branch_patch_train.py.

These tests use temporary git repositories and dry-run mode, so they do not
touch the real remote or GitHub API.
"""

from __future__ import annotations

import os
import subprocess
import tempfile
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "hack" / "release_branch_patch_train.py"


def run(args: list[str], cwd: Path, *, env: dict[str, str] | None = None) -> str:
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)
    return subprocess.run(
        args,
        cwd=cwd,
        env=merged_env,
        check=True,
        text=True,
        capture_output=True,
    ).stdout


class PatchTrainRepo:
    def __init__(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.origin = self.root / "origin.git"
        self.work = self.root / "work"
        run(["git", "init", "--bare", str(self.origin)], self.root)
        run(["git", "clone", str(self.origin), str(self.work)], self.root)
        run(["git", "config", "user.name", "test"], self.work)
        run(["git", "config", "user.email", "test@example.com"], self.work)
        self._seed()

    def cleanup(self) -> None:
        self.tmp.cleanup()

    def _seed(self) -> None:
        for directory in (
            "api",
            "cmd",
            "internal",
            "charts/harbor-entity-operator/templates",
        ):
            (self.work / directory).mkdir(parents=True, exist_ok=True)

        (self.work / "charts/harbor-entity-operator/Chart.yaml").write_text(
            'apiVersion: v2\nname: harbor-entity-operator\nversion: 0.5.0\nappVersion: "0.7.0"\n'
        )
        (self.work / "charts/harbor-entity-operator/templates/deployment.yaml").write_text("old chart\n")
        (self.work / "go.mod").write_text("module example.com/test\n")
        (self.work / "go.sum").write_text("")
        (self.work / "Dockerfile").write_text("FROM scratch\n")
        (self.work / "api/a.go").write_text("package api\n")
        (self.work / "cmd/main.go").write_text("package main\nfunc main(){}\n")
        (self.work / "internal/i.go").write_text("package internal\n")

        self.commit("initial release metadata")
        run(["git", "branch", "-M", "main"], self.work)
        self.tag("v0.7.0", "Release v0.7.0")
        self.tag("chart-v0.5.0", "Chart release chart-v0.5.0 for v0.7.0")
        run(["git", "push", "origin", "main", "--tags"], self.work)
        run(["git", "switch", "-c", "release/v0.7"], self.work)
        run(["git", "push", "origin", "release/v0.7"], self.work)

    def commit(self, message: str) -> None:
        run(["git", "add", "."], self.work)
        run(["git", "commit", "-m", message], self.work)

    def tag(self, tag: str, message: str) -> None:
        run(["git", "tag", "-a", tag, "-m", message], self.work)

    def push_branch_and_tags(self) -> None:
        run(["git", "push", "origin", "release/v0.7", "--tags"], self.work)

    def patch_train_output(self, extra_env: dict[str, str] | None = None) -> str:
        env = {
            "DRY_RUN": "true",
            "RELEASE_BRANCH": "release/v0.7",
        }
        if extra_env:
            env.update(extra_env)
        return run(
            ["python3", str(SCRIPT)],
            self.work,
            env=env,
        )


class ReleaseBranchPatchTrainTests(unittest.TestCase):
    def setUp(self) -> None:
        self.repo = PatchTrainRepo()

    def tearDown(self) -> None:
        self.repo.cleanup()

    def test_manual_run_skips_unsupported_release_line(self) -> None:
        for branch in ("release/v0.8", "release/v0.9", "release/v0.10"):
            run(["git", "branch", branch], self.repo.work)
            run(["git", "push", "origin", branch], self.repo.work)

        output = self.repo.patch_train_output()

        self.assertIn("release/v0.7: outside the supported maintenance window", output)

    def test_emergency_override_requires_and_records_a_reason(self) -> None:
        for branch in ("release/v0.8", "release/v0.9", "release/v0.10"):
            run(["git", "branch", branch], self.repo.work)
            run(["git", "push", "origin", branch], self.repo.work)

        output = self.repo.patch_train_output({"EMERGENCY_OVERRIDE": "true", "EMERGENCY_REASON": "security update"})

        self.assertIn("Emergency override selected for unsupported line release/v0.7 with a documented reason", output)

    def test_emergency_override_without_reason_fails_closed(self) -> None:
        result = subprocess.run(
            ["python3", str(SCRIPT)],
            cwd=self.repo.work,
            env={**os.environ, "DRY_RUN": "true", "RELEASE_BRANCH": "release/v0.7", "EMERGENCY_OVERRIDE": "true"},
            text=True,
            capture_output=True,
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("requires a non-empty EMERGENCY_REASON", result.stdout)

    def test_dependency_update_prepares_paired_release(self) -> None:
        with (self.repo.work / "go.sum").open("a") as go_sum:
            go_sum.write("# dep update\n")
        self.repo.commit("chore(deps): update dependency")
        self.repo.push_branch_and_tags()

        output = self.repo.patch_train_output()

        self.assertIn("Next operator tag: v0.7.1", output)
        self.assertIn("Next chart version: 0.5.1", output)
        self.assertIn("Would create paired release tag v0.7.1", output)
        self.assertNotIn("Next chart tag:", output)

    def test_prior_chart_only_release_does_not_block_dependency_update(self) -> None:
        (self.repo.work / "charts/harbor-entity-operator/Chart.yaml").write_text(
            'apiVersion: v2\nname: harbor-entity-operator\nversion: 0.5.1\nappVersion: "0.7.0"\n'
        )
        (self.repo.work / "charts/harbor-entity-operator/templates/deployment.yaml").write_text(
            "chart-only change\n"
        )
        self.repo.commit("chore(chart): release chart patch")
        self.repo.tag("chart-v0.5.1", "Chart release chart-v0.5.1 for v0.7.0")
        self.repo.push_branch_and_tags()

        with (self.repo.work / "go.sum").open("a") as go_sum:
            go_sum.write("# dep update\n")
        self.repo.commit("chore(deps): update dependency")
        self.repo.push_branch_and_tags()

        output = self.repo.patch_train_output()

        self.assertIn("Next operator tag: v0.7.1", output)
        self.assertIn("Next chart version: 0.5.2", output)

    def test_prior_paired_release_advances_chart_version(self) -> None:
        (self.repo.work / "charts/harbor-entity-operator/Chart.yaml").write_text(
            'apiVersion: v2\nname: harbor-entity-operator\nversion: 0.5.1\nappVersion: "0.7.1"\n'
        )
        self.repo.commit("feat: prepare paired release")
        self.repo.tag("v0.7.1", "Release v0.7.1")
        self.repo.push_branch_and_tags()

        with (self.repo.work / "go.sum").open("a") as go_sum:
            go_sum.write("# dep update\n")
        self.repo.commit("chore(deps): update dependency")
        self.repo.push_branch_and_tags()

        output = self.repo.patch_train_output()

        self.assertIn("Next operator tag: v0.7.2", output)
        self.assertIn("Next chart version: 0.5.2", output)

    def test_unpublished_chart_change_blocks_dependency_update(self) -> None:
        (self.repo.work / "charts/harbor-entity-operator/templates/deployment.yaml").write_text(
            "unpublished chart change\n"
        )
        with (self.repo.work / "go.sum").open("a") as go_sum:
            go_sum.write("# dep update\n")
        self.repo.commit("mixed chart and dependency change")
        self.repo.push_branch_and_tags()

        output = self.repo.patch_train_output()

        self.assertIn("chart changes require explicit chart release intent", output)

    def test_existing_automated_tag_schedules_missing_paired_publish(self) -> None:
        (self.repo.work / "charts/harbor-entity-operator/Chart.yaml").write_text(
            'apiVersion: v2\nname: harbor-entity-operator\nversion: 0.5.1\nappVersion: "0.7.1"\n'
        )
        with (self.repo.work / "go.sum").open("a") as go_sum:
            go_sum.write("# dep update\n")
        self.repo.commit("chore(release): prepare v0.7.1")
        self.repo.tag("v0.7.1", "Automated dependency patch release v0.7.1")
        self.repo.push_branch_and_tags()

        output = self.repo.patch_train_output()

        self.assertIn("Would dispatch publish-release.yaml for v0.7.1", output)

    def test_stale_chart_metadata_blocks_paired_release_recovery(self) -> None:
        with (self.repo.work / "go.sum").open("a") as go_sum:
            go_sum.write("# dep update\n")
        self.repo.commit("automated dependency patch without chart metadata")
        self.repo.tag("v0.7.1", "Automated dependency patch release v0.7.1")
        self.repo.push_branch_and_tags()

        output = self.repo.patch_train_output()

        self.assertIn("skipping automated release recovery", output)


if __name__ == "__main__":
    unittest.main()
