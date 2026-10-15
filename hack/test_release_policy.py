#!/usr/bin/env python3
"""Unit tests for release-line support and chart source validation."""

from __future__ import annotations

import unittest

from release_policy import release_branch_for_version, release_line_key, supported_release_branches


class ReleasePolicyTests(unittest.TestCase):
    def test_numeric_order_handles_zero_minor_lines(self) -> None:
        branches = ["release/v0.9", "release/v0.10", "release/v0.11", "release/v0.12"]

        self.assertEqual(
            supported_release_branches(branches),
            ["release/v0.10", "release/v0.11", "release/v0.12"],
        )

    def test_boundary_counts_three_actual_release_lines_across_one_zero(self) -> None:
        branches = ["release/v0.12", "release/v1.0", "release/v1.1", "release/v1.2"]

        self.assertEqual(
            supported_release_branches(branches),
            ["release/v1.0", "release/v1.1", "release/v1.2"],
        )

    def test_ignores_malformed_release_branches(self) -> None:
        branches = ["release/v0.9", "release/main", "release/v1", "release/v1.0.0"]

        self.assertEqual(supported_release_branches(branches), ["release/v0.9"])
        self.assertIsNone(release_line_key("release/main"))

    def test_release_version_maps_to_correct_line_including_release_candidates(self) -> None:
        self.assertEqual(release_branch_for_version("0.12.4"), "release/v0.12")
        self.assertEqual(release_branch_for_version("2.3.0-rc.2"), "release/v2.3")

    def test_release_version_rejects_non_semver_and_other_prereleases(self) -> None:
        for version in ("latest", "1.2", "1.2.3-beta.1", "v1.2.3"):
            with self.subTest(version=version), self.assertRaises(ValueError):
                release_branch_for_version(version)


if __name__ == "__main__":
    unittest.main()
