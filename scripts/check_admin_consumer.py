#!/usr/bin/env python3
"""Check an external consumer's dependency boundary and protected source files.

The source scan is intentionally static.  It verifies the import and package
forms that are part of the library boundary; it does not pretend to prove
runtime-computed paths or generated output behavior.

Exit codes:
  0: all requested checks pass
  1: a boundary violation or protected-file change was found
  2: an input, configuration, or snapshot could not be read
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import sys
from dataclasses import dataclass
from typing import Iterable, Iterator, Sequence


SYSTEM_MODULE = "github.com/abysstract/abysstract-admin"
# The module root is the repository root; Go packages live under backend/.
SYSTEM_PUBLIC_PREFIX = SYSTEM_MODULE + "/backend/"
# Module path used before the go.mod moved to the repository root.
LEGACY_SYSTEM_MODULE = SYSTEM_MODULE + "/backend"
SYSTEM_FRONTEND_DIRS = (
    Path("frontend/src"),
    Path("frontend/packages"),
    Path("frontend/apps"),
)
PROTECTED_ROOTS = (
    Path("backend"),
    Path("proto"),
    Path("frontend/src"),
    Path("frontend/packages"),
    Path("frontend/apps"),
)
EXCLUDED_DIRS = {
    ".git",
    ".cache",
    "node_modules",
    "dist",
    "build",
    "bin",
    "uploads",
    "coverage",
    "__pycache__",
}
EXCLUDED_FILE_NAMES = {
    ".DS_Store",
    "adminresource-gen",
}
EXCLUDED_FILE_SUFFIXES = (
    ".log",
    ".tmp",
    ".out",
    ".test",
    ".tsbuildinfo",
)
GO_SUFFIXES = {".go"}
PROTO_SUFFIXES = {".proto"}
FRONTEND_SUFFIXES = {".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}


@dataclass(frozen=True)
class Violation:
    path: str
    line: int
    message: str

    def format(self) -> str:
        return f"{self.path}:{self.line}: {self.message}"


class InputError(Exception):
    """The caller supplied an invalid repository or consumer directory."""


def _strip_c_like_comments(source: str) -> str:
    """Remove // and /* */ comments without changing quoted literals.

    Keeping line breaks makes diagnostics useful.  Go and JavaScript raw
    strings are both preserved; a comment marker inside a literal therefore
    cannot create a false import or package match.
    """

    result: list[str] = []
    i = 0
    state = "code"
    quote = ""
    while i < len(source):
        char = source[i]
        nxt = source[i + 1] if i + 1 < len(source) else ""
        if state == "code":
            if char == "/" and nxt == "/":
                result.extend((" ", " "))
                i += 2
                state = "line_comment"
                continue
            if char == "/" and nxt == "*":
                result.extend((" ", " "))
                i += 2
                state = "block_comment"
                continue
            if char == '"':
                result.append(char)
                quote = '"'
                state = "quoted"
                i += 1
                continue
            if char == "'":
                result.append(char)
                quote = "'"
                state = "quoted"
                i += 1
                continue
            if char == "`":
                result.append(char)
                quote = "`"
                state = "raw"
                i += 1
                continue
            result.append(char)
            i += 1
            continue
        if state == "line_comment":
            if char == "\n":
                result.append("\n")
                state = "code"
            else:
                result.append(" ")
            i += 1
            continue
        if state == "block_comment":
            if char == "*" and nxt == "/":
                result.extend((" ", " "))
                i += 2
                state = "code"
            else:
                result.append("\n" if char == "\n" else " ")
                i += 1
            continue
        if state == "raw":
            result.append(char)
            i += 1
            if char == quote:
                state = "code"
            continue
        # Interpreted string or character literal.
        result.append(char)
        i += 1
        if char == "\\" and i < len(source):
            result.append(source[i])
            i += 1
            continue
        if char == quote:
            state = "code"
    if state in {"block_comment", "quoted"}:
        raise InputError("unterminated comment or quoted literal")
    return "".join(result)


def _is_go_letter(char: str) -> bool:
    return char == "_" or char.isalpha()


def _is_go_digit(char: str) -> bool:
    return char.isdecimal()


def _is_go_identifier(token: str) -> bool:
    if not token or not (_is_go_letter(token[0])):
        return False
    return all(_is_go_letter(char) or _is_go_digit(char) for char in token[1:])


def _go_tokens(source: str) -> Iterator[tuple[str, int]]:
    """Yield enough Go tokens to find import declarations reliably."""

    source = _strip_c_like_comments(source)
    i = 0
    line = 1
    while i < len(source):
        char = source[i]
        if char in " \t\r":
            i += 1
            continue
        if char == "\n":
            line += 1
            i += 1
            continue
        if char == '`':
            start = i
            i += 1
            while i < len(source) and source[i] != '`':
                if source[i] == "\n":
                    line += 1
                i += 1
            if i >= len(source):
                raise InputError("unterminated Go raw string")
            yield source[start : i + 1], line
            i += 1
            continue
        if char == '"':
            start = i
            i += 1
            while i < len(source):
                if source[i] == "\\":
                    i += 2
                    continue
                if source[i] == '"':
                    i += 1
                    break
                if source[i] == "\n":
                    line += 1
                i += 1
            else:
                raise InputError("unterminated Go string")
            yield source[start:i], line
            continue
        if _is_go_letter(char):
            start = i
            i += 1
            while i < len(source) and (_is_go_letter(source[i]) or _is_go_digit(source[i])):
                i += 1
            yield source[start:i], line
            continue
        yield char, line
        i += 1


def _decode_go_string(token: str) -> str:
    if token.startswith("`") and token.endswith("`"):
        return token[1:-1]
    try:
        return json.loads(token)
    except json.JSONDecodeError as exc:
        raise InputError(f"invalid Go import string {token!r}: {exc}") from exc


def _go_imports(path: Path, repo_root: Path) -> list[tuple[str, int]]:
    try:
        source = path.read_text(encoding="utf-8")
    except OSError as exc:
        raise InputError(f"cannot read {path}: {exc}") from exc
    tokens = list(_go_tokens(source))
    imports: list[tuple[str, int]] = []

    def is_import_name(token: str) -> bool:
        return token == "." or token == "_" or _is_go_identifier(token)

    def consume_spec(cursor: int) -> int:
        """Consume one Go ImportSpec, including its optional local name."""

        if cursor >= len(tokens) or tokens[cursor][0] == ")":
            return cursor
        token, line = tokens[cursor]
        if token.startswith(('"', "`")):
            imports.append((_decode_go_string(token), line))
            return cursor + 1
        if is_import_name(token):
            cursor += 1
            if cursor < len(tokens) and tokens[cursor][0].startswith(('"', "`")):
                import_token, import_line = tokens[cursor]
                imports.append((_decode_go_string(import_token), import_line))
                return cursor + 1
        # Keep scanning after malformed syntax so one bad import does not hide
        # later declarations. Go compilation remains responsible for reporting
        # the malformed source itself.
        return cursor + 1

    for index, (token, _line) in enumerate(tokens):
        if token != "import" or index + 1 >= len(tokens):
            continue
        cursor = index + 1
        if tokens[cursor][0] == "(":
            cursor += 1
            while cursor < len(tokens) and tokens[cursor][0] != ")":
                cursor = consume_spec(cursor)
        else:
            consume_spec(cursor)
    return imports


def _proto_packages(path: Path) -> list[tuple[str, int]]:
    try:
        source = _strip_c_like_comments(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise InputError(f"cannot read {path}: {exc}") from exc
    return [(match.group(1), source[: match.start()].count("\n") + 1)
            for match in re.finditer(r"\bpackage\s+([A-Za-z_][\w.]*)\s*;", source)]


def _json_with_comments(path: Path) -> object:
    try:
        source = _strip_c_like_comments(path.read_text(encoding="utf-8"))
        return json.loads(source)
    except (OSError, json.JSONDecodeError, InputError) as exc:
        raise InputError(f"cannot parse JSON configuration {path}: {exc}") from exc


def _iter_source_files(root: Path, suffixes: set[str]) -> Iterator[Path]:
    for current, dirs, files in os.walk(root):
        dirs[:] = sorted(d for d in dirs if d not in EXCLUDED_DIRS)
        for name in sorted(files):
            path = Path(current) / name
            if path.suffix in suffixes and name not in EXCLUDED_FILE_NAMES:
                yield path


def _line_number(source: str, position: int) -> int:
    return source[:position].count("\n") + 1


def _frontend_references(path: Path) -> list[tuple[str, int]]:
    try:
        source = _strip_c_like_comments(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise InputError(f"cannot read {path}: {exc}") from exc
    # Static ES module imports/exports and dynamic import("...").  The
    # expression must be a quoted literal; computed runtime paths are outside
    # this check by design.
    pattern = re.compile(
        r"(?:\bfrom\s*|\bimport\s*\(\s*|\bimport\s*)(['\"])([^'\"]+)\1"
    )
    return [(match.group(2), _line_number(source, match.start()))
            for match in pattern.finditer(source)]


def _path_candidates(path: Path) -> list[Path]:
    if path.suffix:
        candidates = [path]
    else:
        candidates = [path.with_suffix(extension) for extension in (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs")]
        candidates.append(path)
    # Add conventional index files for extensionless imports.
    if not path.suffix:
        candidates.extend(path / f"index{extension}" for extension in (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"))
    return candidates


def _resolve_existing(path: Path) -> Path | None:
    for candidate in _path_candidates(path):
        try:
            if candidate.exists():
                return Path(os.path.realpath(candidate))
        except OSError as exc:
            raise InputError(f"cannot resolve {candidate}: {exc}") from exc
    return None


def _tsconfig_paths(consumer_root: Path) -> list[tuple[str, Path, Path]]:
    configs = sorted(consumer_root.rglob("tsconfig*.json"))
    mappings: list[tuple[str, Path, Path]] = []
    for config in configs:
        if any(part in EXCLUDED_DIRS for part in config.relative_to(consumer_root).parts):
            continue
        data = _json_with_comments(config)
        if not isinstance(data, dict):
            raise InputError(f"tsconfig must be an object: {config}")
        compiler = data.get("compilerOptions", {})
        if not isinstance(compiler, dict):
            raise InputError(f"compilerOptions must be an object: {config}")
        paths = compiler.get("paths", {})
        if paths is None:
            continue
        if not isinstance(paths, dict):
            raise InputError(f"compilerOptions.paths must be an object: {config}")
        base_url = compiler.get("baseUrl", ".")
        base = (config.parent / str(base_url)).resolve()
        for alias, targets in paths.items():
            if not isinstance(alias, str) or not isinstance(targets, list):
                raise InputError(f"invalid path alias in {config}: {alias!r}")
            for target in targets:
                if not isinstance(target, str):
                    raise InputError(f"invalid path target in {config}: {target!r}")
                mappings.append((alias, base, Path(target)))
    return mappings


def _resolve_frontend_reference(
    source_path: Path,
    reference: str,
    consumer_root: Path,
    aliases: Sequence[tuple[str, Path, Path]],
) -> Path | None:
    if reference.startswith("."):
        return _resolve_existing(source_path.parent / reference)
    if reference.startswith("/"):
        return _resolve_existing(Path(reference))
    for alias, base, target in aliases:
        regex = "^" + re.escape(alias).replace(r"\*", "(.*)") + "$"
        match = re.match(regex, reference)
        if match:
            suffix = match.group(1) if match.groups() else ""
            target_string = str(target).replace("*", suffix)
            return _resolve_existing(base / target_string)
    # A bare path can be resolved by a tsconfig baseUrl only when it is in the
    # alias table. Package imports remain package imports and are allowed here.
    return None


def _is_under(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
        return True
    except ValueError:
        return False


def _protected_frontend_target(path: Path, repo_root: Path) -> Path | None:
    for relative in SYSTEM_FRONTEND_DIRS:
        protected = Path(os.path.realpath(repo_root / relative))
        if _is_under(path, protected):
            return relative
    return None


def _go_mod_replacements(path: Path, repo_root: Path, consumer_root: Path) -> list[Violation]:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        raise InputError(f"cannot read {path}: {exc}") from exc
    violations: list[Violation] = []
    in_block = False
    relative = path.relative_to(repo_root).as_posix()
    for number, raw_line in enumerate(lines, 1):
        line = raw_line.split("//", 1)[0].strip()
        if not line:
            continue
        if line == ")" and in_block:
            in_block = False
            continue
        if line == "replace (":
            in_block = True
            continue
        expression = line
        if in_block:
            # A block line is already the replacement expression.
            pass
        elif not line.startswith("replace "):
            continue
        else:
            expression = line[len("replace "):].strip()
        if "=>" not in expression:
            raise InputError(f"cannot parse replace directive {relative}:{number}")
        left, right = (part.strip() for part in expression.split("=>", 1))
        old_module = left.split()[0] if left else ""
        target = right.split()[0] if right else ""
        if not target.startswith((".", "/")):
            continue
        target_path = Path(target)
        if not target_path.is_absolute():
            target_path = path.parent / target_path
        try:
            resolved = Path(os.path.realpath(target_path))
        except OSError as exc:
            raise InputError(f"cannot resolve replace target {target_path}: {exc}") from exc
        system_root = Path(os.path.realpath(repo_root))
        system_sources = Path(os.path.realpath(repo_root / "backend"))
        if resolved == system_root and old_module == SYSTEM_MODULE:
            continue
        if old_module == LEGACY_SYSTEM_MODULE:
            violations.append(Violation(
                relative,
                number,
                f"系统模块路径已改为 {SYSTEM_MODULE}，replace 须改为 {SYSTEM_MODULE} => 仓库根目录: {target}",
            ))
        elif _is_under(resolved, system_sources) or old_module == SYSTEM_MODULE:
            violations.append(Violation(
                relative,
                number,
                f"Go replace 必须只指向系统模块根目录（仓库根目录）: {target}",
            ))
    return violations


def check_references(repo_root: Path, consumer_root: Path) -> list[Violation]:
    repo_root = repo_root.resolve()
    consumer_root = consumer_root.resolve()
    if not repo_root.is_dir():
        raise InputError(f"repository root is not a directory: {repo_root}")
    if not consumer_root.is_dir():
        raise InputError(f"consumer directory is not a directory: {consumer_root}")
    try:
        consumer_root.relative_to(repo_root)
    except ValueError as exc:
        raise InputError("consumer directory must be inside repository root") from exc

    violations: list[Violation] = []
    for path in sorted(consumer_root.rglob("go.mod")):
        if any(part in EXCLUDED_DIRS for part in path.relative_to(consumer_root).parts):
            continue
        violations.extend(_go_mod_replacements(path, repo_root, consumer_root))
    for path in _iter_source_files(consumer_root, GO_SUFFIXES):
        rel = path.relative_to(repo_root).as_posix()
        for import_path, line in _go_imports(path, repo_root):
            if import_path.startswith(SYSTEM_PUBLIC_PREFIX + "internal/") or \
                    import_path == SYSTEM_PUBLIC_PREFIX.rstrip("/") + "/internal":
                violations.append(Violation(rel, line, f"业务代码不能导入系统 internal 包: {import_path}"))
    for path in _iter_source_files(consumer_root, PROTO_SUFFIXES):
        rel = path.relative_to(repo_root).as_posix()
        for package_name, line in _proto_packages(path):
            if package_name == "abysstract.admin.v1" or package_name.startswith("abysstract.admin.v1."):
                violations.append(Violation(rel, line, f"业务 Proto 必须使用业务包名，发现系统包名: {package_name}"))
    aliases = _tsconfig_paths(consumer_root)
    for path in _iter_source_files(consumer_root, FRONTEND_SUFFIXES):
        rel = path.relative_to(repo_root).as_posix()
        for reference, line in _frontend_references(path):
            # This package is the public boundary and is deliberately allowed;
            # its source location must never be inferred from the import name.
            if reference == "@abysstract/admin-core" or reference.startswith("@abysstract/admin-core/"):
                continue
            target = _resolve_frontend_reference(path, reference, consumer_root, aliases)
            if target is None:
                continue
            protected = _protected_frontend_target(target, repo_root)
            if protected is not None:
                violations.append(Violation(rel, line, f"前端代码不能直接引用系统源码 {protected}: {reference}"))
    return violations


def _is_excluded_file(path: Path) -> bool:
    if path.name in EXCLUDED_FILE_NAMES:
        return True
    if path.suffix in EXCLUDED_FILE_SUFFIXES:
        return True
    return False


def protected_file_paths(repo_root: Path) -> dict[str, Path]:
    """Return protected source files keyed by repository-relative path."""

    repo_root = repo_root.resolve()
    result: dict[str, Path] = {}
    for relative_root in PROTECTED_ROOTS:
        root = repo_root / relative_root
        if not root.exists():
            continue
        for current, dirs, files in os.walk(root):
            dirs[:] = sorted(d for d in dirs if d not in EXCLUDED_DIRS)
            for name in sorted(files):
                path = Path(current) / name
                if _is_excluded_file(path):
                    continue
                try:
                    resolved = Path(os.path.realpath(path))
                    if not _is_under(resolved, repo_root):
                        # A symlink outside the repository is not part of the
                        # source snapshot and cannot hide a protected change.
                        continue
                    relative = path.resolve().relative_to(repo_root).as_posix()
                except (OSError, ValueError) as exc:
                    raise InputError(f"cannot snapshot {path}: {exc}") from exc
                result[relative] = path
    return result


def snapshot_protected_files(repo_root: Path) -> dict[str, object]:
    files: dict[str, str] = {}
    for relative, path in protected_file_paths(repo_root).items():
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        files[relative] = digest
    return {"version": 1, "files": dict(sorted(files.items()))}


def write_snapshot(repo_root: Path, output: Path) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(snapshot_protected_files(repo_root), ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def compare_snapshot(repo_root: Path, snapshot_path: Path) -> dict[str, list[str]]:
    try:
        expected = json.loads(snapshot_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise InputError(f"cannot read snapshot {snapshot_path}: {exc}") from exc
    if not isinstance(expected, dict) or expected.get("version") != 1 or not isinstance(expected.get("files"), dict):
        raise InputError(f"invalid snapshot format: {snapshot_path}")
    before = {str(key): str(value) for key, value in expected["files"].items()}
    after = snapshot_protected_files(repo_root)["files"]
    assert isinstance(after, dict)
    added = sorted(set(after) - set(before))
    deleted = sorted(set(before) - set(after))
    modified = sorted(key for key in set(before) & set(after) if before[key] != after[key])
    return {"added": added, "deleted": deleted, "modified": modified}


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, required=True)
    parser.add_argument("--consumer-dir", type=Path, required=True)
    parser.add_argument("--snapshot", type=Path, help="write a protected-source snapshot")
    parser.add_argument("--compare", type=Path, help="compare protected sources with a snapshot")
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = _build_parser().parse_args(argv)
    try:
        repo_root = args.repo_root.resolve()
        violations = check_references(repo_root, args.consumer_dir)
        if args.snapshot is not None:
            write_snapshot(repo_root, args.snapshot)
            print(f"snapshot written: {args.snapshot}")
        if args.compare is not None:
            changes = compare_snapshot(repo_root, args.compare)
            if any(changes.values()):
                for kind in ("added", "deleted", "modified"):
                    for path in changes[kind]:
                        print(f"protected source {kind}: {path}")
                return 1
            print("protected source snapshot unchanged")
        if violations:
            for violation in violations:
                print(violation.format())
            return 1
        print("consumer boundary checks passed")
        return 0
    except InputError as exc:
        print(f"input error: {exc}", file=sys.stderr)
        return 2
    except OSError as exc:
        print(f"input error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
