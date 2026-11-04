#!/bin/sh
set -eu
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
phase=all
module=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --phase) phase=$2; shift 2 ;;
    --module) module=$2; shift 2 ;;
    *) printf 'Unknown argument: %s\n' "$1" >&2; exit 1 ;;
  esac
done
case "$phase" in all|model|offline|proto|resource) ;; *) printf 'Unknown phase: %s\n' "$phase" >&2; exit 1 ;; esac
(cd "$repo_root/backend" && GOWORK=off go run ./cmd/admin module sync)
set --
if [ -n "$module" ]; then set -- --module "$module"; fi
if [ "$phase" = all ] || [ "$phase" = model ]; then
  printf '%s\n' '生成数据库模型和查询代码'
  (cd "$repo_root/backend" && GOWORK=off go run ./cmd/modelgen "$@")
fi
if [ "$phase" = all ] || [ "$phase" = offline ] || [ "$phase" = proto ]; then
  printf '%s\n' '生成全部协议及模块间依赖'
  (cd "$repo_root" && node frontend/tools/generate-proto.mjs)
fi
if [ "$phase" = all ] || [ "$phase" = offline ] || [ "$phase" = resource ]; then
  printf '%s\n' '生成 Resource 代码'
  (cd "$repo_root/backend" && GOWORK=off go run ./cmd/adminresource-gen "$@")
fi
printf '%s\n' '代码生成完成'
