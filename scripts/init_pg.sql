-- abysstract-admin 本地 PostgreSQL 初始化脚本（幂等）
--
-- 作用范围：仅初始化 admin 自有的独立数据库与必要扩展，
-- 绝不连接或写入 RAG 服务端的 gridrag 库。
--
-- 执行方式：
--   1) 容器场景：由 postgres 官方镜像 entrypoint 在首次创建数据卷时自动执行，
--      此时已连接到 docker-compose 指定的 POSTGRES_DB（abysstract_admin），
--      数据库本身由镜像创建，本脚本只需保证扩展等对象幂等就绪。
--   2) 本地非容器场景：由 scripts/init_pg.sh 连接到 admin 库后执行本脚本。
--
-- 幂等性：所有对象创建均使用 IF NOT EXISTS，可重复执行而不报错、不重复创建。

-- 必要扩展（按需启用，均为幂等）：
--   pgcrypto：提供 gen_random_uuid()、crypt() 等，供未来主键/密码哈希等场景使用。
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 说明：sys_* 业务表由后端迁移命令（go run ./cmd/migrate）负责创建与维护，
-- 本脚本不在此创建业务表，避免与 DB-first 迁移流程职责重叠。
