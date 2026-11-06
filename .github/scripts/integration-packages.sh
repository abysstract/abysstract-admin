#!/usr/bin/env bash
# 列出含 integration 构建标签测试文件的 Go 包（相对 backend/ 的 ./path）。
# 数据库任务只测试这些包；其余包的单元测试由 "Backend build & test" 负责。
set -euo pipefail
cd "$(dirname "$0")/../../backend"
pkgs="$(grep -rlE --include='*_test.go' '^//go:build .*\bintegration\b' . | xargs -n1 dirname | sort -u)"
if [ -z "$pkgs" ]; then
  echo "未找到带 integration 构建标签的测试文件" >&2
  exit 1
fi
printf '%s\n' "$pkgs"
