#!/usr/bin/env bash
# 将本地 admin 数据库重置到初始可迁移状态（移除所有迁移对象，使迁移可从零重跑）。
set -euo pipefail

PG_HOST="${PG_HOST:-localhost}"
PG_PORT="${PG_PORT:-5433}"
PG_USER="${PG_USER:-admin}"
PG_PASSWORD="${PG_PASSWORD:-admin}"
PG_DB="${PG_DB:-abysstract_admin}"

export PGPASSWORD="$PG_PASSWORD"

echo "==> 重置 admin 数据库（${PG_HOST}:${PG_PORT}/${PG_DB}）..."

psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" <<'SQL'
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
SQL

# 重新执行 init_pg.sql 恢复必要扩展
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -f "$SCRIPT_DIR/init_pg.sql"

echo "==> reset 完成，数据库已回到初始状态"