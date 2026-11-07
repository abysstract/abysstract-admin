from __future__ import annotations

import importlib.util
import sys
import json
from pathlib import Path
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("check_admin_consumer", ROOT / "scripts/check_admin_consumer.py")
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class ConsumerBoundaryTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        (self.root / "backend").mkdir()
        (self.root / "proto").mkdir()
        (self.root / "frontend" / "src").mkdir(parents=True)
        (self.root / "frontend" / "packages").mkdir(parents=True)
        (self.root / "integration" / "consumer").mkdir(parents=True)
        self.consumer = self.root / "integration" / "consumer"

    def tearDown(self) -> None:
        self.temp.cleanup()

    def write(self, relative: str, text: str) -> Path:
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def test_go_public_import_and_business_internal_are_allowed(self) -> None:
        self.write(
            "go.mod",
            "module github.com/abysstract/abysstract-admin\n",
        )
        self.write(
            "integration/consumer/backend/go.mod",
            "module example.com/business\n\n"
            "replace github.com/abysstract/abysstract-admin => ../../..\n",
        )
        self.write(
            "integration/consumer/backend/main.go",
            "package main\n"
            "import (\n"
            "  _ \"github.com/abysstract/abysstract-admin/backend/resource\"\n"
            "  _ \"example.com/business/internal/product\"\n"
            ")\n",
        )
        self.write("integration/consumer/backend/internal/product/product.go", "package product\n")
        self.assertEqual(MODULE.check_references(self.root, self.consumer), [])

    def test_system_internal_import_in_single_line_alias_blank_and_dot_forms_is_rejected(self) -> None:
        aliases = ("alias", "_", ".", "别名", "Δ")
        source = "package main\n" + "\n".join(
            f'import {alias} "github.com/abysstract/abysstract-admin/backend/internal/repo"'
            for alias in aliases
        ) + "\n"
        self.write("integration/consumer/backend/single_line.go", source)
        violations = MODULE.check_references(self.root, self.consumer)
        self.assertEqual(len(violations), len(aliases))
        self.assertEqual([violation.line for violation in violations], list(range(2, 2 + len(aliases))))
        self.assertTrue(all("backend/internal/repo" in violation.message for violation in violations))

    def test_system_internal_import_in_alias_group_and_comment_is_rejected(self) -> None:
        self.write(
            "integration/consumer/backend/main_test.go",
            "package main\n"
            "// import \"github.com/abysstract/abysstract-admin/backend/internal/fake\"\n"
            "import (\n"
            "  internalpkg \"github.com/abysstract/abysstract-admin/backend/internal/repo\"\n"
            ")\n",
        )
        violations = MODULE.check_references(self.root, self.consumer)
        self.assertEqual(len(violations), 1)
        self.assertIn("backend/internal/repo", violations[0].message)
        self.assertEqual(violations[0].line, 4)

    def test_proto_system_name_is_rejected_and_business_name_is_allowed(self) -> None:
        self.write("integration/consumer/proto/good.proto", "syntax = \"proto3\"; package company.product.v1;\n")
        self.write(
            "integration/consumer/proto/bad.proto",
            "// package abysstract.admin.v1;\nsyntax = \"proto3\";\npackage abysstract.admin.v1;\n",
        )
        violations = MODULE.check_references(self.root, self.consumer)
        self.assertEqual(len(violations), 1)
        self.assertIn("abysstract.admin.v1", violations[0].message)

    def test_frontend_relative_and_tsconfig_alias_to_system_source_are_rejected(self) -> None:
        self.write(
            "integration/consumer/frontend/tsconfig.json",
            '{"compilerOptions":{"paths":{"@system/*":["../../../frontend/src/*"]}}}',
        )
        self.write("frontend/src/private.ts", "export const privateValue = 1;\n")
        self.write(
            "integration/consumer/frontend/page.tsx",
            "import { privateValue } from \"../../../frontend/src/private\";\n"
            "export { privateValue } from \"@system/private\";\n"
            "const lazy = import(\"../../../frontend/src/private\");\n",
        )
        violations = MODULE.check_references(self.root, self.consumer)
        self.assertEqual(len(violations), 3)
        self.assertTrue(all("frontend/src" in violation.message for violation in violations))

    def test_public_frontend_package_and_runtime_computed_path_are_not_rejected(self) -> None:
        self.write(
            "integration/consumer/frontend/page.ts",
            "import core from \"@abysstract/admin-core\";\n"
            "const path = \"../../../frontend/src/private\";\n"
            "export { core };\n",
        )
        self.assertEqual(MODULE.check_references(self.root, self.consumer), [])


    def test_only_exact_system_module_replace_is_allowed(self) -> None:
        self.write("go.mod", "module github.com/abysstract/abysstract-admin\n")
        self.write(
            "integration/consumer/backend/go.mod",
            "module example.com/business\n\n"
            "replace github.com/abysstract/abysstract-admin => ../../..\n",
        )
        self.assertEqual(MODULE.check_references(self.root, self.consumer), [])

        for target in ("../../../backend", "../../../backend/internal"):
            self.write(
                "integration/consumer/backend/go.mod",
                "module example.com/business\n\n"
                f"replace github.com/abysstract/abysstract-admin => {target}\n",
            )
            violations = MODULE.check_references(self.root, self.consumer)
            self.assertEqual(len(violations), 1, target)
            self.assertIn("replace", violations[0].message)

    def test_legacy_backend_module_replace_is_rejected(self) -> None:
        self.write("go.mod", "module github.com/abysstract/abysstract-admin\n")
        self.write(
            "integration/consumer/backend/go.mod",
            "module example.com/business\n\n"
            "replace github.com/abysstract/abysstract-admin/backend => ../../../backend\n",
        )
        violations = MODULE.check_references(self.root, self.consumer)
        self.assertEqual(len(violations), 1)
        self.assertIn("github.com/abysstract/abysstract-admin =>", violations[0].message)

    def test_snapshot_detects_added_modified_deleted_but_ignores_build_outputs(self) -> None:
        source = self.write("backend/resource/store.go", "package resource\n")
        self.write("backend/.cache/ignored.go", "package ignored\n")
        self.write("backend/bin/runtime-output", "binary\n")
        self.write("backend/adminresource-gen", "binary\n")
        self.write("backend/resource/debug.log", "initial\n")
        snapshot_path = self.root / "snapshot.json"
        MODULE.write_snapshot(self.root, snapshot_path)
        source.write_text("package resource\n\n// changed\n", encoding="utf-8")
        self.write("proto/new.proto", "syntax = \"proto3\";\n")
        source.unlink()
        changes = MODULE.compare_snapshot(self.root, snapshot_path)
        self.assertEqual(changes["modified"], [])
        self.assertEqual(changes["added"], ["proto/new.proto"])
        self.assertEqual(changes["deleted"], ["backend/resource/store.go"])
        payload = json.loads(snapshot_path.read_text(encoding="utf-8"))
        self.assertNotIn("backend/.cache/ignored.go", payload["files"])
        self.assertNotIn("backend/bin/runtime-output", payload["files"])
        self.assertNotIn("backend/adminresource-gen", payload["files"])
        self.assertNotIn("backend/resource/debug.log", payload["files"])


if __name__ == "__main__":
    unittest.main()
