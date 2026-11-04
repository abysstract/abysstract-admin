from __future__ import annotations

import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("check_admin_ci_policy", ROOT / "scripts/check_admin_ci_policy.py")
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class CIPolicyTests(unittest.TestCase):
    def test_forbidden_publish_is_rejected(self) -> None:
        errors = MODULE.check_workflow(
            "permissions:\n  contents: read\njobs:\n  release:\n    steps:\n      - run: npm publish\n",
            Path(".github/workflows/ci.yml"),
        )
        self.assertTrue(any("npm publish" in error for error in errors), errors)

    def test_fork_pr_target_is_rejected(self) -> None:
        errors = MODULE.check_workflow(
            "on: pull_request_target\npermissions:\n  contents: read\n",
            Path(".github/workflows/ci.yml"),
        )
        self.assertTrue(any("pull_request_target" in error for error in errors), errors)

    def test_read_only_workflow_passes(self) -> None:
        errors = MODULE.check_workflow(
            "name: CI\npermissions:\n  contents: read\non:\n  pull_request:\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo ok\n",
            Path(".github/workflows/ci.yml"),
        )
        self.assertEqual(errors, [])

    def test_current_workflow_file(self) -> None:
        self.assertEqual(MODULE.main(["--repo-root", str(ROOT)]), 0)
