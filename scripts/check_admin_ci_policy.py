#!/usr/bin/env python3
"""Reject unauthorized publish/tag/push actions in public CI workflows.

Exit codes:
  0: workflow policy passed
  1: forbidden action or missing permission isolation
  2: input error
"""

from __future__ import annotations

import argparse
from pathlib import Path
import re
import sys
from typing import Sequence


FORBIDDEN_PATTERNS = (
    (re.compile(r"\bnpm\s+publish\b"), "npm publish"),
    (re.compile(r"\bnpm\s+publish\b"), "npm publish"),
    (re.compile(r"\bgit\s+push\b"), "git push"),
    (re.compile(r"\bgit\s+tag\b"), "git tag"),
    (re.compile(r"\bbuf\s+push\b"), "buf push"),
    (re.compile(r"\bgh\s+release\s+create\b"), "gh release create"),
)

SECRET_PATTERNS = (
    re.compile(r"NPM_TOKEN"),
    re.compile(r"NODE_AUTH_TOKEN"),
    re.compile(r"BUF_TOKEN"),
)


class InputError(Exception):
    pass


class CheckFailure(Exception):
    pass


def check_workflow(text: str, path: Path) -> list[str]:
    errors: list[str] = []
    for pattern, label in FORBIDDEN_PATTERNS:
        if pattern.search(text):
            errors.append(f"{path}: contains unauthorized {label}")
    if "permissions:" not in text:
        errors.append(f"{path}: missing top-level read-only permissions")
    if re.search(r"pull_request_target", text):
        errors.append(f"{path}: pull_request_target would expose secrets to fork PRs")
    for pattern in SECRET_PATTERNS:
        if pattern.search(text):
            errors.append(f"{path}: private distribution credential {pattern.pattern} must not be available to CI")
    return errors


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parents[1])
    args = parser.parse_args(argv)
    workflows = args.repo_root / ".github" / "workflows"
    if not workflows.is_dir():
        print(f"input error: missing {workflows}", file=sys.stderr)
        return 2
    errors: list[str] = []
    for path in sorted(workflows.glob("*.yml")) + sorted(workflows.glob("*.yaml")):
        content = path.read_text(encoding="utf-8")
        rel = path.relative_to(args.repo_root)
        errors.extend(check_workflow(content, rel))
    if errors:
        print("CI policy check failed:", file=sys.stderr)
        for error in errors:
            print(error, file=sys.stderr)
        return 1
    print("CI policy check passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
