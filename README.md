# abysstract-admin

Abysstract 后台管理系统。前端使用 React、TypeScript 和 Ant Design / ProComponents；后端使用 Go、ConnectRPC、GORM / Gen 和 goose。**Proto 定义接口的请求和响应。Resource 定义后台字段规则，以及新增、查询、修改、删除（CRUD）功能。**

## 项目用途

本项目提供可独立运行的管理后台，以及供业务项目接入的 Go 公共包和前端组件。应用预装登录、用户、角色、菜单和商品管理。业务开发直接在当前应用中增加模块、接口和页面。

- 体验自带后台：按下方「快速开始」运行。
- 接入业务功能：按[模块开发教程](docs/new-module-tutorial.md)创建并持续扩展业务模块。
- 修改项目源码：从 [开发说明](docs/development.md) 和 [Resource 开发手册](docs/ai-cookbook.md) 开始。

本系统的接口供后台前端使用。业务前台使用自己的接口，可以与后台共用从数据库生成的 Go 模型。Go 模型用结构体表示数据表。

业务项目的后台维护共享业务表的迁移文件。迁移文件用于修改表结构或初始化数据。框架公共包（Core）提供系统表迁移。应用在 `backend/migrations/` 中统一维护系统与业务迁移。

## 目录结构

```
proto/                         # 框架公共 API（.proto）
admin.modules.json             # 应用模块清单
buf.yaml                       # 生成的系统与模块协议 workspace
backend/                       # 独立 Go 模块：公共包、服务、迁移和模型生成命令
backend/coremigrations/         # Core 系统迁移来源
backend/migrations/             # 自带应用的集中迁移目录
frontend/packages/admin-proto  # 公共 Proto TS 包
frontend/packages/admin-core   # 公共前端运行时与系统页面
frontend/apps/reference        # 自带管理端入口
backend/internal/modules/product/      # 商品服务、协议、模型、Resource 与测试
frontend/apps/reference/src/modules/product/ # 商品页面、协议与 Resource 产物
```

> 公共协议生成物位于 `backend/gen` 与 `frontend/packages/admin-proto/src`，随源码提交。生成命令见[开发说明](docs/development.md)。

## 快速开始：体验自带后台

完成下面的步骤后，你可以登录自带后台，查看用户、角色、菜单和部门。业务接入请继续阅读 [模块开发教程](docs/new-module-tutorial.md)。

