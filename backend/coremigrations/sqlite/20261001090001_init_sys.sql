-- +goose Up
-- abysstract-admin 系统表（RBAC 核心）。与 RAG 业务表隔离，统一 sys_ 前缀。
-- 幂等：可重复执行。status: 1 启用 / 2 停用；del_flag: 0 正常 / 1 删除。

-- 部门
CREATE TABLE IF NOT EXISTS sys_dept (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  parent_id   BIGINT       NOT NULL DEFAULT 0,
  ancestors   VARCHAR(255) NOT NULL DEFAULT '',
  dept_name   VARCHAR(64)  NOT NULL,
  order_num   INT          NOT NULL DEFAULT 0,
  leader      VARCHAR(64)  NOT NULL DEFAULT '',
  phone       VARCHAR(32)  NOT NULL DEFAULT '',
  email       VARCHAR(128) NOT NULL DEFAULT '',
  status      SMALLINT     NOT NULL DEFAULT 1,
  del_flag    SMALLINT     NOT NULL DEFAULT 0,
  create_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 用户
CREATE TABLE IF NOT EXISTS sys_user (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  dept_id     BIGINT,
  username    VARCHAR(64) COLLATE BINARY NOT NULL,
  nickname    VARCHAR(64)  NOT NULL DEFAULT '',
  email       VARCHAR(128) NOT NULL DEFAULT '',
  phone       VARCHAR(32)  NOT NULL DEFAULT '',
  sex         SMALLINT     NOT NULL DEFAULT 0,
  avatar      VARCHAR(255) NOT NULL DEFAULT '',
  password    VARCHAR(100) NOT NULL DEFAULT '',
  status      SMALLINT     NOT NULL DEFAULT 1,
  del_flag    SMALLINT     NOT NULL DEFAULT 0,
  login_ip    VARCHAR(64)  NOT NULL DEFAULT '',
  login_date  DATETIME DEFAULT NULL,
  create_by BIGINT NOT NULL DEFAULT 0,
  token_version INT NOT NULL DEFAULT 0,
  pwd_reset_required SMALLINT NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  create_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 角色
CREATE TABLE IF NOT EXISTS sys_role (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  role_name   VARCHAR(64)  NOT NULL,
  role_key    VARCHAR(64) COLLATE BINARY NOT NULL,
  role_sort   INT          NOT NULL DEFAULT 0,
  data_scope  SMALLINT     NOT NULL DEFAULT 1,
  status      SMALLINT     NOT NULL DEFAULT 1,
  del_flag    SMALLINT     NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  create_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 菜单（含按钮权限）
CREATE TABLE IF NOT EXISTS sys_menu (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  menu_name   VARCHAR(64)  NOT NULL,
  parent_id   BIGINT       NOT NULL DEFAULT 0,
  order_num   INT          NOT NULL DEFAULT 0,
  path        VARCHAR(255) NOT NULL DEFAULT '',
  component   VARCHAR(255) NOT NULL DEFAULT '',
  query       VARCHAR(255) NOT NULL DEFAULT '',
  is_frame    SMALLINT     NOT NULL DEFAULT 0,
  is_cache    SMALLINT     NOT NULL DEFAULT 0,
  menu_type   CHAR(1)      NOT NULL DEFAULT '',
  visible     SMALLINT     NOT NULL DEFAULT 1,
  status      SMALLINT     NOT NULL DEFAULT 1,
  perms       VARCHAR(128) NOT NULL DEFAULT '',
  icon        VARCHAR(128) NOT NULL DEFAULT '',
  create_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 用户-角色 / 角色-菜单 关联
CREATE TABLE IF NOT EXISTS sys_user_role (
  user_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS sys_role_menu (
  role_id BIGINT NOT NULL,
  menu_id BIGINT NOT NULL,
  PRIMARY KEY (role_id, menu_id)
);

-- ---------- 幂等 seed ----------

INSERT INTO sys_dept (id, parent_id, ancestors, dept_name, order_num)
VALUES (1, 0, '0', 'Abysstract 总部', 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO sys_role (id, role_name, role_key, role_sort, data_scope, remark)
VALUES (1, '超级管理员', 'admin', 1, 1, '内置最高权限角色')
ON CONFLICT (id) DO NOTHING;

INSERT INTO sys_menu (id, menu_name, parent_id, order_num, path, component, menu_type, perms, icon) VALUES
  (1,   '系统管理', 0,   1, 'system',    '',                      'M', '',                   'setting'),
  (100, '用户管理', 1,   1, 'user',      'system/user/index',     'C', 'system:user:list',   'user'),
  (101, '角色管理', 1,   2, 'role',      'system/role/index',     'C', 'system:role:list',   'team'),
  (102, '菜单管理', 1,   3, 'menu',      'system/menu/index',     'C', 'system:menu:list',   'menu'),
  (103, '部门管理', 1,   4, 'dept',      'system/dept/index',     'C', 'system:dept:list',   'apartment'),
  (1001,'用户查询', 100, 1, '',          '',                      'F', 'system:user:query',  ''),
  (1002,'用户新增', 100, 2, '',          '',                      'F', 'system:user:add',    ''),
  (1003,'用户修改', 100, 3, '',          '',                      'F', 'system:user:edit',   ''),
  (1004,'用户删除', 100, 4, '',          '',                      'F', 'system:user:remove', ''),
  (1005,'重置密码', 100, 5, '',          '',                      'F', 'system:user:resetPwd','')
ON CONFLICT (id) DO NOTHING;

-- 超级管理员角色拥有以上全部菜单
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE TRUE
ON CONFLICT (role_id, menu_id) DO NOTHING;

-- 重置自增序列，避免与显式 id 冲突

-- Phase 2：字典 / 参数 / 通知公告。幂等，可重复执行。

-- 字典类型
CREATE TABLE IF NOT EXISTS sys_dict_type (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  dict_name   VARCHAR(100) NOT NULL DEFAULT '',
  dict_type   VARCHAR(100) COLLATE BINARY NOT NULL UNIQUE,
  status      SMALLINT     NOT NULL DEFAULT 1,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  create_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 字典数据
CREATE TABLE IF NOT EXISTS sys_dict_data (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  dict_sort   INT          NOT NULL DEFAULT 0,
  dict_label  VARCHAR(100) NOT NULL DEFAULT '',
  dict_value  VARCHAR(100) NOT NULL DEFAULT '',
  dict_type   VARCHAR(100) COLLATE BINARY NOT NULL DEFAULT '',
  css_class   VARCHAR(100) NOT NULL DEFAULT '',
  list_class  VARCHAR(100) NOT NULL DEFAULT '',
  is_default  SMALLINT     NOT NULL DEFAULT 0,
  status      SMALLINT     NOT NULL DEFAULT 1,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  create_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_dict_data_type ON sys_dict_data (dict_type);

-- 参数配置
CREATE TABLE IF NOT EXISTS sys_config (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  config_name  VARCHAR(100) NOT NULL DEFAULT '',
  config_key   VARCHAR(100) COLLATE BINARY NOT NULL UNIQUE,
  config_value VARCHAR(500) NOT NULL DEFAULT '',
  config_type  SMALLINT     NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  create_time  DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time  DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 通知公告
CREATE TABLE IF NOT EXISTS sys_notice (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  notice_title   VARCHAR(100) NOT NULL DEFAULT '',
  notice_type    SMALLINT     NOT NULL DEFAULT 1,
  notice_content TEXT         NOT NULL DEFAULT '',
  status         SMALLINT     NOT NULL DEFAULT 1,
  remark         VARCHAR(500) NOT NULL DEFAULT '',
  create_time    DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time    DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ---------- 菜单 seed（挂在“系统管理”id=1 下）----------
INSERT INTO sys_menu (id, menu_name, parent_id, order_num, path, component, menu_type, perms, icon) VALUES
  (104, '字典管理', 1, 5, 'dict',   'system/dict/index',   'C', 'system:dict:list',   'book'),
  (105, '参数设置', 1, 6, 'config', 'system/config/index', 'C', 'system:config:list', 'tool'),
  (106, '通知公告', 1, 7, 'notice', 'system/notice/index', 'C', 'system:notice:list', 'notification'),
  (1041, '字典新增', 104, 1, '', '', 'F', 'system:dict:add',    ''),
  (1042, '字典修改', 104, 2, '', '', 'F', 'system:dict:edit',   ''),
  (1043, '字典删除', 104, 3, '', '', 'F', 'system:dict:remove', ''),
  (1051, '参数新增', 105, 1, '', '', 'F', 'system:config:add',    ''),
  (1052, '参数修改', 105, 2, '', '', 'F', 'system:config:edit',   ''),
  (1053, '参数删除', 105, 3, '', '', 'F', 'system:config:remove', ''),
  (1061, '公告新增', 106, 1, '', '', 'F', 'system:notice:add',    ''),
  (1062, '公告修改', 106, 2, '', '', 'F', 'system:notice:edit',   ''),
  (1063, '公告删除', 106, 3, '', '', 'F', 'system:notice:remove', '')
ON CONFLICT (id) DO NOTHING;

-- 超级管理员补齐新菜单授权
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE TRUE
ON CONFLICT (role_id, menu_id) DO NOTHING;

-- ---------- 示例数据 ----------
INSERT INTO sys_dict_type (id, dict_name, dict_type, remark) VALUES
  (1, '用户性别', 'sys_user_sex', '用户性别列表'),
  (2, '系统开关', 'sys_normal_disable', '系统启用停用列表')
ON CONFLICT (id) DO NOTHING;

INSERT INTO sys_dict_data (id, dict_sort, dict_label, dict_value, dict_type, is_default) VALUES
  (1, 1, '男',   '0', 'sys_user_sex', 1),
  (2, 2, '女',   '1', 'sys_user_sex', 0),
  (3, 3, '未知', '2', 'sys_user_sex', 0),
  (4, 1, '正常', '1', 'sys_normal_disable', 1),
  (5, 2, '停用', '2', 'sys_normal_disable', 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO sys_config (id, config_name, config_key, config_value, config_type, remark) VALUES
  (1, '系统名称', 'sys.index.title', 'Abysstract 后台管理', 1, '首页标题')
ON CONFLICT (id) DO NOTHING;

INSERT INTO sys_notice (id, notice_title, notice_type, notice_content) VALUES
  (1, '欢迎使用 Abysstract 后台', 2, '<p>这是一条示例公告。</p>')
ON CONFLICT (id) DO NOTHING;

-- 重置序列

-- Phase 3：操作日志 / 登录日志。幂等。status: 1 成功 / 2 失败。

CREATE TABLE IF NOT EXISTS sys_oper_log (
  oper_param VARCHAR(2000) NOT NULL DEFAULT '',
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  title      VARCHAR(100)  NOT NULL DEFAULT '',
  method     VARCHAR(200)  NOT NULL DEFAULT '',
  oper_name  VARCHAR(64)   NOT NULL DEFAULT '',
  oper_id    BIGINT        NOT NULL DEFAULT 0,
  oper_ip    VARCHAR(64)   NOT NULL DEFAULT '',
  status     SMALLINT      NOT NULL DEFAULT 1,
  error_msg  VARCHAR(2000) NOT NULL DEFAULT '',
  cost_time  BIGINT        NOT NULL DEFAULT 0,
  oper_time  DATETIME   NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_oper_log_time ON sys_oper_log (oper_time DESC);

CREATE TABLE IF NOT EXISTS sys_login_log (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  username   VARCHAR(64)  NOT NULL DEFAULT '',
  status     SMALLINT     NOT NULL DEFAULT 1,
  ip         VARCHAR(64)  NOT NULL DEFAULT '',
  msg        VARCHAR(255) NOT NULL DEFAULT '',
  login_time DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_login_log_time ON sys_login_log (login_time DESC);

-- 菜单（系统管理 id=1 下）
INSERT INTO sys_menu (id, menu_name, parent_id, order_num, path, component, menu_type, perms, icon) VALUES
  (107, '操作日志', 1, 8, 'operlog',    'monitor/operlog/index',    'C', 'system:operlog:list',     'profile'),
  (108, '登录日志', 1, 9, 'logininfor', 'monitor/logininfor/index', 'C', 'system:logininfor:list',  'login'),
  (1071, '清空操作日志', 107, 1, '', '', 'F', 'system:operlog:remove',    ''),
  (1081, '清空登录日志', 108, 1, '', '', 'F', 'system:logininfor:remove', '')
ON CONFLICT (id) DO NOTHING;

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE TRUE
ON CONFLICT (role_id, menu_id) DO NOTHING;


-- data_scope 闭环：自定义部门集合 + 受控表创建者标识。幂等。
--
-- sys_role_dept：承载「自定义」(data_scope=2) 角色显式配置的可见部门集合。
-- sys_user.create_by：受控表「仅本人」(data_scope=5) 所需的创建者标识，
--   默认 0 表示历史数据/系统创建，迁移后由 CreateUser 写入当前用户 ID。

CREATE TABLE IF NOT EXISTS sys_role_dept (
  role_id BIGINT NOT NULL,
  dept_id BIGINT NOT NULL,
  PRIMARY KEY (role_id, dept_id)
);


-- 账号安全：token 版本号会话失效、首次改密门禁、登录锁定、刷新令牌。
-- 幂等。所有列在已有表/数据上 ADD COLUMN IF NOT EXISTS，默认值兼容历史行。
--
-- token_version：JWT claims 携带 ver，拦截器比对 user.token_version；改密/登出使其 +1。
-- pwd_reset_required：1 表示首次登录（或被重置密码后）需改密，仅放行改密与登出。
-- sys_login_attempt：单行 per username 维护失败计数与锁定截止；到期后由代码重置。
-- sys_refresh_token：仅在配置启用刷新令牌特性时使用；存 SHA-256(token) 避免明文泄露。


CREATE TABLE IF NOT EXISTS sys_login_attempt (
  username     VARCHAR(64) COLLATE BINARY PRIMARY KEY,
  fail_count   INT NOT NULL DEFAULT 0,
  locked_until DATETIME
);

CREATE TABLE IF NOT EXISTS sys_refresh_token (
  token_version INT NOT NULL DEFAULT 0,
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    BIGINT NOT NULL,
  token_hash CHAR(64) NOT NULL UNIQUE,
  expires_at DATETIME NOT NULL,
  revoked    SMALLINT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_refresh_token_user ON sys_refresh_token (user_id);

CREATE UNIQUE INDEX uq_sys_user_username ON sys_user (username) WHERE del_flag = 0;
CREATE UNIQUE INDEX uq_sys_role_role_key ON sys_role (role_key) WHERE del_flag = 0;
-- +goose Down
-- Forward-only installation baseline.
