from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("check_admin_public_packages", ROOT / "scripts/check_admin_public_packages.py")
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


EXPECTED_PACKAGES = (
    "resource", "module", "permission", "resourcegen", "coremigrations",
    "database", "system", "adminconfig", "identity", "adminapp", "moduleseed",
)


class PublicPackageProbeTests(unittest.TestCase):
    def make_backend(self, packages: dict[str, str] | None = None, extra: str = "") -> tuple[tempfile.TemporaryDirectory, Path]:
        temp = tempfile.TemporaryDirectory()
        root = Path(temp.name)
        backend = root / "backend"
        backend.mkdir()
        (root / "go.mod").write_text(
            "module github.com/abysstract/abysstract-admin\n\n"
            "go 1.25\n" + extra,
            encoding="utf-8",
        )
        for package, source in (packages or {}).items():
            package_dir = backend / package
            package_dir.mkdir(parents=True)
            (package_dir / f"{package.replace('/', '_')}.go").write_text(
                "package " + package.rsplit("/", 1)[-1] + "\n" + source,
                encoding="utf-8",
            )
        return temp, root

    # The k1 fixtures have no backend packages. Keep the module proxy out of
    # the lookup so a published module cannot supply the missing path.
    @mock.patch.dict("os.environ", {"GOPROXY": "off"})
    def test_k1_requires_the_precise_missing_package_diagnostic(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        results = MODULE.run_probe(root, "k1")
        self.assertEqual([(r.package, r.category, r.passed) for r in results],
                         [(package, "missing_package", True) for package in EXPECTED_PACKAGES])

    def test_k2_accepts_all_importable_public_packages(self) -> None:
        temp, root = self.make_backend({package: "type Marker struct{}\n" for package in EXPECTED_PACKAGES})
        self.addCleanup(temp.cleanup)
        results = MODULE.run_probe(root, "k2")
        self.assertEqual([(r.package, r.category, r.passed) for r in results],
                         [(package, "ok", True) for package in EXPECTED_PACKAGES])

    def test_missing_dependency_is_not_reported_as_expected_k1_missing_package(self) -> None:
        temp, root = self.make_backend(
            {"resource": 'import _ "example.com/unavailable/module"\n'},
        )
        self.addCleanup(temp.cleanup)
        result = MODULE.run_probe(root, "k2")[0]
        self.assertNotEqual(result.category, "missing_package")
        self.assertEqual(result.category, "dependency")
        self.assertFalse(result.passed)

    def test_existing_package_reports_build_failure(self) -> None:
        temp, root = self.make_backend({package: "type Marker struct{}\n" for package in EXPECTED_PACKAGES})
        self.addCleanup(temp.cleanup)
        results = MODULE.run_probe(root, "k1")
        self.assertEqual([(r.package, r.category, r.passed) for r in results],
                         [(package, "ok", False) for package in EXPECTED_PACKAGES])

    def make_fake_go(self, root: Path, body: str) -> Path:
        fake = root / "fake-go"
        fake.write_text(
            "#!/bin/sh\n"
            "if [ \"$1\" = \"version\" ]; then echo 'go1.26.4'; exit 0; fi\n"
            + body,
            encoding="utf-8",
        )
        fake.chmod(0o755)
        return fake

    def test_missing_target_directory_requires_target_diagnostic(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        fake = self.make_fake_go(root, "echo 'dial tcp: lookup proxy.golang.org: no such host' >&2; exit 1\n")
        result = MODULE.probe_package(root, "resource", "k1", go_bin=str(fake))
        self.assertEqual(result.category, "dependency")
        self.assertFalse(result.passed)

    def test_missing_target_directory_with_syntax_error_is_not_missing_package(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        fake = self.make_fake_go(root, "echo 'resource/bad.go: syntax error: unexpected token' >&2; exit 1\n")
        result = MODULE.probe_package(root, "resource", "k1", go_bin=str(fake))
        self.assertEqual(result.category, "syntax")
        self.assertFalse(result.passed)

    def test_missing_target_directory_with_timeout_is_not_missing_package(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        fake = self.make_fake_go(root, "sleep 1\n")
        result = MODULE.probe_package(root, "resource", "k1", go_bin=str(fake), timeout=0.01)
        self.assertEqual(result.category, "timeout")
        self.assertFalse(result.passed)

    @mock.patch.dict("os.environ", {"GOPROXY": "off"})
    def test_clean_missing_package_accepts_tls_in_replacement_path_as_data(self) -> None:
        with tempfile.TemporaryDirectory(prefix="admin-probe-tls-fixture-") as temp_name:
            root = Path(temp_name)
            (root / "backend").mkdir()
            (root / "go.mod").write_text(
                "module github.com/abysstract/abysstract-admin\n\ngo 1.25\n",
                encoding="utf-8",
            )
            work = root / "probe-work"
            work.mkdir()
            result = MODULE.probe_package(root, "resource", "k1", temp_root=work)
        self.assertEqual(result.category, "missing_package")
        self.assertTrue(result.passed)

    def test_target_lookup_http_forbidden_is_not_missing_package(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        target = "github.com/abysstract/abysstract-admin/backend/resource"
        fake = self.make_fake_go(
            root,
            f'echo "cannot find module providing package {target}: reading https://modules.company.example/example/@v/list: 403 Forbidden" >&2; exit 1\n',
        )
        result = MODULE.probe_package(root, "resource", "k1", go_bin=str(fake))
        self.assertNotEqual(result.category, "missing_package")
        self.assertFalse(result.passed)

    def test_target_lookup_permission_error_is_not_missing_package(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        target = "github.com/abysstract/abysstract-admin/backend/resource"
        fake = self.make_fake_go(
            root,
            f'echo "cannot find module providing package {target}: open /tmp/review-cache: permission denied" >&2; exit 1\n',
        )
        result = MODULE.probe_package(root, "resource", "k1", go_bin=str(fake))
        self.assertNotEqual(result.category, "missing_package")
        self.assertFalse(result.passed)

    def test_target_missing_with_unknown_compile_line_is_not_missing_package(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        target = "github.com/abysstract/abysstract-admin/backend/resource"
        # The unknown line must invalidate the positive grammar.
        fake = root / "fake-go"
        fake.write_text(
            "#!/bin/sh\n"
            "if [ \"$1\" = \"version\" ]; then echo go1.26.4; exit 0; fi\n"
            f'printf "module github.com/abysstract/abysstract-admin found, but does not contain package {target}\\nother.go:4:2: undefined: MissingSymbol\\n" >&2; exit 1\n',
            encoding="utf-8",
        )
        fake.chmod(0o755)
        result = MODULE.probe_package(root, "resource", "k1", go_bin=str(fake))
        self.assertNotEqual(result.category, "missing_package")
        self.assertFalse(result.passed)

    def test_target_lookup_failure_with_target_name_is_not_missing_package(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        target = "github.com/abysstract/abysstract-admin/backend/resource"
        fake = self.make_fake_go(
            root,
            f'echo \"cannot find module providing package {target}: Get https://proxy.golang.org/example/@v/list: dial tcp: lookup proxy.golang.org: no such host\" >&2; exit 1\n',
        )
        result = MODULE.probe_package(root, "resource", "k1", go_bin=str(fake))
        self.assertEqual(result.category, "dependency")
        self.assertFalse(result.passed)

    def test_target_missing_diagnostic_with_other_go_sum_error_is_not_missing_package(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        target = "github.com/abysstract/abysstract-admin/backend/resource"
        fake = self.make_fake_go(
            root,
            f'printf \"module {target} found, but does not contain package {target}\\nother.go: missing go.sum entry for module providing package example.com/dep\\n\" >&2; exit 1\n',
        )
        result = MODULE.probe_package(root, "resource", "k1", go_bin=str(fake))
        self.assertEqual(result.category, "dependency")
        self.assertFalse(result.passed)

    def test_missing_target_directory_with_non_target_missing_dependency_is_not_missing_package(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        fake = self.make_fake_go(
            root,
            "echo 'no required module provides package example.com/other/dependency' >&2; exit 1\n",
        )
        result = MODULE.probe_package(root, "resource", "k1", go_bin=str(fake))
        self.assertEqual(result.category, "dependency")
        self.assertFalse(result.passed)

    def test_toolchain_failure_is_distinguished(self) -> None:
        temp, root = self.make_backend()
        self.addCleanup(temp.cleanup)
        fake = Path(temp.name) / "fake-go"
        fake.write_text("#!/bin/sh\necho 'go: toolchain unavailable' >&2\nexit 1\n", encoding="utf-8")
        fake.chmod(0o755)
        result = MODULE.probe_package(root, "resource", "k2", go_bin=str(fake))
        self.assertEqual(result.category, "toolchain")
        self.assertFalse(result.passed)


if __name__ == "__main__":
    unittest.main()
