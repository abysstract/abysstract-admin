# 更新日志（Changelog）

本项目的所有重要变更都会记录在本文件中。

格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本（Semantic Versioning）](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.2.0] - 2026-11-07

首个可通过 `go get` 使用的 Go 模块版本；npm 包没有变化，仍为 0.1.0。

### Changed

- **破坏性变更**：Go 模块路径由 `github.com/abysstract/abysstract-admin/backend` 改为 `github.com/abysstract/abysstract-admin`，`go.mod` 移到仓库根目录。包的导入路径（如 `github.com/abysstract/abysstract-admin/backend/resource`）不变，业务应用只需把 `go.mod` 中的 `require`（以及本地联调用的 `replace`）改成新模块路径，并删除旧的 `.../backend` 依赖，否则会出现 ambiguous import。Go 模块与 npm 包从此共用同一个 `vX.Y.Z` tag，Go 模块的首个版本是 `v0.2.0`。`go.mod` 撤回（retract）`v0.1.0`：该 tag 只发布了 npm 包，其中没有可用的 Go 包。
- `module` 命令和生成、检查脚本同时识别仓库根目录的 `go.mod` 与 `backend/go.mod`，沿用旧目录结构的业务应用无需改动即可继续使用。
- 文档：npm 安装命令为 peer 依赖固定大版本，避免装到 React 19 时安装失败。

## [0.1.0] - 2026-11-06

首个公开预览版。

### Added

- Go + ConnectRPC 后端、React + Ant Design Pro 前端，Proto 统一定义 API；动态菜单、RBAC 权限与系统模块（用户/角色/菜单/部门/字典/参数/通知/日志）。
- 数据库迁移与内置管理员初始化（幂等），支持 PostgreSQL、MySQL、MariaDB 与 SQLite。
- 模块生命周期命令（`go run ./cmd/admin module ...`）与标准 JSON CRUD 生成。
- 前端公共包 `@abysstract/admin-proto` 与 `@abysstract/admin-core` 发布到 npm。
- 开源发布材料：LICENSE（Apache-2.0）、CONTRIBUTING、SECURITY、Issue/PR 模板。
- 标准 JSON CRUD 支持 MySQL/MariaDB `BIGINT UNSIGNED AUTO_INCREMENT` 主键和无符号外键列，Model、Proto、Resource 与前端统一使用 `int64`，取值上限为 2^63−1。
- 模块表前缀冲突检查：`module new`、`check`、`sync`、`update`、`generate` 与 `modelgen` 对相同或互为前缀的 `tablePrefix`（如 `shop_` 与 `shop_order_`）报错并指出冲突模块；`tablePrefix` 须为 snake_case。
- `resource.ErrOutOfRange`：MySQL/MariaDB 越界写入（如向无符号列写入负数）在生成的服务中返回 `invalid_argument`。

### Changed

- MySQL/MariaDB `INT UNSIGNED` 列的模型类型由 `int32` 改为 `int64`，`SMALLINT UNSIGNED` 由 `int16` 改为 `int32`，避免大于有符号上限的值读取失败。已生成的资源若暴露这类列，`module generate` 会报告类型变化；可在模块 `codegen/modelcfg` 中用 `gen.FieldType` 保留原类型。PostgreSQL 与 SQLite 不受影响。

### Fixed

- `modelgen` 按 `<前缀>_` 边界选择模块表，`tablePrefix` 不带下划线时不再选中其他模块的表。

[Unreleased]: https://github.com/abysstract/abysstract-admin/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/abysstract/abysstract-admin/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/abysstract/abysstract-admin/releases/tag/v0.1.0
