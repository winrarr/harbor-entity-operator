#!/usr/bin/env python3
"""Unit tests for chart-only release metadata and immutable-tag policy."""

from __future__ import annotations

import unittest

from chart_release_policy import validate_chart_release, validate_chart_tag_target


SUPPORTED = ["release/v0.10", "release/v0.11", "release/v0.12"]


class ChartReleasePolicyTests(unittest.TestCase):
    def test_accepts_main_chart_release_with_supported_stable_operator_image(self) -> None:
        self.assertEqual(
            validate_chart_release("main", "2.4.1", "0.12.3", False, "", SUPPORTED),
            "chart-v2.4.1",
        )

    def test_rejects_unapproved_source_refs_and_non_stable_versions(self) -> None:
        invalid = [
            ("feature/chart", "2.4.1", "0.12.3", False, ""),
            ("release/v0.12.0", "2.4.1", "0.12.3", False, ""),
            ("main", "2.4.1-rc.1", "0.12.3", False, ""),
            ("main", "2.4.1", "0.12.3-rc.1", False, ""),
        ]
        for values in invalid:
            with self.subTest(values=values), self.assertRaises(ValueError):
                validate_chart_release(*values, SUPPORTED)

    def test_unsupported_operator_line_needs_an_explicit_emergency_reason(self) -> None:
        with self.assertRaisesRegex(ValueError, "outside the supported maintenance window"):
            validate_chart_release("main", "2.4.1", "0.9.8", False, "", SUPPORTED)

        self.assertEqual(
            validate_chart_release("main", "2.4.1", "0.9.8", True, "security fix", SUPPORTED),
            "chart-v2.4.1",
        )

    def test_unsupported_source_branch_needs_emergency_approval_even_with_supported_image(self) -> None:
        with self.assertRaisesRegex(ValueError, "source release branch.*outside"):
            validate_chart_release("release/v0.9", "2.4.1", "0.12.3", False, "", SUPPORTED)

        self.assertEqual(
            validate_chart_release("release/v0.9", "2.4.1", "0.12.3", True, "security fix", SUPPORTED),
            "chart-v2.4.1",
        )

    def test_emergency_flag_and_reason_must_be_consistent(self) -> None:
        for override, reason in ((True, "  "), (False, "security fix")):
            with self.subTest(override=override, reason=reason), self.assertRaisesRegex(ValueError, "emergency"):
                validate_chart_release("main", "2.4.1", "0.12.3", override, reason, SUPPORTED)

    def test_chart_tag_must_be_missing_or_point_at_selected_source(self) -> None:
        self.assertFalse(validate_chart_tag_target("chart-v2.4.1", "abc123", ""))
        self.assertTrue(validate_chart_tag_target("chart-v2.4.1", "abc123", "abc123"))
        with self.assertRaisesRegex(ValueError, "immutable chart tag"):
            validate_chart_tag_target("chart-v2.4.1", "abc123", "different")


if __name__ == "__main__":
    unittest.main()
