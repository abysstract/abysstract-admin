# 商品模块的手写接口扩展参考

新业务模块的持续开发入口见[模块教程](new-module-tutorial.md)。本文说明现有商品模块的手写 Proto、Go Resource 和业务扩展方式。商品模块当前仍需手写 Proto 和 Go Resource，`module generate product` 执行它的下游生成链路。

完成 [README 快速开始](../README.md#快速开始体验自带后台)后，商品管理已经可用。本教程在现有商品模块中增加品牌字段。继续使用根目录的 `.env`、数据库、账号和前端依赖。

SQL 示例分为 PostgreSQL 和 MySQL 两组。MariaDB 使用 MySQL 示例，迁移目录改为 `backend/migrations/mariadb`。SQLite 在 `backend/migrations/sqlite` 中创建迁移，本例的增列 SQL 也适用。

## 1. 认识商品模块

打开 **商品管理 → 商品列表**，新建一条商品，修改价格并刷新，确认数据保存成功。

商品代码已放在应用对应目录中：

| 位置 | 职责 |
|---|---|
| `backend/internal/app/modules.go` | 组合系统与商品模块 |
| `backend/internal/app/server.go` | 组合应用路由、全局中间件和 HTTP 服务 |
| `backend/internal/modules/product/` | 商品接口、数据范围、回调与事件 |
| `backend/internal/modules/product/definition/` | 商品 Resource 字段和操作规则 |
| `backend/migrations/` | 系统与商品迁移 |
| `backend/internal/modules/product/model/` | 生成的商品模型与查询代码 |
| `backend/internal/modules/product/proto/company/product/v1/` | 商品接口协议 |
| `frontend/apps/reference/src/modules.ts` | 组合前端模块 |
| `frontend/apps/reference/src/modules/product/pages/ProductList.tsx` | 商品页面 |

后端迁移创建商品表、事件表、菜单绑定表，以及“商品只读”和“商品编辑”角色。为测试账号分配角色，可以验证操作权限和数据范围。新业务使用 `resources/*.json` 和 `module generate` 生成公共代码，步骤见[模块教程](new-module-tutorial.md)。

## 2. 为商品增加“品牌”

下面亲手增加一个可选的品牌字段，最多 60 个字符。完成后，列表会显示品牌，新建和编辑表单都可以填写，刷新页面后数据仍然存在。

安装协议生成工具 [buf](https://buf.build/docs/installation)。以下命令在仓库根目录执行。

先创建一条编号为 `P-BEFORE` 的商品，并记下名称和价格。后面用它检查已有数据。然后在后端和前端终端按 `Ctrl+C`，停止当前应用。

### 2.1 为已有数据库增加一列

先安装与后端一致的 Goose 命令行工具：

```bash
go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
```

#### PostgreSQL

创建商品品牌列的迁移文件：

```bash
( cd backend && "$(go env GOPATH)/bin/goose" -dir migrations/postgres create product_brand sql )
```

将新文件内容替换为：

```sql
-- +goose Up
ALTER TABLE product_item ADD COLUMN brand VARCHAR(60) NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE product_item DROP COLUMN brand;
```

#### MySQL

创建商品品牌列的迁移文件：

```bash
( cd backend && "$(go env GOPATH)/bin/goose" -dir migrations/mysql create product_brand sql )
```

将新文件内容替换为：

```sql
-- +goose Up
ALTER TABLE product_item ADD COLUMN brand VARCHAR(60) NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE product_item DROP COLUMN brand;
```

#### 执行迁移

`Up` 定义升级操作，`Down` 定义回滚操作。执行后，旧商品的品牌列为默认的空字符串。应用按 `ADMIN_DB_DRIVER` 选择迁移目录，重新编译时会收录新增的 SQL 文件。

执行迁移。再次运行同一命令时，goose 会根据 `goose_db_version` 跳过已执行版本：

```bash
( cd backend && go run ./cmd/migrate )
```

### 2.2 了解 Go 模型怎样更新

Go 模型用结构体表示数据表。第 2.4 节的 `module generate product` 会读取数据库，并更新 `backend/internal/modules/product/model/` 中的模型和查询代码。生成的商品模型会包含 `Brand` 字段。其他 Go 服务也可以导入这个模型包。

完成下面的 Proto 和 Resource 修改后，再执行生成命令。

### 2.3 让接口能够读写品牌

打开 `backend/internal/modules/product/proto/company/product/v1/product.proto`。在下列三个消息定义（`message`）中分别追加字段。保留已有字段和编号：

| message | 追加内容 | 用途 |
|---|---|---|
| `Product` | `string brand = 10;` | 列表和详情返回品牌 |
| `CreateProductRequest` | `string brand = 6;` | 创建商品时填写品牌 |
| `UpdateProductRequest` | `optional string brand = 5;` | 编辑商品时修改或清空品牌 |

更新请求中的 `optional` 表示这个字段可以省略。省略 `brand` 时，后端保留原值。传入空字符串时，后端清空品牌。

### 2.4 声明字段规则并生成代码

打开 `backend/internal/modules/product/definition/product.go`，在 `Fields` 列表中追加：

```go
schema.F("Brand", schema.Label("品牌"), schema.MaxLen(60)),
```

`Brand` 是生成后的 Go 模型字段名。`Label("品牌")` 为列表、新建和编辑表单设置同一个标签。`MaxLen(60)` 将输入长度限制为 60 个字符。

在仓库根目录执行商品模块的生成命令。工具自动读取项目的 `.env`：

```bash
( cd backend && go run ./cmd/admin module generate product )
```

命令会更新商品模型、查询代码和 Resource 实现，并刷新已安装模块的 Go/TS 协议类型。完成后，商品接口和读写代码中会包含品牌字段。

商品模块使用手写 Proto 和 Go Resource，命令根据前面修改的定义生成代码。提交前的编译、类型检查和测试见[开发说明](development.md#提交前检查)。

### 2.5 使用页面默认行为

本练习使用默认页面行为，保留 `frontend/apps/reference/src/modules/product/pages/ProductList.tsx` 的现有内容。生成后，页面会有以下行为：

- 列表显示“品牌”列。
- 新建和编辑表单显示“品牌”输入框。
- 新建表单中的品牌为空字符串。
- 编辑表单显示商品已保存的品牌。

这些行为来自 Resource 字段声明和生成的前端配置。需要特殊组件、格式或初值时，再修改页面配置。配置的优先顺序见[普通字段的页面默认行为](admin-core-usage.md#普通字段的页面默认行为)。

### 2.6 验证从页面到数据库的完整流程

在当前应用中重新启动服务。后端终端执行：

```bash
( cd backend && go run ./cmd/server )
```

另开终端，进入仓库根目录执行：

```bash
( cd frontend && npm run dev -w @abysstract/admin-reference -- --strictPort )
```

打开 http://localhost:5173，依次验证：

1. 查询 `P-BEFORE`，原有名称和价格保持不变，品牌列为空。
2. 新建商品 `P-BRAND`，品牌填写“示例品牌”，保存后列表显示该品牌。
3. 刷新浏览器，再次查询 `P-BRAND`，品牌仍然存在。
4. 编辑该商品，品牌输入框应回填“示例品牌”。只修改价格，保存后品牌仍然保留。
5. 再次编辑并清空品牌，保存、刷新后品牌为空。

这次修改串起了完整流程：**数据库增加列 → 模型映射 → 接口增加字段 → Resource 声明规则 → 生成代码 → 页面展示与提交**。接入更多普通字段时，可以沿用这个顺序。

## 3. 移除商品示例

先停止前后端服务，在仓库根目录预览卸载。命令列出模块清单、模块加载代码、协议文件和数据库迁移等变更：

```bash
( cd backend && go run ./cmd/admin module uninstall product --dry-run )
```

执行卸载，更新已安装模块列表并执行清理迁移：

```bash
( cd backend && go run ./cmd/admin module uninstall product )
```

审查 Git diff。迁移清理商品模块拥有的菜单、示例角色及其用户角色、角色部门和角色菜单关联，保留商品表、事件表及数据。应用历史迁移持续保留。命令成功后，可将源码移到临时目录，供后面恢复使用：

```bash
product_archive=$(mktemp -d)
mv backend/internal/modules/product "$product_archive/backend-product"
mv frontend/apps/reference/src/modules/product "$product_archive/frontend-product"
```

重新启动应用并登录，确认系统管理可用，商品菜单及示例角色已清理。源码移走后，安装和清理快照留在应用迁移中，新库可以执行完整历史。若需要删除业务数据，另建应用迁移，明确列出要删除的业务表。

恢复源码：

```bash
mv "$product_archive/backend-product" backend/internal/modules/product
mv "$product_archive/frontend-product" frontend/apps/reference/src/modules/product
```

在仓库根目录重新接入商品模块，恢复应用加载和公共生成任务，并执行新的安装迁移，写入商品菜单和示例角色：

```bash
( cd backend && go run ./cmd/admin module update product )
```

检查模块加载代码、公共生成清单和 Buf 配置是否与安装清单一致：

```bash
( cd backend && go run ./cmd/admin module check )
```

恢复的是卸载前的源码及其生成文件，可以直接复用保留的业务数据。按 README 的方式重启后端和前端，登录后确认商品菜单和原有商品数据可用。此前清理的角色授权按当前需求重新配置。

## 4. 从零创建业务模块

按[笔记 CRUD 模块教程](new-module-tutorial.md)创建独立业务模块。先建表并声明业务字段，再用 `module generate` 生成标准协议、Resource、服务注册、权限菜单与前端页面，完成安装、运行、移除和重新安装。

模块声明和 Seed 字段说明见[模块指南](module-guide.md)。

## 继续阅读

- [模块指南](module-guide.md)：接口、权限与页面注册。
- [Resource 开发手册](ai-cookbook.md)：从数据库到页面的开发流程。
- [配置说明](configuration.md)：根目录配置与迁移。
