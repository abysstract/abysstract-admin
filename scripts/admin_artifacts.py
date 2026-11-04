#!/usr/bin/env python3
"""Pack and verify private frontend package artifacts.

The first distribution form is a private npm tarball. This command records
version, sha256, source identity, build environment and retention. It does not
publish, tag, or push.

Exit codes:
  0: pack or verify succeeded
  1: checksum, private flag, content, or policy failure
  2: input error
"""

from __future__ import annotations

import argparse
import hashlib
import io
import json
import re
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
from datetime import datetime, timezone
from typing import Any, Mapping, Sequence


PACKAGES = (
    ("@abysstract/admin-proto", Path("frontend/packages/admin-proto")),
    ("@abysstract/admin-core", Path("frontend/packages/admin-core")),
)

DEFAULT_RETENTION_DAYS = 90
FORBIDDEN_NAMES = {".env", ".npmrc", "id_rsa", "credentials.json"}
FORBIDDEN_SUBSTRINGS = ("password=", "ADMIN_DB_DSN=", "ADMIN_PG_DSN=", "ADMIN_JWT_SECRET=", "BEGIN OPENSSH PRIVATE KEY")
FORBIDDEN_PATH_PARTS = ("backend/internal/modules/", "frontend/apps", "backend/internal/service")


def forbidden_file_name(name: str) -> bool:
    base = Path(name).name
    return base in FORBIDDEN_NAMES or base.startswith(".env")


def inspect_secret_bytes(origin: str, name: str, data: bytes) -> None:
    if forbidden_file_name(name):
        raise CheckFailure(f"{origin} contains forbidden file {name}")
    if any(part in name.replace("\\", "/") for part in FORBIDDEN_PATH_PARTS):
        raise CheckFailure(f"{origin} contains business or app source {name}")
    if b"\0" in data[:1024]:
        return
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        return
    lowered = text.lower()
    for needle in FORBIDDEN_SUBSTRINGS:
        if needle.lower() in lowered:
            raise CheckFailure(f"{origin} contains secret material in {name}")


def inspect_source_file(root: Path, path: Path) -> None:
    rel = path.relative_to(root).as_posix()
    inspect_secret_bytes("source tree", rel, path.read_bytes())


class InputError(Exception):
    pass


class CheckFailure(Exception):
    pass


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _run(cmd: Sequence[str], cwd: Path) -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(cmd, cwd=cwd, check=False, capture_output=True, text=True)
    except FileNotFoundError:
        return subprocess.CompletedProcess(cmd, 127, stdout="", stderr=f"{cmd[0]} unavailable")


SOURCE_DIR_EXCLUDES = {"node_modules", "dist", "build", "bin", "__pycache__"}
SOURCE_SUFFIX_EXCLUDES = {".tgz", ".tsbuildinfo"}


def git_head(root: Path) -> str:
    proc = _run(["git", "rev-parse", "HEAD"], root)
    if proc.returncode == 0:
        return proc.stdout.strip()
    return ""


def package_source_files(root: Path) -> list[Path]:
    files: list[Path] = []
    for _, relative in PACKAGES:
        package_dir = root / relative
        if not package_dir.is_dir():
            continue
        for path in package_dir.rglob("*"):
            if not path.is_file():
                continue
            parts = set(path.relative_to(package_dir).parts)
            if parts & SOURCE_DIR_EXCLUDES:
                continue
            if path.suffix in SOURCE_SUFFIX_EXCLUDES or path.name.endswith(".tgz"):
                continue
            files.append(path)
    return sorted(files)


SOURCE_PATH_PREFIXES = (
    "frontend/packages/admin-core/",
    "frontend/packages/admin-proto/",
)
SOURCE_ARCHIVE_NAME = "admin-private-source.tar.gz"
SOURCE_MANIFEST_NAME = "admin-private-source-manifest.json"
VERSION_A_ARCHIVE_SHA256 = "9d15c373864d949fec223ea6978cf910fb377c87a57f763a6e74978586c02cfa"
HEX64 = re.compile(r"^[0-9a-f]{64}$")


def source_file_entries(root: Path) -> list[dict[str, str]]:
    entries = []
    for path in package_source_files(root):
        inspect_source_file(root, path)
        entries.append({"path": path.relative_to(root).as_posix(), "sha256": sha256_file(path)})
    return canonical_source_entries(entries)


