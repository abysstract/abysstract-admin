#!/usr/bin/env bash
# 本地非容器 PostgreSQL 初始化脚本（幂等），连接到 admin 库后执行 init_pg.sql。
set -euo pipefail

PG_HOST="${PG_HOST:-localhost}"
PG_PORT="${PG_PORT:-5433}"
PG_USER="${PG_USER:-admin}"
PG_PASSWORD="${PG_PASSWORD:-admin}"
PG_DB="${PG_DB:-abysstract_admin}"

export PGPASSWORD="$PG_PASSWORD"

echo "==> 初始化独立 admin PostgreSQL（${PG_HOST}:${PG_PORT}/${PG_DB}）..."

# 1. 确保 admin 库存在（幂等）
psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d postgres -tc \
  "SELECT 1 FROM pg_database WHERE datname = '$PG_DB'" | grep -q 1 || \
  psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d postgres -c \
  "CREATE DATABASE \"$PG_DB\""

# 2. 执行 init_pg.sql（幂等扩展创建）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -f "$SCRIPT_DIR/init_pg.sql"

echo "==> init_pg 完成"