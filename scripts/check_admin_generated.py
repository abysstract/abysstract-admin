#!/usr/bin/env python3
"""Check system and business generated outputs for drift and scope errors.

Exit codes:
  0: the selected checks passed
  1: drift, stale manifest, overlap, or missing generated file
  2: input / git / configuration error
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
from typing import Iterable, Sequence


SYSTEM_GENERATED = (
    Path("backend/gen/abysstract"),
    Path("frontend/packages/admin-core/src/gen"),
    Path("frontend/packages/admin-proto/src"),
    Path("backend/internal/resource/gen"),
    Path("backend/internal/dal"),
    Path("backend/internal/resource/schema/testpb"),
    Path("backend/internal/resource/schema/testenum"),
    Path("frontend/tools/fixtures/gen"),
)

BUSINESS_GENERATED = (Path("backend/internal/resourceprobe/query"),)

BUILD_ARTIFACT_PATHS = (
    Path("frontend/packages/admin-core/dist"),
    Path("frontend/packages/admin-proto/dist"),
    Path("frontend/apps/reference/dist"),
    Path("frontend/node_modules"),
    Path("frontend/packages/admin-core/node_modules"),
    Path("frontend/packages/admin-proto/node_modules"),
    Path("frontend/apps/reference/node_modules"),
    Path("backend/bin"),
)

class InputError(Exception):
    pass


def _run_git(root: Path, args: Sequence[str]) -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(
            ["git", *args],
            cwd=root,
            check=False,
            capture_output=True,
            text=True,
        )
    except OSError as exc:
        raise InputError(f"git is not available: {exc}") from exc


def _paths_for_scope(scope: str) -> tuple[Path, ...]:
    if scope == "system":
        return SYSTEM_GENERATED
    if scope == "business":
        return BUSINESS_GENERATED
    if scope == "all":
        return SYSTEM_GENERATED + BUSINESS_GENERATED
    raise InputError(f"unknown scope: {scope}")


def check_overlap(business_outputs: Iterable[Path], system_outputs: Iterable[Path]) -> list[str]:
    errors: list[str] = []
    system = [path.resolve() for path in system_outputs]
    for output in business_outputs:
        resolved = output.resolve()
        for system_path in system:
            if resolved == system_path or system_path in resolved.parents or resolved in system_path.parents:
                errors.append(
                    f"business generated output {output} overlaps system generated directory {system_path}"
                )
    return errors


def _sibling_generated(dir_path: Path, suffix: str) -> list[str]:
    return sorted(path.name for path in dir_path.iterdir() if path.is_file() and path.name.endswith(suffix))


def _compare_manifest(root: Path, manifest: Path, suffix: str) -> list[str]:
    errors: list[str] = []
    generated = _sibling_generated(manifest.parent, suffix)
    listed = [line.strip() for line in manifest.read_text(encoding="utf-8").splitlines() if line.strip()]
    listed_set = set(listed)
    generated_set = set(generated)
    rel = manifest.relative_to(root)
    missing = sorted(generated_set - listed_set)
    extra = sorted(listed_set - generated_set)
    if missing:
        errors.append(f"{rel} omitted generated files: {', '.join(missing)}")
    if extra:
        errors.append(f"{rel} is stale; extra entries: {', '.join(extra)}")
    return errors


def check_manifests(root: Path, directories: Iterable[Path]) -> list[str]:
    errors: list[str] = []
    for directory in directories:
        abs_dir = root / directory
        if not abs_dir.is_dir():
            continue
        for manifest in abs_dir.rglob(".adminresource-go.manifest"):
            if "node_modules" in manifest.parts:
                continue
            errors.extend(_compare_manifest(root, manifest, ".gen.go"))
        for manifest in abs_dir.rglob(".adminresource-ts.manifest"):
            if "node_modules" in manifest.parts:
                continue
            errors.extend(_compare_manifest(root, manifest, ".gen.ts"))
        for generated in abs_dir.rglob("*.gen.ts"):
            if "node_modules" in generated.parts or not generated.is_file():
                continue
            manifest = generated.parent / ".adminresource-ts.manifest"
            if not manifest.exists():
                errors.append(f"missing manifest {manifest.relative_to(root)} for .gen.ts files")
    return sorted(set(errors))


def check_git_drift(root: Path, directories: Sequence[Path]) -> list[str]:
    rels = [str(path) for path in directories] + [":!**/*_test.go"]
    diff = _run_git(root, ["diff", "--exit-code", "--", *rels])
    errors: list[str] = []
    if diff.returncode not in (0, 1):
        raise InputError(diff.stderr.strip() or "git diff failed")
    if diff.returncode == 1:
        errors.append("tracked generated files differ:\n" + (diff.stdout or diff.stderr))
    untracked = _run_git(root, ["ls-files", "--others", "--exclude-standard", "--", *rels])
    if untracked.returncode != 0:
        raise InputError(untracked.stderr.strip() or "git ls-files failed")
    extra = [line for line in untracked.stdout.splitlines() if line.strip()]
    if extra:
        errors.append("untracked generated files:\n" + "\n".join(extra))
    return errors


def excluded_build_paths() -> tuple[Path, ...]:
    return BUILD_ARTIFACT_PATHS


def run_checks(root: Path, scope: str, include_git: bool) -> list[str]:
    business_resources = tuple(
        manifest.parent.relative_to(root)
        for manifest in (root / "backend/internal").rglob(".adminresource-go.manifest")
        if not any(manifest.parent.is_relative_to(root / path) for path in SYSTEM_GENERATED)
    )
    module_outputs = tuple(
        directory.relative_to(root)
        for base in (root / "backend/internal/modules", root / "frontend/apps/reference/src/modules")
        for module in sorted(base.glob("*"))
        for name in ("model", "query", "gen")
        if (directory := module / name).is_dir()
    )
    # Module declarations cover custom output locations as well as conventional directories.
    selection = root / "admin.modules.json"
    declared_outputs: tuple[Path, ...] = ()
    if selection.exists():
        selected = json.loads(selection.read_text())["modules"]
        outputs = []
        for entry in selected:
            declaration = json.loads((root / entry).read_text())
            if declaration["name"] == "system":
                continue
            generation = declaration.get("generation")
            if generation:
                outputs.extend([
                    Path(generation["modelOutput"]) / "model",
                    Path(generation["modelOutput"]) / "query",
                    Path(generation["goOutput"]),
                    Path(generation["tsOutput"]),
                ])
        declared_outputs = tuple(outputs)
        if scope == "all":
            env = os.environ.copy()
            env["GOWORK"] = "off"
            check = subprocess.run(
                ["go", "run", "./cmd/admin", "module", "check"],
                cwd=root / "backend", env=env, capture_output=True, text=True,
            )
            if check.returncode:
                return ["module registration check failed:\n" + check.stdout + check.stderr]

    directories = _paths_for_scope(scope)
    if scope in ("business", "all"):
        directories += business_resources + module_outputs + declared_outputs
    errors = check_overlap(
        (root / path for path in BUSINESS_GENERATED + business_resources + module_outputs + declared_outputs),
        (root / path for path in SYSTEM_GENERATED),
    )
    errors.extend(check_manifests(root, directories))
    if include_git:
        registration = (
            Path("backend/internal/app/modules.gen.go"),
            Path("backend/internal/codegen/resources/tasks.gen.go"),
            Path("backend/internal/codegen/modules.json"),
            Path("backend/migrations/go_registry.gen.go"),
            Path("frontend/apps/reference/src/modules.gen.ts"),
            Path("buf.yaml"),
        ) if selection.exists() and scope == "all" else ()
        errors.extend(check_git_drift(root, directories + registration))
    return errors


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--scope", choices=("system", "business", "all"), default="all")
    parser.add_argument("--skip-git", action="store_true", help="only check manifests and output overlap")
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    root = args.repo_root.resolve()
    if not (root / "backend" / "go.mod").is_file():
        print(f"input error: {root} is not the admin repository root", file=sys.stderr)
        return 2
    try:
        errors = run_checks(root, args.scope, include_git=not args.skip_git)
    except InputError as exc:
        print(f"input error: {exc}", file=sys.stderr)
        return 2
    if errors:
        print("generated-code check failed:", file=sys.stderr)
        for error in errors:
            print(error, file=sys.stderr)
        print("excluded build artifacts:", file=sys.stderr)
        for path in excluded_build_paths():
            print(f"  {path}", file=sys.stderr)
        return 1
    print(f"generated-code check passed scope={args.scope}")
    print("excluded build artifacts:")
    for path in excluded_build_paths():
        print(f"  {path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