def canonical_source_entries(entries: list[Any]) -> list[dict[str, str]]:
    if not isinstance(entries, list) or not entries:
        raise CheckFailure("source manifest files must be a non-empty list")
    cleaned: list[dict[str, str]] = []
    seen: set[str] = set()
    for item in entries:
        if not isinstance(item, dict):
            raise CheckFailure("source manifest file entry must be an object")
        path = str(item.get("path") or "").strip()
        digest = str(item.get("sha256") or "").strip().lower()
        if not path or path.startswith("/") or "\\" in path:
            raise CheckFailure(f"source path {path!r} must be a relative posix path")
        parts = Path(path).parts
        if ".." in parts:
            raise CheckFailure(f"source path escapes the package tree: {path}")
        if not path.startswith(SOURCE_PATH_PREFIXES):
            raise CheckFailure(f"source path is outside the public packages: {path}")
        if not HEX64.fullmatch(digest):
            raise CheckFailure(f"source sha256 for {path} must be 64 lowercase hex characters")
        if path in seen:
            raise CheckFailure(f"duplicate source path {path}")
        seen.add(path)
        cleaned.append({"path": path, "sha256": digest})
    cleaned.sort(key=lambda entry: entry["path"])
    return cleaned


def source_manifest_digest(entries: list[dict[str, str]]) -> str:
    canonical = canonical_source_entries(entries)
    payload = json.dumps(canonical, sort_keys=True, separators=(",", ":")).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def write_source_archive(root: Path, dest: Path, entries: list[dict[str, str]]) -> str:
    dest.parent.mkdir(parents=True, exist_ok=True)
    with tarfile.open(dest, "w:gz") as archive:
        for entry in entries:
            source = (root / entry["path"]).resolve()
            package_root = (root / "frontend/packages").resolve()
            if package_root not in source.parents:
                raise CheckFailure(f"source file is outside frontend/packages: {entry['path']}")
            if source.is_symlink() or not source.is_file():
                raise CheckFailure(f"source file is missing or a symlink: {entry['path']}")
            data = source.read_bytes()
            inspect_secret_bytes("source tree", entry["path"], data)
            info = tarfile.TarInfo(name=entry["path"])
            info.size = len(data)
            info.mtime = 0
            info.mode = 0o644
            archive.addfile(info, io.BytesIO(data))
    return sha256_file(dest)


def packages_are_clean_tracked(root: Path, entries: list[dict[str, str]]) -> bool:
    rels = [str(relative) for _, relative in PACKAGES]
    listed = _run(["git", "ls-files", "--", *rels], root)
    if listed.returncode != 0:
        return False
    tracked = {line.strip() for line in listed.stdout.splitlines() if line.strip()}
    if not tracked:
        return False
    status = _run(["git", "status", "--porcelain", "--", *rels], root)
    if status.returncode != 0 or status.stdout.strip():
        return False
    source_paths = {entry["path"] for entry in entries}
    return source_paths <= tracked


