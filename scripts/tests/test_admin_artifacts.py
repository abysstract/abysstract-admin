from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("admin_artifacts", ROOT / "scripts/admin_artifacts.py")
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class ArtifactTests(unittest.TestCase):
    def make_tarball(self, directory: Path, name: str, pkg: dict, extra: dict[str, bytes] | None = None) -> Path:
        src = directory / "src"
        src.mkdir(exist_ok=True)
        (src / "package.json").write_text(json.dumps(pkg), encoding="utf-8")
        (src / "index.js").write_text("export const ok = true;\n", encoding="utf-8")
        for filename, content in (extra or {}).items():
            path = src / filename
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(content)
        tarball = directory / name
        with tarfile.open(tarball, "w:gz") as archive:
            archive.add(src, arcname="package")
        return tarball

    def test_wrong_checksum_is_rejected(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        tarball = self.make_tarball(
            root,
            "abysstract-admin-core-0.0.0-dev.0.tgz",
            {"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}},
        )
        with self.assertRaises(MODULE.CheckFailure):
            MODULE.verify_checksum_only(tarball, "0" * 64)

    def test_matching_checksum_is_accepted(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        tarball = self.make_tarball(
            root,
            "abysstract-admin-core-0.0.0-dev.0.tgz",
            {"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}},
        )
        MODULE.verify_checksum_only(tarball, MODULE.sha256_file(tarball))

    def test_secret_and_business_source_are_rejected(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        tarball = self.make_tarball(
            root,
            "abysstract-admin-core-0.0.0-dev.0.tgz",
            {"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}},
            extra={".env": b"ADMIN_DB_DSN=host=localhost password=secret\n"},
        )
        with self.assertRaises(MODULE.CheckFailure):
            MODULE.inspect_tarball(tarball)

    def test_private_tarball_is_rejected(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        tarball = self.make_tarball(
            root,
            "abysstract-admin-core-0.0.0-dev.0.tgz",
            {"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "private": True},
        )
        manifest = {
            "schema": "abysstract.admin.private-artifacts/v1",
            "access": {"public_npm_publish": False},
            "source_identity": {"kind": "git-commit", "commit": "abc", "workspace": "clean-tracked"},
            "packages": [
                {
                    "name": "@abysstract/admin-core",
                    "version": "0.0.0-dev.0",
                    "file": tarball.name,
                    "sha256": MODULE.sha256_file(tarball),
                },
                {
                    "name": "@abysstract/admin-proto",
                    "version": "0.0.0-dev.0",
                    "file": tarball.name,
                    "sha256": MODULE.sha256_file(tarball),
                },
            ],
        }
        path = root / "admin-private-artifacts.json"
        path.write_text(json.dumps(manifest), encoding="utf-8")
        with self.assertRaises(MODULE.CheckFailure):
            MODULE.verify_artifacts(path)

    def test_cli_check_file_exit_codes(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        tarball = self.make_tarball(
            root,
            "abysstract-admin-proto-0.0.0-dev.0.tgz",
            {"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}},
        )
        self.assertEqual(MODULE.main(["check-file", "--file", str(tarball), "--sha256", MODULE.sha256_file(tarball)]), 0)
        self.assertEqual(MODULE.main(["check-file", "--file", str(tarball), "--sha256", "abcd"]), 1)

    def test_uncommitted_packages_use_source_manifest_not_head(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        self._git(root, "init")
        self._git(root, "config", "user.email", "checks@example.invalid")
        self._git(root, "config", "user.name", "checks")
        (root / "README").write_text("head only\n", encoding="utf-8")
        self._git(root, "add", "README")
        self._git(root, "commit", "-m", "head")
        head = MODULE.git_head(root)
        core = root / "frontend/packages/admin-core"
        proto = root / "frontend/packages/admin-proto"
        core.mkdir(parents=True)
        proto.mkdir(parents=True)
        (core / "package.json").write_text(json.dumps({"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}), encoding="utf-8")
        (proto / "package.json").write_text(json.dumps({"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}), encoding="utf-8")
        dest = root / "out"
        identity = MODULE.build_source_identity(root, dest)
        self.assertEqual(identity["kind"], "workspace-source-manifest")
        self.assertEqual(identity["workspace"], "uncommitted")
        self.assertNotEqual(identity["sha256"], head)
        self.assertEqual(identity.get("git_head"), head)
        sidecar = dest / "admin-private-source-manifest.json"
        self.assertTrue(sidecar.is_file())
        listed = json.loads(sidecar.read_text(encoding="utf-8"))
        self.assertEqual(listed["sha256"], identity["sha256"])
        self.assertEqual(listed["sha256"], MODULE.source_manifest_digest(listed["files"]))
        self.assertTrue(any(item["path"].endswith("admin-core/package.json") for item in listed["files"]))
        archive = dest / "admin-private-source.tar.gz"
        self.assertTrue(archive.is_file())
        self.assertEqual(identity.get("archive"), "admin-private-source.tar.gz")
        self.assertEqual(identity.get("archive_sha256"), MODULE.sha256_file(archive))
        self.assertEqual(
            identity.get("version_a_archive_sha256"),
            "9d15c373864d949fec223ea6978cf910fb377c87a57f763a6e74978586c02cfa",
        )

    def test_clean_tracked_packages_use_git_commit(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        self._git(root, "init")
        self._git(root, "config", "user.email", "checks@example.invalid")
        self._git(root, "config", "user.name", "checks")
        for name in ("admin-core", "admin-proto"):
            pkg = root / "frontend/packages" / name
            pkg.mkdir(parents=True)
            (pkg / "package.json").write_text(
                json.dumps({"name": f"@abysstract/{name}", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}),
                encoding="utf-8",
            )
        self._git(root, "add", "frontend/packages")
        self._git(root, "commit", "-m", "packages")
        identity = MODULE.build_source_identity(root, None)
        self.assertEqual(identity, {"kind": "git-commit", "commit": MODULE.git_head(root), "workspace": "clean-tracked"})

    def test_bare_commit_source_identity_is_rejected(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        tarball = self.make_tarball(
            root,
            "abysstract-admin-core-0.0.0-dev.0.tgz",
            {"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}},
        )
        manifest = {
            "schema": "abysstract.admin.private-artifacts/v1",
            "access": {"public_npm_publish": False},
            "source_identity": "0628f99e8848534ba3e85e504fdc587557ee3168",
            "packages": [
                {"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "file": tarball.name, "sha256": MODULE.sha256_file(tarball)},
                {"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "file": tarball.name, "sha256": MODULE.sha256_file(tarball)},
            ],
        }
        path = root / "admin-private-artifacts.json"
        path.write_text(json.dumps(manifest), encoding="utf-8")
        with self.assertRaises(MODULE.CheckFailure):
            MODULE.verify_artifacts(path)

    def test_tampered_source_file_digest_is_rejected(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        self._git(root, "init")
        self._git(root, "config", "user.email", "checks@example.invalid")
        self._git(root, "config", "user.name", "checks")
        (root / "README").write_text("head only\n", encoding="utf-8")
        self._git(root, "add", "README")
        self._git(root, "commit", "-m", "head")
        core = root / "frontend/packages/admin-core"
        proto = root / "frontend/packages/admin-proto"
        core.mkdir(parents=True)
        proto.mkdir(parents=True)
        (core / "package.json").write_text(json.dumps({"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}), encoding="utf-8")
        (proto / "package.json").write_text(json.dumps({"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}), encoding="utf-8")
        dest = root / "out"
        identity = MODULE.build_source_identity(root, dest)
        tarball = self.make_tarball(
            dest,
            "abysstract-admin-core-0.0.0-dev.0.tgz",
            {"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}},
        )
        proto_tar = self.make_tarball(
            dest,
            "abysstract-admin-proto-0.0.0-dev.0.tgz",
            {"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}},
        )
        manifest = {
            "schema": "abysstract.admin.private-artifacts/v1",
            "access": {"public_npm_publish": False},
            "source_identity": identity,
            "packages": [
                {"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "file": tarball.name, "sha256": MODULE.sha256_file(tarball)},
                {"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "file": proto_tar.name, "sha256": MODULE.sha256_file(proto_tar)},
            ],
        }
        path = dest / "admin-private-artifacts.json"
        path.write_text(json.dumps(manifest), encoding="utf-8")
        MODULE.verify_artifacts(path)
        sidecar = dest / "admin-private-source-manifest.json"
        listed = json.loads(sidecar.read_text(encoding="utf-8"))
        original = listed["files"][0]["sha256"]
        listed["files"][0]["sha256"] = "0" * 64
        sidecar.write_text(json.dumps(listed), encoding="utf-8")
        with self.assertRaises(MODULE.CheckFailure) as raised:
            MODULE.verify_artifacts(path)
        self.assertIn("canonical files digest", str(raised.exception))
        listed["files"][0]["sha256"] = original
        sidecar.write_text(json.dumps(listed), encoding="utf-8")
        MODULE.verify_artifacts(path)

    def test_missing_source_archive_is_rejected(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        self._git(root, "init")
        self._git(root, "config", "user.email", "checks@example.invalid")
        self._git(root, "config", "user.name", "checks")
        (root / "README").write_text("head only\n", encoding="utf-8")
        self._git(root, "add", "README")
        self._git(root, "commit", "-m", "head")
        core = root / "frontend/packages/admin-core"
        proto = root / "frontend/packages/admin-proto"
        core.mkdir(parents=True)
        proto.mkdir(parents=True)
        (core / "package.json").write_text(json.dumps({"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}), encoding="utf-8")
        (proto / "package.json").write_text(json.dumps({"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}), encoding="utf-8")
        dest = root / "out"
        identity = MODULE.build_source_identity(root, dest)
        (dest / "admin-private-source.tar.gz").unlink()
        tarball = self.make_tarball(
            dest,
            "abysstract-admin-core-0.0.0-dev.0.tgz",
            {"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}},
        )
        proto_tar = self.make_tarball(
            dest,
            "abysstract-admin-proto-0.0.0-dev.0.tgz",
            {"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}},
        )
        manifest = {
            "schema": "abysstract.admin.private-artifacts/v1",
            "access": {"public_npm_publish": False},
            "source_identity": identity,
            "packages": [
                {"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "file": tarball.name, "sha256": MODULE.sha256_file(tarball)},
                {"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "file": proto_tar.name, "sha256": MODULE.sha256_file(proto_tar)},
            ],
        }
        path = dest / "admin-private-artifacts.json"
        path.write_text(json.dumps(manifest), encoding="utf-8")
        with self.assertRaises(MODULE.CheckFailure) as raised:
            MODULE.verify_artifacts(path)
        self.assertIn("source archive", str(raised.exception))

    def test_source_archive_rejects_env_secret(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        self._git(root, "init")
        self._git(root, "config", "user.email", "checks@example.invalid")
        self._git(root, "config", "user.name", "checks")
        (root / "README").write_text("head only\n", encoding="utf-8")
        self._git(root, "add", "README")
        self._git(root, "commit", "-m", "head")
        core = root / "frontend/packages/admin-core"
        proto = root / "frontend/packages/admin-proto"
        core.mkdir(parents=True)
        proto.mkdir(parents=True)
        (core / "package.json").write_text(json.dumps({"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}), encoding="utf-8")
        (proto / "package.json").write_text(json.dumps({"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}), encoding="utf-8")
        (core / ".env").write_text("ADMIN_JWT_SECRET=review_dummy_secret\n", encoding="utf-8")
        dest = root / "artifacts"
        with self.assertRaises(MODULE.CheckFailure) as raised:
            MODULE.build_source_identity(root, dest)
        self.assertIn("forbidden file", str(raised.exception))
        self.assertTrue("ADMIN_JWT_SECRET" in str(raised.exception) or ".env" in str(raised.exception))
        self.assertFalse((dest / "admin-private-source.tar.gz").exists())

    def test_source_archive_rejects_secret_content(self) -> None:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        self._git(root, "init")
        self._git(root, "config", "user.email", "checks@example.invalid")
        self._git(root, "config", "user.name", "checks")
        (root / "README").write_text("head only\n", encoding="utf-8")
        self._git(root, "add", "README")
        self._git(root, "commit", "-m", "head")
        core = root / "frontend/packages/admin-core"
        proto = root / "frontend/packages/admin-proto"
        core.mkdir(parents=True)
        proto.mkdir(parents=True)
        (core / "package.json").write_text(json.dumps({"name": "@abysstract/admin-core", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}), encoding="utf-8")
        (proto / "package.json").write_text(json.dumps({"name": "@abysstract/admin-proto", "version": "0.0.0-dev.0", "publishConfig": {"access": "public"}}), encoding="utf-8")
        (core / "notes.txt").write_text("ADMIN_JWT_SECRET=review_dummy_secret\n", encoding="utf-8")
        dest = root / "artifacts"
        with self.assertRaises(MODULE.CheckFailure) as raised:
            MODULE.build_source_identity(root, dest)
        self.assertIn("secret material", str(raised.exception))
        self.assertFalse((dest / "admin-private-source.tar.gz").exists())

    def _git(self, root: Path, *args: str) -> None:
        import subprocess
        proc = subprocess.run(["git", *args], cwd=root, check=False, capture_output=True, text=True)
        if proc.returncode != 0:
            self.fail(proc.stderr or proc.stdout or "git failed")

    def test_publish_config_must_be_public(self) -> None:
        with self.assertRaises(MODULE.CheckFailure):
            MODULE.check_publish_config({"name": "@abysstract/admin-core"}, "admin-core")
        with self.assertRaises(MODULE.CheckFailure):
            MODULE.check_publish_config({"private": True, "publishConfig": {"access": "public"}}, "admin-core")
        MODULE.check_publish_config({"publishConfig": {"access": "public"}}, "admin-core")

    def test_missing_tool_is_recorded_unavailable(self) -> None:
        proc = MODULE._run(["definitely-missing-admin-core-tool"], Path.cwd())
        self.assertEqual(proc.returncode, 127)
        self.assertIn("unavailable", proc.stderr)
