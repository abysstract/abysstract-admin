#!/usr/bin/env python3
"""Probe the public Go packages from an isolated external module.

The probe never changes the repository's go.mod.  It creates a temporary
module, requires the system module explicitly, replaces it with the absolute
repository root supplied by --repo-root, and compiles one tiny package import
from backend/ at a time.

Exit codes:
  0: every package met the selected stage expectation
  1: a package had the wrong availability or a real compile/dependency error
  2: probe configuration or input error
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
from typing import Sequence


SYSTEM_MODULE = "github.com/abysstract/abysstract-admin"
PACKAGE_PREFIX = f"{SYSTEM_MODULE}/backend"
TARGET_PACKAGES = ("resource", "module", "permission", "resourcegen", "coremigrations", "database", "system", "adminconfig", "identity", "adminapp", "moduleseed", "storage", "storage/localstorage", "storage/s3storage")


class ProbeInputError(Exception):
    pass


@dataclass(frozen=True)
class GoRunResult:
    exit_code: int
    stdout: str
    stderr: str
    failure: str | None = None

    @property
    def output(self) -> str:
        return (self.stdout + self.stderr).strip()


@dataclass(frozen=True)
class ProbeResult:
    package: str
    stage: str
    category: str
    exit_code: int
    output: str

    @property
    def passed(self) -> bool:
        if self.stage == "k1":
            return self.category == "missing_package"
        return self.category == "ok"

    def line(self) -> str:
        return (
            f"package={self.package} stage={self.stage} result={'pass' if self.passed else 'fail'} "
            f"category={self.category} exit={self.exit_code}"
        )


def _module_name(go_mod: Path) -> str:
    try:
        content = go_mod.read_text(encoding="utf-8")
    except OSError as exc:
        raise ProbeInputError(f"cannot read {go_mod}: {exc}") from exc
    match = re.search(r"(?m)^\s*module\s+([^\s]+)\s*$", content)
    if match is None:
        raise ProbeInputError(f"module declaration missing: {go_mod}")
    return match.group(1)


def _go_version(go_mod: Path) -> str:
    content = go_mod.read_text(encoding="utf-8")
    match = re.search(r"(?m)^\s*go\s+([0-9]+(?:\.[0-9]+){1,2})\s*$", content)
    return match.group(1) if match else "1.25"


def _version_tuple(version: str) -> tuple[int, int, int]:
    values = [int(part) for part in version.split(".")]
    values.extend([0] * (3 - len(values)))
    return tuple(values[:3])


def _run_go(go_bin: str, args: list[str], cwd: Path, timeout: float = 60.0) -> GoRunResult:
    env = os.environ.copy()
    env["GOWORK"] = "off"
    try:
        completed = subprocess.run(
            [go_bin, *args],
            cwd=cwd,
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired as exc:
        stdout = exc.stdout.decode() if isinstance(exc.stdout, bytes) else (exc.stdout or "")
        stderr = exc.stderr.decode() if isinstance(exc.stderr, bytes) else (exc.stderr or "")
        return GoRunResult(124, stdout, stderr or f"go command timed out after {timeout}s", "timeout")
    except OSError as exc:
        return GoRunResult(127, "", str(exc), "execution")
    return GoRunResult(completed.returncode, completed.stdout, completed.stderr)


def _toolchain_error(go_bin: str, required: str, module_root: Path) -> tuple[str, str] | None:
    result = _run_go(go_bin, ["version"], module_root, timeout=10)
    if result.failure is not None:
        return result.failure, result.output
    if result.exit_code != 0:
        return "toolchain", result.output
    match = re.search(r"go([0-9]+\.[0-9]+(?:\.[0-9]+)?)", result.stdout)
    if match is None:
        return "toolchain", f"unrecognised go version output: {result.output}"
    if _version_tuple(match.group(1)) < _version_tuple(required):
        return "toolchain", f"go tool {match.group(1)} is older than module requirement {required}"
    return None


PROBE_MODULE = "example.com/admin-public-package-probe"


def _target_missing_diagnostic(output: str, package_path: str) -> bool:
    """Parse the complete Go loader result and accept only known absence forms.

    The accepted grammar describes Go's complete missing-package result. Any
    unrecognised line, including an extra compiler or environment diagnostic,
    makes the result a failed probe. Paths, versions, and URLs inside the
    structured module-found line remain data and are never interpreted as
    error keywords.
    """

    target = package_path.lower()
    target_re = re.escape(target)
    module_re = re.escape(SYSTEM_MODULE.lower())
    lines = [line.strip() for line in output.splitlines() if line.strip()]
    if not lines:
        return False

    target_missing = False
    allow_go_get = False
    for line in lines:
        lowered = line.lower()
        if re.fullmatch(r"FAIL(?:\s+.*)?", line):
            continue
        if line == f"# {PROBE_MODULE}":
            continue
        if line == f"go: finding module for package {package_path}":
            continue
        if re.fullmatch(
            rf"probe\.go:\d+:\d+: module {module_re}(?:@\S+)? found \(.*\), but does not contain package {target_re}",
            lowered,
        ):
            target_missing = True
            continue
        if re.fullmatch(
            rf"(?:probe\.go:\d+:\d+: )?no required module provides package {target_re}; to add it:",
            lowered,
        ):
            target_missing = True
            allow_go_get = True
            continue
        if re.fullmatch(rf"(?:go: )?package {target_re}: no go files", lowered):
            target_missing = True
            continue
        if allow_go_get and lowered == f"go get {target}":
            allow_go_get = False
            continue
        # Every line not covered by the known Go result grammar is an
        # additional failure or an unknown toolchain format.
        return False
    return target_missing and not allow_go_get


def _classify(exit_code: int, output: str, package_path: str, failure: str | None = None) -> str:
    if failure is not None:
        return failure
    if exit_code == 0:
        return "ok"
    if _target_missing_diagnostic(output, package_path):
        return "missing_package"
    lowered = output.lower()
    if any(marker in lowered for marker in ("syntax error", "expected 'package'", "malformed", "invalid go", "undefined:", "cannot use ")):
        return "syntax"
    if any(marker in lowered for marker in ("no required module provides", "cannot find module providing package", "missing go.sum", "sum.golang.org", "lookup ", "dial tcp", "no such host", "connection refused")):
        return "dependency"
    return "compile"


def probe_package(
    module_root: Path,
    package_name: str,
    stage: str,
    go_bin: str = "go",
    temp_root: Path | None = None,
    timeout: float = 60.0,
) -> ProbeResult:
    module_root = module_root.resolve()
    go_mod = module_root / "go.mod"
    if not go_mod.is_file():
        raise ProbeInputError(f"system module not found: {go_mod}")
    if _module_name(go_mod) != SYSTEM_MODULE:
        raise ProbeInputError(f"system module path must be {SYSTEM_MODULE}: {go_mod}")
    if package_name not in TARGET_PACKAGES:
        raise ProbeInputError(f"unsupported package: {package_name}")
    if stage not in {"k1", "k2"}:
        raise ProbeInputError(f"unsupported stage: {stage}")
    required_go = _go_version(go_mod)
    toolchain_problem = _toolchain_error(go_bin, required_go, module_root)
    if toolchain_problem:
        category, message = toolchain_problem
        return ProbeResult(package_name, stage, category, 127, message)

    owns_temp = temp_root is None
    temp_dir = Path(tempfile.mkdtemp(prefix="admin-public-package-probe-", dir=temp_root))
    try:
        probe_module = PROBE_MODULE
        (temp_dir / "go.mod").write_text(
            f"module {probe_module}\n\ngo {required_go}\n\n"
            f"require {SYSTEM_MODULE} v0.0.0\n\n"
            f"replace {SYSTEM_MODULE} => {module_root}\n",
            encoding="utf-8",
        )
        import_path = f"{PACKAGE_PREFIX}/{package_name}"
        (temp_dir / "probe.go").write_text(
            "package probe\n\n"
            "import _ " + repr(import_path).replace("'", '"') + "\n",
            encoding="utf-8",
        )
        run = _run_go(go_bin, ["test", "-mod=mod", "-count=1", "./..."], temp_dir, timeout=timeout)
        output = run.output
        category = _classify(run.exit_code, output, import_path, run.failure)
        return ProbeResult(package_name, stage, category, run.exit_code, output)
    finally:
        if owns_temp:
            shutil.rmtree(temp_dir, ignore_errors=True)


def run_probe(repo_root: Path, stage: str, go_bin: str = "go", timeout: float = 60.0) -> list[ProbeResult]:
    repo_root = repo_root.resolve()
    if not (repo_root / "backend").is_dir():
        raise ProbeInputError(f"backend directory not found: {repo_root / 'backend'}")
    return [probe_package(repo_root, package, stage, go_bin=go_bin, timeout=timeout) for package in TARGET_PACKAGES]


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, required=True)
    parser.add_argument("--stage", choices=("k1", "k2"), required=True)
    parser.add_argument("--go-bin", default="go", help="Go executable used for the isolated probe")
    parser.add_argument("--timeout", type=float, default=60.0, help="Timeout in seconds for each package compilation")
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = _build_parser().parse_args(argv)
    try:
        results = run_probe(args.repo_root, args.stage, args.go_bin, timeout=args.timeout)
    except ProbeInputError as exc:
        print(f"input error: {exc}", file=sys.stderr)
        return 2
    all_passed = True
    for result in results:
        print(result.line())
        if result.output:
            print(result.output)
        all_passed = all_passed and result.passed
    return 0 if all_passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
