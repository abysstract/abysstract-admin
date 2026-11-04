from __future__ import annotations

import importlib.util
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("check_admin_generated", ROOT / "scripts/check_admin_generated.py")
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class GeneratedCheckTests(unittest.TestCase):
    def git(self, root: Path, *args: str) -> None:
        proc = subprocess.run(["git", *args], cwd=root, check=False, capture_output=True, text=True)
        if proc.returncode != 0:
            self.fail(proc.stderr or proc.stdout or "git failed")

    def make_repo(self) -> tuple[tempfile.TemporaryDirectory, Path]:
        temp = tempfile.TemporaryDirectory()
        root = Path(temp.name)
        (root / "backend").mkdir()
        (root / "backend/go.mod").write_text("module example.com/admin\n", encoding="utf-8")
        system = root / "backend/internal/resource/gen"
        business = root / "backend/internal/inventorygen"
        system.mkdir(parents=True)
        business.mkdir(parents=True)
        (system / "format.gen.go").write_text("package gen\n", encoding="utf-8")
        (system / ".adminresource-go.manifest").write_text("format.gen.go\n", encoding="utf-8")
        (business / "contract_conv.gen.go").write_text("package gen\n", encoding="utf-8")
        (business / ".adminresource-go.manifest").write_text("contract_conv.gen.go\n", encoding="utf-8")
        self.git(root, "init")
        self.git(root, "config", "user.email", "checks@example.invalid")
        self.git(root, "config", "user.name", "checks")
        self.git(root, "add", ".")
        self.git(root, "commit", "-m", "generated")
        return temp, root

    def test_clean_generated_tree_passes(self) -> None:
        temp, root = self.make_repo()
        self.addCleanup(temp.cleanup)
        self.assertEqual(MODULE.run_checks(root, "all", include_git=True), [])

    def test_hand_edited_managed_file_fails(self) -> None:
        temp, root = self.make_repo()
        self.addCleanup(temp.cleanup)
        target = root / "backend/internal/resource/gen/format.gen.go"
        target.write_text("package gen\n// edited\n", encoding="utf-8")
        errors = MODULE.run_checks(root, "system", include_git=True)
        self.assertTrue(any("differ" in error for error in errors), errors)

    def test_business_resource_drift_uses_its_manifest(self) -> None:
        temp, root = self.make_repo()
        self.addCleanup(temp.cleanup)
        target = root / "backend/internal/inventorygen/contract_conv.gen.go"
        target.write_text("package gen\n// edited\n", encoding="utf-8")
        errors = MODULE.run_checks(root, "business", include_git=True)
        self.assertTrue(any("differ" in error for error in errors), errors)

    def test_module_outputs_are_discovered_without_a_resource_manifest(self) -> None:
        for relative in (
            "backend/internal/modules/sales/model/sales_order.gen.go",
            "backend/internal/modules/sales/query/sales_order.gen.go",
            "backend/internal/modules/sales/gen/proto/company/sales/v1/order.pb.go",
            "frontend/apps/reference/src/modules/sales/gen/proto/company/sales/v1/order_pb.ts",
        ):
            with self.subTest(relative=relative):
                temp, root = self.make_repo()
                self.addCleanup(temp.cleanup)
                target = root / relative
                target.parent.mkdir(parents=True)
                target.write_text("// generated baseline\n", encoding="utf-8")
                self.git(root, "add", ".")
                self.git(root, "commit", "-m", "module outputs")
                target.write_text("// generated drift\n", encoding="utf-8")
                errors = MODULE.run_checks(root, "business", include_git=True)
                self.assertTrue(any("differ" in error for error in errors), errors)

    def test_handwritten_tests_in_generated_package_are_not_outputs(self) -> None:
        temp, root = self.make_repo()
        self.addCleanup(temp.cleanup)
        target = root / "backend/internal/resource/gen/probe_test.go"
        target.write_text("package gen\n", encoding="utf-8")
        self.git(root, "add", ".")
        self.git(root, "commit", "-m", "test fixture")
        target.write_text("package gen\n// updated fixture\n", encoding="utf-8")
        (target.parent / "new_test.go").write_text("package gen\n", encoding="utf-8")
        self.assertEqual(MODULE.run_checks(root, "system", include_git=True), [])

    def test_missing_new_generated_file_fails(self) -> None:
        temp, root = self.make_repo()
        self.addCleanup(temp.cleanup)
        extra = root / "backend/internal/resource/gen/config_conv.gen.go"
        extra.write_text("package gen\n", encoding="utf-8")
        errors = MODULE.run_checks(root, "system", include_git=True)
        self.assertTrue(any("untracked" in error or "omitted" in error for error in errors), errors)

    def test_stale_manifest_fails(self) -> None:
        temp, root = self.make_repo()
        self.addCleanup(temp.cleanup)
        manifest = root / "backend/internal/resource/gen/.adminresource-go.manifest"
        manifest.write_text("format.gen.go\nmissing.gen.go\n", encoding="utf-8")
        errors = MODULE.check_manifests(root, [Path("backend/internal/resource/gen")])
        self.assertTrue(any("stale" in error for error in errors), errors)

    def test_business_output_overlapping_system_fails(self) -> None:
        errors = MODULE.check_overlap(
            [Path("/tmp/repo/backend/internal/resource/gen")],
            [Path("/tmp/repo/backend/internal/resource/gen")],
        )
        self.assertTrue(errors)

    def test_build_artifacts_are_recorded(self) -> None:
        paths = {str(path) for path in MODULE.excluded_build_paths()}
        self.assertIn("frontend/packages/admin-core/dist", paths)
        self.assertIn("frontend/apps/reference/node_modules", paths)

    def test_nested_ts_manifest_missing_fails(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        nested = root / "frontend/apps/reference/src/modules/sales/gen/resource"
        nested.mkdir(parents=True)
        (nested / "contract.gen.ts").write_text("export {}\n", encoding="utf-8")
        errors = MODULE.check_manifests(root, [Path("frontend/apps/reference/src/modules/sales/gen")])
        self.assertTrue(any("missing manifest" in error for error in errors), errors)

    def test_nested_ts_manifest_stale_fails(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        nested = root / "frontend/apps/reference/src/modules/sales/gen/resource"
        nested.mkdir(parents=True)
        (nested / "contract.schema.gen.ts").write_text("export {}\n", encoding="utf-8")
        (nested / ".adminresource-ts.manifest").write_text("contract.schema.gen.ts\nmissing.gen.ts\n", encoding="utf-8")
        errors = MODULE.check_manifests(root, [Path("frontend/apps/reference/src/modules/sales/gen")])
        self.assertTrue(any("stale" in error for error in errors), errors)

    def test_nested_ts_manifest_clean_passes(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        nested = root / "frontend/apps/reference/src/modules/sales/gen/resource"
        nested.mkdir(parents=True)
        (nested / "contract.schema.gen.ts").write_text("export {}\n", encoding="utf-8")
        (nested / ".adminresource-ts.manifest").write_text("contract.schema.gen.ts\n", encoding="utf-8")
        errors = MODULE.check_manifests(root, [Path("frontend/apps/reference/src/modules/sales/gen")])
        self.assertEqual(errors, [])