先安装 **Go 1.26+ 和 Node.js 20+**。仓库已包含生成的代码，首次启动可以直接使用。修改接口时，再安装协议生成工具 [buf](https://buf.build/docs/installation)。

先克隆仓库，并进入仓库目录。下面每个命令块都会回到仓库根目录。

先准备一个可连接的数据库。数据库要求及连接方式见[数据库配置](docs/configuration.md#数据库配置)；需要新建本地数据库时，按[本地数据库准备](docs/database-setup.md)操作。

### 1. 配置数据库连接

首次使用时，运行以下命令创建 `.env`。已有 `.env` 时，保留文件，并核对下方的数据库配置。

```bash
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
cp .env.example .env
```

在 `.env` 中设置数据库类型和连接串。以下示例与[本地 PostgreSQL 准备方法](docs/database-setup.md#postgresql)一致。使用自己的数据库时，修改地址、账号、密码和数据库名。

```dotenv
ADMIN_DB_DRIVER=postgres
ADMIN_DB_DSN=host=localhost port=5433 user=admin password=admin dbname=abysstract_admin sslmode=disable
```

### 2. 初始化系统表和管理员

```bash
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
( cd backend && go run ./cmd/migrate )
```

命令根据 `ADMIN_DB_DRIVER` 选择 `backend/migrations/` 下对应数据库的迁移目录。成功退出后，数据库中已创建系统表和初始管理员。迁移会记录已执行版本，后续重复运行只应用新版本。

### 3. 启动后端

在第一个终端运行以下命令。启动后，保持这个终端运行：

```bash
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
( cd backend && go run ./cmd/server )
```

后端默认监听 `http://localhost:8080`。在另一个终端检查数据库就绪状态：

```bash
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
curl --fail http://localhost:8080/readyz
```

命令成功退出且返回 HTTP 200，表示后端可以连接数据库。

### 4. 启动前端

在第二个终端进入本仓库，安装依赖并构建公共包，然后启动自带应用：

```bash
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
(
  cd frontend &&
  npm ci &&
  npm run build -w @abysstract/admin-proto -w @abysstract/admin-core &&
  npm run dev -w @abysstract/admin-reference -- --strictPort
)
```

打开 **http://localhost:5173**，使用 **admin / admin123** 登录。这是新数据库的初始账号；已有数据库使用已设置的密码。若提示修改初始密码，按页面提示完成。

### 5. 确认运行成功

- 登录后显示管理后台的菜单和当前用户。
- 打开“系统管理 → 用户管理”，列表中能看到管理员。
- 打开角色、菜单和部门页面，列表正常加载。
- 打开“商品管理 → 商品列表”，可以新建、编辑和删除商品。

至此，后台已经运行。接下来按[模块开发教程](docs/new-module-tutorial.md)创建并持续扩展业务模块，继续使用当前配置、数据库和账号。

### 停止与再次启动

前后端终端分别按 `Ctrl+C` 停止进程。数据库的停止方法见[本地数据库准备](docs/database-setup.md#停止与再次启动)。

再次使用时，先确认数据库已启动，再启动后端和前端。依赖变化时重新运行 `npm ci`。有新迁移时，先执行迁移。

## 配置与开发

- 数据库要求、连接信息和服务环境变量见[配置说明](docs/configuration.md)。
- 本地数据库的创建与启停见[数据库准备](docs/database-setup.md)。
- 修改表结构、接口或 Resource 后，按[开发说明](docs/development.md)执行迁移和生成。
- 业务接入从[模块教程](docs/new-module-tutorial.md)开始，字段与回调规则见[Resource 手册](docs/ai-cookbook.md)。

## 默认账号与内置保护

内置管理员为 `sys_user.id = 1`（`admin`）。账号操作遵循以下规则：

- 内置管理员不可删除、不可停用。
- 不可删除 / 停用当前登录用户。
- 内置管理员密码仅限其本人重置。

## 常见问题

- **启动报「缺少 sys_user 等系统表」**：先执行 `go run ./cmd/migrate`。
- **前端登录跨域失败**：检查 `ADMIN_CORS_ORIGINS` 是否包含前端来源（dev 默认放通）。
- **生产启动被拒绝**：按提示设置 `ADMIN_JWT_SECRET`、`ADMIN_DB_DSN`、`ADMIN_CORS_ORIGINS`。

## 文档

- [模块开发教程](docs/new-module-tutorial.md) — 创建模块、增加表和字段、生成代码及安装升级
- [商品手写扩展参考](docs/admin-core-getting-started.md) — 现有商品模块的 Proto、Resource 与 Hook
- [公共 API 用法](docs/admin-core-usage.md) — 最小导入、回调、权限、页面注册与故障处理
- [项目理解教程](docs/project-guide.md) — 项目组成、常用概念、接口调用与开发入口
- [配置说明](docs/configuration.md) — 环境变量、运行模式、安全检查
- [本地开发](docs/development.md) — 迁移、完整生成、局部生成与开发检查
- [权限模型](docs/permission.md) — RBAC、权限标识、数据范围、内置账号保护
- [模块指南](docs/module-guide.md) — 新增功能模块的推荐步骤
- [Resource 开发手册](docs/ai-cookbook.md) — 字段声明、业务回调及数据范围
- [部署](docs/deployment.md) — 构建、生产配置检查清单

## 社区与许可

- [LICENSE](LICENSE) — Apache-2.0
- [贡献指南](CONTRIBUTING.md)
- [安全策略](SECURITY.md)
- [更新日志](CHANGELOG.md)

