# 新建独立应用

本教程在一个新仓库中搭建后台应用：应用有自己的 Go 模块路径和前端目录，通过 `go get` 和 npm 依赖本框架，不复制框架源码。完成后，你会得到一个可以登录的后台，以及一个带增删改查页面的笔记模块 `notes`。

示例应用名为 `shop`，Go 模块路径为 `example.com/shop`，数据库使用 PostgreSQL。使用 MySQL 时，按各步骤中的说明替换驱动目录和 SQL。

如果只想体验框架自带的后台，或在框架仓库内开发模块，请看 [README 快速开始](../README.md#快速开始体验自带后台) 和[模块开发教程](new-module-tutorial.md)。

## 准备

- Go 1.26+、Node.js 20+。
- [buf](https://buf.build/docs/installation)：`module generate` 用它生成协议代码，需要在 `PATH` 中。
- 一个空数据库。新建本地数据库的方法见[本地数据库准备](database-setup.md)。

本教程需要框架 v0.3.0 或更新版本。下文命令都在应用根目录 `shop/` 执行。

完成后的目录结构：

```
shop/
  go.mod                      # module example.com/shop
  admin.modules.json          # 应用加载的模块与前端目录
  .env                        # 数据库连接
  backend/
    cmd/server/main.go        # 后端入口
    cmd/migrate/main.go       # 迁移入口
    internal/app/app.go       # 选择模块；modules.gen.go 由 admin module 维护
    internal/modules/notes/   # 业务模块（第 6 步生成）
    migrations/               # 系统表、业务表和模块菜单迁移
  frontend/
    package.json
    src/main.tsx              # 前端入口；modules.gen.ts 由 admin module 维护
    src/modules/notes/        # 业务页面（第 6 步生成）
```

## 1. 创建 Go 模块并安装命令

```bash
mkdir shop && cd shop
go mod init example.com/shop
go get github.com/abysstract/abysstract-admin@v0.3.0
go get -tool github.com/abysstract/abysstract-admin/backend/cmd/admin github.com/abysstract/abysstract-admin/backend/cmd/admin-migrations
```

`go get -tool` 把两个命令记录在 `go.mod` 的 `tool` 中，版本与框架依赖一致，之后用 `go tool admin` 和 `go tool admin-migrations` 运行。确认命令可用：

```bash
go tool admin module --help
```

输出列出 `new`、`resource`、`generate`、`update` 等子命令。

## 2. 声明应用

在应用根目录创建 `admin.modules.json`：

```json
{
  "frontendRoot": "frontend/src",
  "modules": ["system"]
}
```

- `frontendRoot` 是前端源码目录。`module new` 在 `<frontendRoot>/modules/<名称>` 创建页面，`module sync` 和 `module update` 在这里写入 `modules.gen.ts`。省略时为框架仓库自带应用的 `frontend/apps/reference/src`。
- `modules` 列出已安装的模块。`"system"` 表示框架的系统模块（登录、用户、角色、菜单等），它的代码已经由框架生成；业务模块由 `module update` 追加。

创建 `.env`，填写数据库连接：

```dotenv
ADMIN_DB_DRIVER=postgres
ADMIN_DB_DSN=host=localhost port=5432 user=shop password=shop dbname=shop sslmode=disable
```

使用 MySQL 时写 `ADMIN_DB_DRIVER=mysql` 和 `ADMIN_DB_DSN=shop:shop@tcp(127.0.0.1:3306)/shop`。其他配置项见[配置说明](configuration.md)。命令从当前目录或上一级目录读取 `.env`。

## 3. 编写后端入口

后端由三部分组成：选择模块的 `internal/app`、迁移入口和服务入口。

`backend/internal/app/app.go`：

```go
// Package app selects the application's modules.
package app

import "github.com/abysstract/abysstract-admin/backend/module"

// Modules returns the modules listed in admin.modules.json. admin module
// sync and update keep modules.gen.go in step with that file.
func Modules() ([]module.Definition, error) { return selectedModules() }
```

`backend/migrations/migrations.go`：

```go
// Package migrations holds the application's goose migrations: Core system
// tables imported by admin-migrations sync, business tables, and module menus
// written by admin module update.
package migrations

import (
	"context"
	"embed"
	"io/fs"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/coremigrations"
	"github.com/abysstract/abysstract-admin/backend/database"
)

//go:embed postgres/*.sql
var sqlFiles embed.FS

// Run applies every pending migration of the configured database.
func Run(ctx context.Context, db *gorm.DB) error {
	driver := database.DriverOf(db)
	selected, err := fs.Sub(sqlFiles, driver)
	if err != nil {
		return err
	}
	return coremigrations.Run(ctx, db, selected, goMigrations(driver)...)
}
```

使用 MySQL 时把 `postgres/*.sql` 改为 `mysql/*.sql`。同时支持多种数据库时，为每种数据库列出目录，例如 `//go:embed postgres/*.sql mysql/*.sql`。`goMigrations` 由下一步的 `module sync` 生成。

`backend/cmd/migrate/main.go`：

```go
// Command migrate applies the application's migrations.
package main

import (
	"context"
	"errors"
	"log"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/database"

	"example.com/shop/backend/migrations"
)

func main() {
	cfg := adminconfig.Load()
	if err := cfg.ValidateDatabase(); err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		log.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}
	if err := errors.Join(migrations.Run(ctx, db), pool.Close()); err != nil {
		log.Fatal(err)
	}
	log.Println("migrate OK")
}
```

`module update` 在 `backend/` 下执行 `go run ./cmd/migrate`，因此迁移入口必须放在 `backend/cmd/migrate`。

`backend/cmd/server/main.go`：

```go
// Command server runs the shop admin backend.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	"github.com/abysstract/abysstract-admin/backend/adminconfig"

	"example.com/shop/backend/internal/app"
)

func main() {
	cfg := adminconfig.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}
	modules, err := app.Modules()
	if err != nil {
		log.Fatal(err)
	}
	server, err := adminapp.New(adminapp.Options{Config: *cfg, Modules: modules})
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("listening on %s", cfg.Addr)
	if err := server.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
```

需要自定义路由、中间件或 HTTP 服务时，改用 `adminapp.App.Start` 取得 handler 并自己创建 `http.Server`，写法见[应用启动与关闭](admin-core-usage.md#应用启动与关闭)。

## 4. 初始化系统表

导入框架的系统表迁移，再生成模块加载代码和迁移函数列表：

```bash
go tool admin-migrations sync --dir backend/migrations
go tool admin module sync
go mod tidy
```

`admin-migrations sync` 按 `ADMIN_DB_DRIVER` 在 `backend/migrations/postgres/` 下创建 3 个 `*_core_*` 文件，并打印它们的路径。`module sync` 打印写入的文件：

```
backend/internal/app/modules.gen.go
backend/migrations/go_registry.gen.go
frontend/src/modules.gen.ts
```

`modules.gen.go` 导入框架的 `backend/system`；`go_registry.gen.go` 只包含存在的数据库目录（这里是 `postgres`），导入路径是 `example.com/shop/backend/migrations/postgres`。

执行迁移：

```bash
go run ./backend/cmd/migrate
```

输出以 `migrate OK` 结束，数据库中已创建系统表和管理员 `admin / admin123`。

系统迁移的版本号取导入时的当前时间。新应用先执行 `admin-migrations sync`，再编写业务迁移；业务迁移的版本号不要晚于当前时间，否则之后的 `sync` 会拒绝执行。

## 5. 启动后端和前端

启动后端，并保持终端运行：

```bash
go run ./backend/cmd/server
```

在另一个终端检查后端：

```bash
curl --fail http://localhost:8080/readyz
```

命令成功退出，表示后端已连接数据库。

接着创建前端。`frontend/package.json`：

```json
{
  "name": "shop-admin",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc --noEmit && vite build",
    "typecheck": "tsc --noEmit"
  }
}
```

安装框架前端包、peer 依赖和构建工具（版本与 Go 依赖保持一致）：

```bash
cd frontend
npm install @abysstract/admin-core@^0.3.0 @abysstract/admin-proto@^0.3.0 \
  @bufbuild/protobuf@^2 @connectrpc/connect@^2 @connectrpc/connect-web@^2 \
  antd@^5 @ant-design/icons@^5 @ant-design/pro-components@^2 \
  react@^18 react-dom@^18 react-router-dom@^6
npm install -D typescript@^5 vite@^5 @vitejs/plugin-react@^4 @types/react@^18 @types/react-dom@^18
cd ..
```

`frontend/index.html`：

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Shop 后台</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

`frontend/vite.config.ts`：

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: { port: 5173 },
});
```

`frontend/tsconfig.json`：

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "noEmit": true
  },
  "include": ["src"]
}
```

`frontend/src/vite-env.d.ts`：

```ts
/// <reference types="vite/client" />
```

`frontend/src/main.tsx`：

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import zhCN from "antd/locale/zh_CN";
import { bindAdminAppPageLifecycle, createAdminApp, createLocalAuthStorage } from "@abysstract/admin-core";
import { modules } from "./modules.gen";
import "antd/dist/reset.css";
import "@abysstract/admin-core/styles.css";

const mode = import.meta.env.PROD ? "production" : "development";
const apiBaseUrl = import.meta.env.VITE_API_BASE ?? "http://localhost:8080";

const application = createAdminApp({
  host: {
    apiBaseUrl,
    mode,
    authStorage: createLocalAuthStorage("shop_admin_token"),
    title: "Shop 后台",
    locale: zhCN,
  },
  modules,
});

const root = ReactDOM.createRoot(document.getElementById("root")!);
root.render(
  <React.StrictMode>
    <BrowserRouter>
      <application.Root />
    </BrowserRouter>
  </React.StrictMode>,
);
bindAdminAppPageLifecycle({ application, root });
```

启动前端：

```bash
( cd frontend && npm run dev )
```

打开 **http://localhost:5173**，用 **admin / admin123** 登录，可以看到“系统管理”下的用户、角色、菜单和部门页面。

## 6. 创建笔记模块

后续步骤会修改后端代码。先在后端终端按 `Ctrl+C` 停止后端，前端保持运行。

### 6.1 创建模块

```bash
go tool admin module new notes
```

命令创建 `backend/internal/modules/notes/` 和 `frontend/src/modules/notes/`。`module.json` 记录模块各部分的位置，生成的 Go 代码使用 `example.com/shop/backend/...` 导入应用自己的包，例如 `definition/codegen.go` 中的 `GenQueryImport: "example.com/shop/backend/internal/modules/notes/query"`。

### 6.2 创建业务表

业务表以模块名为前缀。用当前 UTC 时间作为版本号创建迁移文件：

```bash
touch "backend/migrations/postgres/$(date -u +%Y%m%d%H%M%S)_notes_note.sql"
```

填写 SQL：

```sql
-- +goose Up
CREATE TABLE notes_note (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  pinned BOOLEAN NOT NULL DEFAULT FALSE,
  update_time TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose Down
DROP TABLE notes_note;
```

MySQL 写在 `backend/migrations/mysql/` 下：

```sql
-- +goose Up
CREATE TABLE notes_note (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  title VARCHAR(200) NOT NULL,
  pinned BOOLEAN NOT NULL DEFAULT FALSE,
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
);
-- +goose Down
DROP TABLE notes_note;
```

执行迁移：

```bash
go run ./backend/cmd/migrate
```

输出以 `migrate OK` 结束。

### 6.3 声明字段并生成代码

根据表结构创建资源声明：

```bash
go tool admin module resource notes notes_note --title 笔记
```

命令创建 `backend/internal/modules/notes/resources/note.json`。把字段标签改成中文，并让标题必填、可搜索：

```json
{
  "name": "Note",
  "table": "notes_note",
  "title": "笔记",
  "updatedAt": "update_time",
  "fields": [
    { "column": "title", "label": "标题", "required": true, "searchable": true },
    { "column": "pinned", "label": "置顶" }
  ]
}
```

生成模型、协议、Resource 和页面：

```bash
go tool admin module generate notes
```

命令列出写入的声明文件，然后依次生成 Model/query、Go 与 TypeScript 协议代码（调用 `buf`）和 Resource 实现，最后打印 `Generation complete.`。生成的 Go 协议包位于 `backend/internal/modules/notes/gen/proto`，导入路径为 `example.com/shop/backend/internal/modules/notes/gen/proto/company/notes/v1`；TypeScript 协议和页面位于 `frontend/src/modules/notes/`。

### 6.4 安装模块

```bash
go tool admin module update notes
```

命令把模块加入 `admin.modules.json`，更新 `modules.gen.go` 和 `modules.gen.ts`，在 `backend/migrations/postgres/` 下写入菜单迁移，然后执行迁移。输出以模块状态结束：

```
Module notes: registered=true
Registration files current=true
Pending application migrations=0
Database: install applied at <版本号>; pending module migrations=0
```

## 7. 验证增删改查

重新启动后端：

```bash
go run ./backend/cmd/server
```

刷新浏览器并重新登录。左侧出现“notes → 笔记列表”，可以新建、编辑、搜索和删除笔记；标题为空时提交会提示“字段 title 不能为空”。

也可以用 Connect JSON 接口验证。在另一个终端执行：

```bash
token=$(curl -s -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' \
  http://localhost:8080/abysstract.admin.v1.AuthService/Login | sed 's/.*"token": *"\([^"]*\)".*/\1/')
api=http://localhost:8080/company.notes.v1.NoteService
call() { curl -s -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$2" "$api/$1"; echo; }
call CreateNote '{"title":"第一条","pinned":true}'
call UpdateNote '{"id":"1","title":"已修改"}'
call ListNotes '{"page":{"page":1,"pageSize":10},"keyword":"修改"}'
call DeleteNote '{"id":"1"}'
```

前三条分别返回新建、修改后的笔记和包含该笔记的列表，`DeleteNote` 返回 `{}`。

至此，独立应用已经可以按“迁移 → `module resource` 或修改 JSON → `module generate` → `module update`”持续开发。字段规则、Hook 和自定义接口见[模块开发教程](new-module-tutorial.md)和[模块指南](module-guide.md)；那些教程中的 `go run ./cmd/admin module ...` 在独立应用中写作 `go tool admin module ...`。

## 与框架仓库的区别

| 项目 | 框架仓库 | 独立应用 |
| --- | --- | --- |
| 运行命令 | `cd backend && go run ./cmd/admin module ...` | `go tool admin module ...` |
| 生成代码的导入路径 | `github.com/abysstract/abysstract-admin/backend/...` | 从应用 `go.mod` 推导，如 `example.com/shop/backend/...` |
| 系统模块 | `backend/system/module.json`，由仓库生成 | `admin.modules.json` 中写 `"system"`，使用框架已生成的代码 |
| 前端目录 | `frontend/apps/reference/src` | `admin.modules.json` 的 `frontendRoot` |
| 迁移方言 | 四种数据库目录 | 只为 `backend/migrations/` 下已存在的数据库目录生成菜单迁移和函数列表 |
| `sync` 额外维护 | `buf.yaml`、`backend/internal/codegen/` 下的生成清单 | 无 |

`module generate` 在命令进程内生成 Model/query，用 `buf` 生成协议代码。框架的公共 Proto（如 `abysstract/admin/v1/common.proto`）从 `go.mod` 依赖的框架模块读取，生成的 TypeScript 代码从 `@abysstract/admin-proto` 导入它们。Resource 实现用 `go run` 编译应用自己的 `definition` 包生成。

`module.json` 中的路径决定生成位置：`backend.package` 放模块代码，`resource.package` 放 Resource 声明，`generation.modelOutput`、`goOutput`、`protoSource` 和 `tsOutput` 分别放 Model/query、Go 协议、Proto 源文件和 TypeScript 协议，`frontend.entry` 所在目录放页面。字段说明见[模块声明](module-guide.md#模块声明)。

## 升级框架

把 Go 依赖、命令和前端包升级到同一版本：

```bash
go get github.com/abysstract/abysstract-admin@vX.Y.Z
go get -tool github.com/abysstract/abysstract-admin/backend/cmd/admin github.com/abysstract/abysstract-admin/backend/cmd/admin-migrations
go tool admin-migrations sync --dir backend/migrations
go tool admin module sync
( cd frontend && npm install @abysstract/admin-core@X.Y.Z @abysstract/admin-proto@X.Y.Z )
```

`admin-migrations sync` 只导入新版本增加的系统迁移。之后执行 `go run ./backend/cmd/migrate`，并按[更新日志](../CHANGELOG.md)处理破坏性变更。

## 常见问题

- **`module generate` 报 `buf is required`**：安装 buf，并确认它在 `PATH` 中。
- **`locate github.com/abysstract/abysstract-admin sources`**：先执行 `go mod download`，或检查 `go.mod` 中的 `replace` 路径。
- **`no migration directory`**：先执行第 4 步，确保 `backend/migrations/<数据库>` 目录存在。
- **`go run ./cmd/migrate` 失败**：`module update` 在 `backend/` 下运行迁移入口，确认第 3 步的 `backend/cmd/migrate/main.go` 存在。
- **本地联调未发布的框架改动**：`go mod edit -replace github.com/abysstract/abysstract-admin=/absolute/path/to/abysstract-admin`，前端按[安装前端包](admin-core-usage.md#安装前端包)打包后安装。
