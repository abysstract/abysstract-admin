# 项目结构与开发入口

第一次体验请按 [README 快速开始](../README.md#快速开始体验自带后台)运行后台；接入自己的业务请从[模块教程](new-module-tutorial.md)开始。

## 项目组成

本仓库包含公共包和业务应用：

- **公共包**：提供应用启动、权限、迁移、Resource、前端组件和系统页面。
- **业务应用**：从系统管理功能起步，在当前应用中加入业务模块；应用负责路由、中间件和服务启停。

商品管理预装在业务应用中，提供表、接口、回调和页面作为开发参考。

业务模块随应用一起编译和部署。业务项目维护自己的表、接口、页面和迁移，通过公共包使用系统功能。

## 常用概念

| 名称 | 负责什么 |
|---|---|
| Proto | 定义接口消息、字段和 RPC 方法 |
| ConnectRPC | 让前端通过 HTTP 调用 Proto 定义的接口 |
| Migration（迁移） | 按版本修改表结构或初始化数据 |
| Model（模型） | 将 Go 字段映射到数据库列 |
| Resource | 声明字段行为和标准 CRUD 操作 |
| Repository | 处理数据查询与写入 |
| Hook（回调） | 在标准操作的指定阶段执行业务逻辑 |
| Module（模块） | 将服务、权限或页面组织为可接入的功能 |

## 主要目录

| 位置 | 内容 |
|---|---|
| `backend/` | Go 公共包与业务应用 |
| `backend/internal/app/` | 后端模块组合、HTTP 服务与生命周期 |
| `proto/` | 系统与业务接口定义 |
| `frontend/packages/admin-core/` | 公共前端组件与系统页面 |
| `frontend/packages/admin-proto/` | 系统协议类型 |
| `frontend/apps/reference/` | 自带前端应用入口 |
| `backend/internal/modules/product/` | 预装商品业务模块 |

商品示例中的文件分工见[教程中的接入说明](admin-core-getting-started.md#1-认识商品模块)。业务项目通过公共包接入，生成文件通过源定义和生成命令更新。

## 页面如何调用接口

以商品列表为例：

1. 页面将查询条件交给 `ResourceTable`。
2. 生成的 operations 将条件转换成 Proto 请求，通过 Connect 客户端发送。
3. 后端检查身份、操作权限和数据范围，再查询数据库。
4. 查询结果返回页面并显示在列表中。

排查问题时，先在浏览器网络面板确认请求地址、参数和响应。接口和字段问题参考 [Resource 手册](ai-cookbook.md)，菜单与页面映射问题参考[模块指南](module-guide.md#seed-声明)。

## 登录、权限、数据范围与菜单

- **登录**确认用户身份，请求携带登录凭证。
- **操作权限**决定可以调用哪些接口，页面据此显示操作，后端执行授权。
- **数据范围**决定可以访问哪些记录，例如本部门数据。
- **菜单**提供页面入口，`component` 对应前端模块配置的页面标识。

菜单可见而接口返回 `PermissionDenied` 时，分别检查操作权限和记录的数据范围。操作权限见[权限说明](permission.md)。数据范围取值见[数据范围](permission.md#数据范围)，Resource 接入见 [Resource 手册](ai-cookbook.md#6-权限与数据范围)。

## 按任务查阅

| 任务 | 文档 |
|---|---|
| 运行后台 | [快速开始](../README.md#快速开始体验自带后台) |
| 配置数据库与服务 | [配置说明](configuration.md) |
| 接入业务模块 | [模块教程](new-module-tutorial.md)、[模块指南](module-guide.md) |
| 新增字段或调整 CRUD | [Resource 手册](ai-cookbook.md) |
| 配置公共包和前端页面 | [API 用法](admin-core-usage.md) |
| 选择 Hook、Repository 或专用接口 | [扩展方式](resource-migration.md) |
| 执行迁移和生成 | [开发说明](development.md) |
| 发布应用 | [部署说明](deployment.md) |
