-- +goose Up
-- 请求日志（access log）：accesslog 的 db sink 异步批量写入，按 ADMIN_ACCESS_LOG_RETENTION_DAYS 定期清理。
-- 与审计用的 sys_oper_log 不同，队列满时会丢弃条目。
-- kind: rpc / http；code: Connect 错误码（HTTP 请求为空）；status: 1 成功 / 2 失败；
-- duration_us: 微秒；attrs: 业务字段的 JSON 文本。
CREATE TABLE sys_access_log (
  id             BIGSERIAL PRIMARY KEY,
  log_time       TIMESTAMPTZ   NOT NULL,
  request_id     VARCHAR(64)   NOT NULL DEFAULT '',
  kind           VARCHAR(8)    NOT NULL DEFAULT '',
  service        VARCHAR(200)  NOT NULL DEFAULT '',
  procedure_name VARCHAR(255)  NOT NULL DEFAULT '',
  http_method    VARCHAR(16)   NOT NULL DEFAULT '',
  path           VARCHAR(512)  NOT NULL DEFAULT '',
  principal_type VARCHAR(32)   NOT NULL DEFAULT '',
  principal_id   VARCHAR(64)   NOT NULL DEFAULT '',
  principal_name VARCHAR(128)  NOT NULL DEFAULT '',
  client_ip      VARCHAR(64)   NOT NULL DEFAULT '',
  user_agent     VARCHAR(255)  NOT NULL DEFAULT '',
  code           VARCHAR(32)   NOT NULL DEFAULT '',
  http_status    INT           NOT NULL DEFAULT 0,
  status         SMALLINT      NOT NULL DEFAULT 1,
  error_msg      VARCHAR(1000) NOT NULL DEFAULT '',
  duration_us    BIGINT        NOT NULL DEFAULT 0,
  attrs          TEXT          NOT NULL
);
CREATE INDEX idx_access_log_time ON sys_access_log (log_time);
CREATE INDEX idx_access_log_principal_id ON sys_access_log (principal_id, log_time);
CREATE INDEX idx_access_log_principal_name ON sys_access_log (principal_name, log_time);
CREATE INDEX idx_access_log_procedure ON sys_access_log (procedure_name, log_time);

-- 菜单「请求日志」（系统管理 id=1 下）与超级管理员授权。按 perms 幂等，不占用固定 id。
INSERT INTO sys_menu (menu_name, parent_id, order_num, path, component, menu_type, perms, icon)
SELECT '请求日志', 1, 10, 'accesslog', 'monitor/accesslog/index', 'C', 'system:accesslog:list', 'api'
WHERE NOT EXISTS (SELECT 1 FROM sys_menu WHERE perms = 'system:accesslog:list');

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE perms = 'system:accesslog:list'
ON CONFLICT (role_id, menu_id) DO NOTHING;

-- +goose Down
DELETE FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE perms = 'system:accesslog:list');
DELETE FROM sys_menu WHERE perms = 'system:accesslog:list';
DROP TABLE sys_access_log;
