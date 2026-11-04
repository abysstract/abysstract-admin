#!/usr/bin/env bash
# lint-staged 的 Go 格式化任务：跳过生成代码（*.pb.go / *.gen.go 及受管生成目录）。
# 生成物由生成器产出，并由 CI generated-code-integrity 校验，禁止在提交钩子中改写。
set -euo pipefail

files=()
for f in "$@"; do
  case "$f" in
    *.pb.go|*.gen.go|*/gen/*|*/internal/resource/gen/*|*/internal/resource/schema/testpb/*|*/internal/resource/schema/testenum/*)
      ;;
    *)
      files+=("$f")
      ;;
  esac
done

if [ "${#files[@]}" -gt 0 ]; then
  gofmt -w "${files[@]}"
  goimports -w -local github.com/abysstract/abysstract-admin/backend "${files[@]}"
fi
