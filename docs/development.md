# 本地开发

## 工具与首次启动

安装 Go 1.26+、Node.js 20+；修改协议时还需要 [buf](https://buf.build/docs/installation)。数据库要求及本地准备方法见[数据库配置](configuration.md#数据库配置)和[数据库准备](database-setup.md)。

首次运行按 [README 快速开始](../README.md#快速开始体验自带后台)操作。仓库已包含生成文件，首次启动可以直接使用。

## 修改与完整生成

在 `backend/internal/modules/<module>/resources/*.json` 维护暴露字段和业务规则。新增表或字段的流程为：

1. 新增并执行数据库迁移，更新表结构。
2. 新表执行 `module resource <模块名> <表名>` 创建初始 JSON；随后修改资源声明，选择暴露列并填写标签、校验、搜索和排序规则。
3. 执行 `module generate <名称>`，生成 Model/query、Proto、Resource、服务注册代码和页面入口。
4. 按需在保留的手写文件中添加 Hook、Overrides 或页面定制。
5. 首次接入执行 `update`。菜单、权限或示例角色有变化时再执行 `update`；只改字段时可先 `module status` 查看声明差异。启动或重启后端，在前端页面验证功能。

以下示例用于已经安装的 notes 模块，命令在仓库根目录执行。

执行数据库迁移：

```bash
( cd backend && go run ./cmd/migrate )
```

修改 notes 的 `resources/*.json` 后，生成代码：

```bash
( cd backend && go run ./cmd/admin module generate notes )
```

菜单或权限有变化时，更新已安装模块：

```bash
( cd backend && go run ./cmd/admin module update notes )
```

在后端终端停止原进程，重新启动：

```bash
( cd backend && go run ./cmd/server )
```

前端使用开发服务；尚未启动时，在另一个终端执行：

```bash
( cd frontend && npm run dev )
```

这条命令会先构建公共包，再启动自带应用。

`go run` 会编译并启动后端，前端开发服务会加载修改后的页面。日常修改后直接查看页面结果，编译、类型检查和测试集中在[提交前检查](#提交前检查)执行。

首次开发和新增第二张表的完整步骤见[持续开发教程](new-module-tutorial.md)。生成的 Proto、Resource 定义和 `resources.lock.json` 一并提交；字段编号由锁文件维护。

应用从自己的 `.env` 读取数据库配置，配置方式见[数据库配置](configuration.md)。新增模块先执行 `admin module new <名称>`，表名使用 `<模块名>_<表名>`。`module.json` 自动填写模型、协议与 Resource 生成入口；字段说明与文件影响见[模块指南](module-guide.md)。

应用运行模块清单为根 `admin.modules.json`。公共生成只处理该清单中的模块；`module generate NAME` 直接读取指定模块的配置，支持安装前生成。`sync` 更新已安装模块的加载代码、开发任务清单及 Buf workspace，固定 Buf 规则在 `buf.base.yaml` 维护。提交前执行 `module check`，确认加载代码、公共生成清单和 Buf 配置与 `admin.modules.json` 一致，并提交更新后的模块加载代码。

### 手写接口与底层生成命令

系统与现有商品模块在手写 Proto、Go `schema.Resource` 中维护接口和行为。它们的流程是“迁移 → 修改 Proto 和 Resource → 生成”，商品字段练习见[手写商品扩展参考](admin-core-getting-started.md#2-为商品增加品牌)。商品模块迁入 JSON 配置的工作仍需完成。

在仓库根目录执行：

```bash
( cd backend && go run ./cmd/migrate )
sh scripts/generate.sh
```

`generate.sh` 依次运行 sync、Model/query、协议类型及 Resource 实现生成。**它处理已有的 Proto 和 Go Resource 定义；修改 `resources/*.json` 后，应执行 `module generate <名称>` 来更新这些定义。** `go generate ./...` 同样执行下游 Proto → Resource 生成，使用已提交的模型。

统一入口可用 `--phase model|offline|proto|resource` 选择阶段（默认 `all`），用 `--module product` 或 `--module system` 选择模型与 Resource 任务。例如：

```bash
sh scripts/generate.sh --phase model --module product
sh scripts/generate.sh --phase offline --module product
```

协议阶段统一刷新已选模块协议及模块间依赖，然后执行选中模块的 Resource。`offline` 使用已提交模型，无需连接数据库。模型生成按 GORM 文件头识别所属产物，清理当前模块的过期文件并保留手写文件。

## 局部生成

| 修改内容 | 从仓库根目录执行 |
|---|---|
| 已选模块的 Proto | `node frontend/tools/generate-proto.mjs` |
| 已选模块的 Resource | `cd backend && go run ./cmd/adminresource-gen` |
| 系统与业务 Model/query | `cd backend && go run ./cmd/modelgen` |
| 离线 Proto、Resource 与框架夹具 | `cd backend && go generate ./...` |

本仓库提交的模型统一从 PostgreSQL 生成。业务项目使用自己的目标数据库生成模型。列名、可空性、主键和默认值来自数据库；数据库整数类型通过共用映射固定 Go 宽度，并识别驱动返回的大小写名称；MySQL/MariaDB 的无符号整数映射为能容纳其范围的有符号类型（`INT UNSIGNED`、`BIGINT UNSIGNED` 为 `int64`，`SMALLINT UNSIGNED` 为 `int32`），规则与取舍见[模块指南](module-guide.md#mysqlmariadb-无符号整数)。模型生成按 `tablePrefix` 的 `<前缀>_` 边界选择表，模块前缀相同或互为前缀时报错。SQLite 的整数主键按 64 位自增标识处理；其他可空列保留数据库的 NULL 含义。特殊规则在模块的 `codegen/modelcfg` 中以 `map[string][]gen.ModelOpt` 提供，在 `internal/codegen/modelcfg.Overrides` 中配置。配置包只依赖生成工具和表名，使首次模型生成可以独立编译。

在数据库边界保留 NULL；现有 Proto 标量在转换时映射为零值。请求省略字段时保留原值。本仓库对其他数据库的模型生成检查在隔离副本运行，保留仓库提交的 PostgreSQL 模型。

业务应用的 Resource 配置见[公共 API 用法](admin-core-usage.md#生成器配置)。生成器只负责产物，提交前的编译和测试按下节执行。

## 提交前检查

提交前根据修改范围，在对应应用目录运行检查：

| 目录 | 检查命令 |
|---|---|
| `backend/` | `go build ./...`、`go vet ./...`、`go test ./...` |
| `frontend/` | `npm run typecheck`、`npm test`、`npm run build` |

项目贡献流程见 [CONTRIBUTING.md](../CONTRIBUTING.md)。

## CI 数据库测试

CI 根据本次修改的文件选择数据库任务：

| 修改内容 | 自动运行的数据库测试 |
| --- | --- |
| 仅文档或前端 | 跳过数据库测试 |
| 后端应用入口、HTTP 等代码 | PostgreSQL |
| 只改 PostgreSQL 迁移 | PostgreSQL |
| MySQL 专用迁移 | PostgreSQL、MySQL |
| MariaDB 专用迁移 | PostgreSQL、MariaDB |
| SQLite 专用实现或迁移 | PostgreSQL、SQLite |
| 共享存储、`backend/internal/modules/`、共用迁移、协议、生成器或 Go 依赖 | 四种数据库 |
| CI 工作流或数据库选择规则 | 四种数据库 |

额外验证某个数据库时，在 GitHub 的 **Actions → CI → Run workflow** 中选择分支和 `database`：`postgres`、`sqlite`、`mysql`、`mariadb` 或 `all`。单独选择 MySQL、MariaDB 或 SQLite 时，只运行该数据库任务。工作流结果中的 **Select database tests** 会列出本次选择。

在仓库根目录用命令验证当前分支的 MySQL：

```sh
gh workflow run CI --ref "$(git branch --show-current)" -f database=mysql
```

## 发布前端包

`@abysstract/admin-proto` 与 `@abysstract/admin-core` 由维护者在本地发布到 npm，CI 不发布。两个包使用同一版本号，`admin-core` 固定依赖同版本的 `admin-proto`。

1. 在 PR 中同步修改两个包和 `frontend/apps/reference/package.json` 中的版本号，执行 `cd frontend && npm install --package-lock-only` 更新锁文件，并在 `CHANGELOG.md` 记录版本。
2. PR 合并后，在最新的 `main` 上打包并检查：

   ```bash
   python3 scripts/admin_artifacts.py pack --dest /tmp/admin-artifacts
   python3 scripts/admin_artifacts.py verify --manifest /tmp/admin-artifacts/admin-private-artifacts.json
   ```

3. 先发布 `admin-proto`，再发布 `admin-core`：

   ```bash
   cd frontend
   npm publish --dry-run -w @abysstract/admin-proto
   npm publish -w @abysstract/admin-proto
   npm publish --dry-run -w @abysstract/admin-core
   npm publish -w @abysstract/admin-core
   ```

4. 用 `npm view @abysstract/admin-core version` 确认版本，并在 GitHub 上为该提交创建 `vX.Y.Z` 版本发布。