def build_source_identity(root: Path, dest_dir: Path | None = None) -> dict[str, Any]:
    entries = source_file_entries(root)
    head = git_head(root)
    if packages_are_clean_tracked(root, entries) and head:
        return {"kind": "git-commit", "commit": head, "workspace": "clean-tracked"}
    digest = source_manifest_digest(entries)
    identity: dict[str, Any] = {
        "kind": "workspace-source-manifest",
        "sha256": digest,
        "workspace": "uncommitted",
        "manifest": SOURCE_MANIFEST_NAME,
        "archive": SOURCE_ARCHIVE_NAME,
        "version_a_archive_sha256": VERSION_A_ARCHIVE_SHA256,
    }
    if dest_dir is not None:
        dest_dir.mkdir(parents=True, exist_ok=True)
        (dest_dir / SOURCE_MANIFEST_NAME).write_text(
            json.dumps({"files": entries, "sha256": digest}, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        identity["archive_sha256"] = write_source_archive(root, dest_dir / SOURCE_ARCHIVE_NAME, entries)
    if head:
        identity["git_head"] = head
    return identity


def tool_versions() -> dict[str, str]:
    versions: dict[str, str] = {}
    for name, args in (("go", ["go", "version"]), ("node", ["node", "-v"]), ("buf", ["buf", "--version"])):
        proc = _run(args, Path.cwd())
        versions[name] = (proc.stdout or proc.stderr or f"{name} unavailable").strip().splitlines()[0]
    return versions


def package_metadata(package_dir: Path) -> dict[str, Any]:
    pkg = json.loads((package_dir / "package.json").read_text(encoding="utf-8"))
    if pkg.get("private") is not True:
        raise CheckFailure(f"{pkg.get('name')} must keep private: true")
    return pkg


def pack_one(root: Path, package_dir: Path, dest_dir: Path) -> dict[str, Any]:
    pkg = package_metadata(package_dir)
    dest_dir = dest_dir.resolve()
    dest_dir.mkdir(parents=True, exist_ok=True)
    proc = _run(
        ["npm", "pack", "-w", pkg["name"], "--pack-destination", str(dest_dir)],
        root / "frontend",
    )
    if proc.returncode != 0:
        raise CheckFailure(f"npm pack failed for {pkg['name']}: {(proc.stderr or proc.stdout).strip()}")
    tarballs = sorted(dest_dir.glob("*.tgz"), key=lambda path: path.stat().st_mtime, reverse=True)
    expected = f"{pkg['name'].split('/')[-1]}-{pkg['version']}.tgz"
    match = next((path for path in tarballs if path.name == expected or path.name.endswith(expected)), None)
    if match is None:
        raise CheckFailure(f"packed tarball {expected} was not produced in {dest_dir}")
    return {
        "name": pkg["name"],
        "version": pkg["version"],
        "private": True,
        "file": match.name,
        "sha256": sha256_file(match),
    }


def inspect_tarball(path: Path) -> None:
    with tarfile.open(path, "r:gz") as archive:
        for member in archive.getmembers():
            name = member.name
            if member.issym() or member.islnk():
                raise CheckFailure(f"{path.name} contains a link: {name}")
            if member.isfile():
                extracted = archive.extractfile(member)
                if extracted is None:
                    continue
                inspect_secret_bytes(path.name, name, extracted.read())


def write_manifest(root: Path, dest_dir: Path, packages: list[dict[str, Any]], retention_days: int) -> Path:
    payload = {
        "schema": "abysstract.admin.private-artifacts/v1",
        "created_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "source_identity": build_source_identity(root, dest_dir),
        "retention_days": retention_days,
        "access": {
            "channel": "private-tarball",
            "public_npm_publish": False,
            "go_tag_push": False,
            "buf_publish": False,
            "configured_storage": False,
            "download_permission": "repository-authorized-readers",
        },
        "build": tool_versions(),
        "packages": packages,
    }
    dest = dest_dir / "admin-private-artifacts.json"
    dest.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return dest


def pack_artifacts(root: Path, dest_dir: Path, retention_days: int) -> Path:
    dest_dir.mkdir(parents=True, exist_ok=True)
    frontend = root / "frontend"
    for name, _ in PACKAGES:
        built = _run(["npm", "run", "build", "-w", name], frontend)
        if built.returncode != 0:
            raise CheckFailure(f"npm build failed for {name}: {(built.stderr or built.stdout).strip()}")
    packages: list[dict[str, Any]] = []
    for _, relative in PACKAGES:
        package_dir = root / relative
        if not (package_dir / "package.json").is_file():
            raise InputError(f"missing package.json: {package_dir}")
        with tempfile.TemporaryDirectory(prefix="admin-pack-") as tmp:
            packed = pack_one(root, package_dir, Path(tmp))
            target = dest_dir / packed["file"]
            shutil.copy2(Path(tmp) / packed["file"], target)
            packed["sha256"] = sha256_file(target)
            inspect_tarball(target)
            packages.append(packed)
    return write_manifest(root, dest_dir, packages, retention_days)


def verify_workspace_source(identity: Mapping[str, Any], listed: Mapping[str, Any], base: Path) -> None:
    entries = canonical_source_entries(listed.get("files") or [])
    digest = source_manifest_digest(entries)
    if listed.get("sha256") != digest:
        raise CheckFailure("source manifest sha256 does not match the canonical files digest")
    if identity.get("sha256") != digest:
        raise CheckFailure("source_identity sha256 does not match the canonical files digest")
    archive_name = str(identity.get("archive") or SOURCE_ARCHIVE_NAME)
    archive = base / archive_name
    if not archive.is_file():
        raise CheckFailure(f"missing recoverable source archive {archive_name}")
    archive_digest = sha256_file(archive)
    expected_archive = identity.get("archive_sha256")
    if expected_archive and expected_archive != archive_digest:
        raise CheckFailure("source archive sha256 does not match source_identity")
    by_path = {entry["path"]: entry["sha256"] for entry in entries}
    seen: set[str] = set()
    with tarfile.open(archive, "r:gz") as tar:
        for member in tar.getmembers():
            if member.issym() or member.islnk():
                raise CheckFailure(f"source archive contains a link: {member.name}")
            if not member.isfile():
                continue
            name = member.name.lstrip("./")
            if ".." in Path(name).parts or name.startswith("/"):
                raise CheckFailure(f"source archive path escapes: {member.name}")
            extracted = tar.extractfile(member)
            if extracted is None:
                raise CheckFailure(f"source archive member cannot be read: {name}")
            data = extracted.read()
            inspect_secret_bytes(archive_name, name, data)
            digest = hashlib.sha256(data).hexdigest()
            expected = by_path.get(name)
            if expected is None:
                raise CheckFailure(f"source archive contains extra file {name}")
            if digest != expected:
                raise CheckFailure(f"source archive bytes for {name} do not match the files digest")
            seen.add(name)
    missing = sorted(set(by_path) - seen)
    if missing:
        raise CheckFailure("source archive omitted files: " + ", ".join(missing))


def verify_artifacts(manifest_path: Path, expected: Mapping[str, str] | None = None) -> None:
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if manifest.get("schema") != "abysstract.admin.private-artifacts/v1":
        raise CheckFailure("unsupported artifact manifest schema")
    if manifest.get("access", {}).get("public_npm_publish") is True:
        raise CheckFailure("manifest must not enable public npm publish")
    identity = manifest.get("source_identity")
    if isinstance(identity, str):
        raise CheckFailure("source_identity must not be a bare git commit; record a recoverable source manifest or a clean tracked commit")
    if not isinstance(identity, dict) or identity.get("kind") not in {"git-commit", "workspace-source-manifest"}:
        raise CheckFailure("source_identity must declare kind git-commit or workspace-source-manifest")
    if identity.get("kind") == "git-commit" and not identity.get("commit"):
        raise CheckFailure("git-commit source_identity is missing commit")
    if identity.get("kind") == "workspace-source-manifest":
        if identity.get("workspace") != "uncommitted":
            raise CheckFailure("workspace source_identity must record workspace=uncommitted")
        if not identity.get("sha256"):
            raise CheckFailure("workspace source_identity is missing sha256")
        sidecar = manifest_path.parent / (identity.get("manifest") or "admin-private-source-manifest.json")
        if not sidecar.is_file():
            raise CheckFailure(f"missing source manifest {sidecar.name}")
        listed = json.loads(sidecar.read_text(encoding="utf-8"))
        verify_workspace_source(identity, listed, manifest_path.parent)
    base = manifest_path.parent
    packages = manifest.get("packages") or []
    if len(packages) != 2:
        raise CheckFailure("manifest must record both frontend packages")
    for item in packages:
        path = base / item["file"]
        if not path.is_file():
            raise CheckFailure(f"missing packed file {path.name}")
        digest = sha256_file(path)
        if digest != item.get("sha256"):
            raise CheckFailure(f"checksum mismatch for {path.name}: expected {item.get('sha256')} got {digest}")
        if expected and item["name"] in expected and expected[item["name"]] != digest:
            raise CheckFailure(f"checksum rejected for {item['name']}")
        inspect_tarball(path)
        with tarfile.open(path, "r:gz") as archive:
            pkg_file = archive.extractfile("package/package.json")
            if pkg_file is None:
                raise CheckFailure(f"{path.name} missing package/package.json")
            pkg = json.loads(pkg_file.read().decode("utf-8"))
            if pkg.get("private") is not True:
                raise CheckFailure(f"{item['name']} tarball lost private: true")
            if pkg.get("name") != item["name"] or pkg.get("version") != item["version"]:
                raise CheckFailure(f"{path.name} package identity does not match manifest")


def verify_checksum_only(path: Path, expected: str) -> None:
    digest = sha256_file(path)
    if digest != expected:
        raise CheckFailure(f"checksum rejected for {path.name}: expected {expected} got {digest}")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    pack = sub.add_parser("pack", help="create private tarballs and a checksum manifest")
    pack.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parents[1])
    pack.add_argument("--dest", type=Path, required=True)
    pack.add_argument("--retention-days", type=int, default=DEFAULT_RETENTION_DAYS)
    verify = sub.add_parser("verify", help="verify packed tarballs against a manifest")
    verify.add_argument("--manifest", type=Path, required=True)
    checksum = sub.add_parser("check-file", help="reject a file whose sha256 does not match")
    checksum.add_argument("--file", type=Path, required=True)
    checksum.add_argument("--sha256", required=True)
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        if args.command == "pack":
            manifest = pack_artifacts(args.repo_root.resolve(), args.dest.resolve(), args.retention_days)
            print(manifest)
            return 0
        if args.command == "verify":
            verify_artifacts(args.manifest.resolve())
            print(f"verified {args.manifest}")
            return 0
        verify_checksum_only(args.file.resolve(), args.sha256)
        print(f"verified {args.file}")
        return 0
    except InputError as exc:
        print(f"input error: {exc}", file=sys.stderr)
        return 2
    except CheckFailure as exc:
        print(f"artifact check failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
