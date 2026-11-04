# 权限模型（RBAC）

## 实体关系

```
用户 sys_user ──< sys_user_role >── 角色 sys_role ──< sys_role_menu >── 菜单 sys_menu
```

- 一个用户可绑定多个角色，一个角色可分配多个菜单/按钮。
- 权限标识（`perms`）定义在 `sys_menu` 上，形如 `system:user:add`。

## 权限标识约定

格式：`<模块>:<对象>:<操作>`，例如：

- `system:user:list` — 用户列表查看
- `system:user:add` / `edit` / `remove` / `resetPwd`
- `system:role:list` / `add` / `edit` / `remove`
- `*:*:*` — 超级管理员通配，放行一切

## 数据范围

接口权限决定可以调用哪些方法。数据范围决定这些方法能访问哪些业务记录。角色的 `data_scope` 取值：

| 值 | 含义 |
| --- | --- |
| 1 | 全部 |
| 2 | 自定义部门，部门列表在 `sys_role_dept` |
| 3 | 本部门 |
| 4 | 本部门及以下 |
| 5 | 仅本人 |

多个角色取并集；任一角色为全部时不再限制范围。没有有效角色时只看本人记录。

标准 JSON CRUD 默认不启用数据范围。要启用时，在手写 Resource 定义中填写部门和创建者字段，并在 `configure<Resource>` 中注入 `rt.DataScope`。接入方式见 [Resource 手册](ai-cookbook.md#6-权限与数据范围)。

## 后端鉴权（Connect 拦截器）

1. `NewAuthInterceptor`：校验 Bearer token，注入 Claims 到 context；`AuthService/Login` 放行。
2. `NewPermissionInterceptor`：按 RPC 过程映射到所需权限标识校验；含 `admin` 角色返回 `*:*:*` 通配放行。
3. `NewOperLogInterceptor`：记录写操作审计日志（注册在最内层）。

## 前端鉴权

- 登录后由 `GetUserInfo` 取回 `permissions` 与可见 `menus`，存入 `AccessProvider`。
- 用 `useAccess().has(perm)` 控制操作按钮的显隐，与后端使用同一套标识。
- 侧边菜单与路由由后端 `menus` 动态生成（详见[模块接入](module-guide.md#后端服务与前端页面)）。
- 权限加载完成前显示 loading；加载失败统一清理 token 并跳转登录。

## 内置账号保护

为避免脚手架环境被操作到不可恢复，内置管理员（`sys_user.id = 1`）与当前登录用户受保护：

- 内置管理员不可删除、不可停用。
- 不可删除 / 停用当前登录用户。
- 内置管理员密码仅限其本人重置。

## 软删除与唯一约束

用户名和角色标识在有效记录中保持唯一。记录软删除后，可以重新创建同名用户或角色。
