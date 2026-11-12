# 更新日志（Changelog）

本项目的所有重要变更都会记录在本文件中。

格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本（Semantic Versioning）](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Changed

- **破坏性**：`@abysstract/admin-core` 迁移到 antd 6。peer 依赖改为 `antd ^6.7.0`、`@ant-design/icons ^6.3.4`、`@ant-design/pro-components 3.1.15-9`（pro-components 3 尚无正式版，精确锁定）；React 放宽为 `^18.3.0 || ^19.0.0`，自带应用改用 React 19。前端开发与 CI 需要 Node.js 22.12+（pro-components 3 的要求）。
- 业务应用升级时同时升级 antd、icons 和 pro-components，并按 [antd v6 迁移指南](https://ant.design/docs/react/migration-v6-cn) 处理自己页面中的弃用 API；pro-components 3 中 `ProColumns.hideInSearch` 改为 `search: false`。antd 6 默认启用 CSS 变量，自定义主题和依赖内部 DOM 的样式需要复查。
- admin-core 内部改用 antd 6 的新写法（Drawer `size`、Space `orientation`、Alert `title`、Modal `mask.closable`、Descriptions `items`），公共组件的返回类型由全局 `JSX.Element` 改为 `ReactElement`。

## [0.5.0] - 2026-11-12

Go 模块与 `@abysstract/admin-core`、`@abysstract/admin-proto` 同步发布 0.5.0。新增可替换的请求日志。执行迁移前，业务应用先用 `admin-migrations sync` 导入新的系统迁移（`sys_access_log` 表与「请求日志」菜单）。

### Added

- 请求日志：公共包 `backend/accesslog` 为每个请求记录时间、请求 ID、接口、调用方、客户端 IP、Connect 错误码或 HTTP 状态、错误信息、耗时和业务字段 `Attrs`。请求只把记录放入有界队列，由后台批量写入；队列满时丢弃并计数，不阻塞也不影响请求。见[请求日志](docs/access-log.md)。
- 可替换的 sink：`accesslog.Sink`（`Write`、`Close`）与可选的 `accesslog.Reader`，通过 `accesslog.Register` 按名称注册。多个 sink 各自独立写入，一个失败、变慢或 panic 不影响其他 sink。内置 `accesslog/stdoutsink`（`stdout`，每行一个字段稳定的 JSON）和 `accesslog/dbsink`（`db`，写入 `sys_access_log`，按保留天数定期清理，实现 `Reader`）。
- 采集：`accesslog.Middleware` 与 `accesslog.Interceptor`，以及 `SetPrincipal`、`SetAttr`、`AddAttrs`、`SetError`、`Skip` 等上下文函数。应用自己的 Connect 服务（例如用 API 凭证认证的同步接口）可以直接使用同一拦截器并在认证中设置调用方。
- 系统迁移 `sys_access_log`（PostgreSQL、MySQL、MariaDB、SQLite），索引覆盖时间、调用方和接口；「系统管理 → 请求日志」菜单与权限 `system:accesslog:list`。
- `LogService.ListAccessLogs`（时间范围、调用方、接口、结果、错误码、请求 ID 筛选）与 `LogService.GetAccessLogStatus`（sink 列表、队列与丢弃统计）；前端「请求日志」页面，详情中显示业务字段。没有可查询的 sink 时菜单隐藏，接口返回 `failed_precondition`。
- 配置项 `ADMIN_ACCESS_LOG_SINKS`（默认 `db,stdout`，空或 `none` 关闭）、`ADMIN_ACCESS_LOG_QUEUE_SIZE`、`ADMIN_ACCESS_LOG_BATCH_SIZE`、`ADMIN_ACCESS_LOG_FLUSH_INTERVAL_MS`、`ADMIN_ACCESS_LOG_RETENTION_DAYS`（默认 30），启动时校验 sink 是否已注册。
- `adminapp.Options.AccessLog`（共享应用打开的分发器）与 `adminapp.Options.AccessLogSinks`（追加应用自己的 sink）；`adminapp.OpenAccessLog`、`adminapp.AccessLogCapture`、`App.AccessLog()` 与 `module.Runtime.AccessLog`。

### Changed

- 自带后台默认启用请求日志，写入 `sys_access_log` 并输出到标准输出。关闭时在关闭数据库前写完队列中的记录。启用 `db` sink 但数据库缺少 `sys_access_log` 时启动失败并提示执行迁移；暂不迁移的部署可设置 `ADMIN_ACCESS_LOG_SINKS=stdout`。
- `adminconfig.Load` 在未设置 `ADMIN_ACCESS_LOG_SINKS` 时使用 `db,stdout`；直接构造的 `adminconfig.Config` 默认不启用请求日志。

## [0.4.0] - 2026-11-09

Go 模块与 `@abysstract/admin-core`、`@abysstract/admin-proto` 同步发布 0.4.0。新增可替换的文件存储与前端直传；直接调用系统模块 `Create` 的代码需要提供 `module.Runtime.Storage`（见 Changed）。

### Added

- 可替换的文件存储：公共包 `backend/storage` 定义 `storage.Storage`（`Put`、`Stat`、`PresignPut`、`URL`）、`storage.ErrNotFound` 和对象键工具（`ValidateKey`、`JoinKey`、`RandomKey`）。业务保存对象键，输出时用 `URL(key)` 解析访问地址。见[文件存储](docs/storage.md)。
- 存储驱动：`storage/localstorage`（默认，本地目录，`/static/` 下提供下载并接收 HMAC 签名的 `PUT` 直传）与 `storage/s3storage`（aws-sdk-go-v2，支持 endpoint 覆盖、路径风格寻址和公开访问前缀，适配 MinIO、R2；只有导入该包的应用才会编译 AWS SDK）。驱动通过 `storage.Register` 注册，应用可接入自己的实现。
- 配置项 `ADMIN_STORAGE_DRIVER`、`ADMIN_STORAGE_PUBLIC_BASE_URL`、`ADMIN_STORAGE_UPLOAD_TTL_SECONDS`、`ADMIN_STORAGE_SIGNING_SECRET` 与 `ADMIN_STORAGE_S3_BUCKET` / `_REGION` / `_ENDPOINT` / `_PATH_STYLE`，启动时校验。
- `adminapp.Options.Storage` 注入应用自己的存储实例，`module.Runtime.Storage` 把它交给所有模块；`adminapp.OpenStorage` / `adminapp.StorageConfig` 按配置打开同一驱动，`App.Storage()` 返回运行中的实例。
- `UploadService.CreateUpload`：按 `UploadFile` 相同的大小上限与扩展名白名单签发预签名上传，返回对象键、访问 URL、上传地址、请求头和过期时间。`UploadFileResponse`、`UploadAvatarResponse` 新增 `key` 字段。
- 业务自建预签名接口的辅助函数：`storage.UploadPolicy`、`storage.Presign`、`storage.VerifyUpload`、`storage.ContentTypeByExt`、`storage.ImageExts` / `FileExts`。
- 前端 `@abysstract/admin-core/upload`：`useDirectUpload`（申请地址 → 直传 → 返回 key 与 URL，支持进度与取消、自定义 `presign`）和图片表单项 `DirectImageUpload`（表单值为对象键）。

### Changed

- `UploadFile` / `UploadAvatar` 改为写入 `storage.Storage`；新文件的对象键为 `files/<年>/<月>/<随机名>` 与 `avatars/<年>/<月>/<随机名>`，本地驱动下访问地址仍以 `/static/` 开头，已上传的文件不受影响。头像仍把访问 URL 写入 `sys_user.avatar`。
- 系统模块要求 `module.Runtime.Storage`，未提供时 `Create` 返回 `system.ErrMissingStorage`。通过 `adminapp` 启动的应用不受影响；直接调用系统模块 `Create` 的测试需要传入存储。
- `/static/` 只在存储驱动由后端自行提供文件时挂载（本地驱动）。框架保留 `GET /static/` 与 `PUT /static/` 路由；使用本地驱动时 CORS 允许 `PUT`。
- 自带后台（`backend/internal/app`）导入 `storage/s3storage`，可直接使用 `ADMIN_STORAGE_DRIVER=s3`。

## [0.3.0] - 2026-11-07

Go 模块与 `@abysstract/admin-core`、`@abysstract/admin-proto` 同步发布 0.3.0。

### Added

- 文档：[新建独立应用](docs/standalone-app.md)，按任务顺序在自己的仓库和 Go 模块中搭建后台、安装 module 命令并开发第一个模块。
- `admin.modules.json` 支持 `frontendRoot`（前端源码目录，默认 `frontend/apps/reference/src`），并可用 `"system"` 选择框架的系统模块。

### Changed

- `module` 命令可在框架仓库之外使用（`go get -tool github.com/abysstract/abysstract-admin/backend/cmd/admin` 后运行 `go tool admin module ...`）：生成的 Go 代码按应用 `go.mod` 的模块路径导入应用自己的包，框架公共包仍从 `github.com/abysstract/abysstract-admin/backend/` 导入。
- `module generate` 不再调用 `go run ./cmd/modelgen` 和 `node frontend/tools/generate-proto.mjs`：Model/query 在命令进程内生成，协议代码直接调用 `buf`，框架公共 Proto 从依赖的框架模块读取，Go 协议包路径按应用模块路径设置。
- `module new`、`generate`、`sync` 和 `update` 按 `module.json` 中声明的路径读写模块代码、协议、页面和 Resource 生成配置，不再写死 `frontend/apps/reference/src`。
- 模块菜单迁移和 `go_registry.gen.go` 只覆盖 `backend/migrations/` 下已存在的方言目录；迁移目录中不带版本号的 Go 文件（如 `doc.go`）不再被当作迁移。`sync` 只在框架仓库中维护 `buf.yaml` 和 `backend/internal/codegen/` 生成清单。
- 生成的 Resource 服务不再把驱动原始信息（库名、表名、约束定义）返回给客户端：未归类的错误返回 `internal`，消息为“服务内部错误”，原始错误写入服务日志。
- `resource.NormalizeDBError` 新增归类：外键约束为 `resource.ErrForeignKey`（`failed_precondition`），CHECK 约束为 `resource.ErrCheckViolation`（`invalid_argument`），触发器以 MySQL/MariaDB `SIGNAL SQLSTATE '45xxx'` 或 PostgreSQL `RAISE EXCEPTION` 拒绝的写入为 `*resource.RejectedError`（`failed_precondition`，消息为数据库中的 `MESSAGE_TEXT`）。现有应用重新执行 Resource 生成后生效。

### Fixed

- 没有自定义操作的 JSON 资源生成的 `definition/<resource>.gen.go` 不再写出单参数的 `append`，`go vet` 不再报 `append with no values`。
- 标准创建表单不再给可空字段（创建请求中的 Proto `optional`）填零值：可选外键等字段默认留空并从创建请求中省略。生成的 TS schema 为这类字段加上 `optional: true`，需重新生成 Resource。

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

[Unreleased]: https://github.com/abysstract/abysstract-admin/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/abysstract/abysstract-admin/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/abysstract/abysstract-admin/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/abysstract/abysstract-admin/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/abysstract/abysstract-admin/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/abysstract/abysstract-admin/releases/tag/v0.1.0
