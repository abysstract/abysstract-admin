# 更新日志（Changelog）

本项目的所有重要变更都会记录在本文件中。

格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本（Semantic Versioning）](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added

- Google 登录：登录页在密码表单下方增加“使用 Google 账号登录”，与账号密码并存。设置 `ADMIN_GOOGLE_CLIENT_ID` 启用，可用 `ADMIN_GOOGLE_ALLOWED_DOMAINS` 限定域名。只有邮箱已在后台用户中的 Google 账号能登录，首次登录时按已验证邮箱匹配用户并把 Google 账号 ID 绑定到 `sys_user_identity`，之后只认账号 ID；不会自动建号。服务端校验 Google 的 ID token，不需要 client secret。见[Google 登录](docs/configuration.md#google-登录)。
- `AuthService` 新增公开方法 `LoginWithGoogle` 与 `GetLoginOptions`（登录页据此判断是否显示 Google 按钮）。`texts.login` 新增 `or`、`googleFailed`、`googleLoadFailed`。
- 数据库迁移：新增 `sys_user_identity`（四种数据库均有）。新增迁移 `20261118090000_google_login`，升级后执行 `admin-migrations sync` 和 `migrate`（见[执行迁移](docs/configuration.md#执行迁移)）。不启用 Google 登录也需要执行，该表是空的。

## [0.7.1] - 2026-11-17

Go 模块与 `@abysstract/admin-core`、`@abysstract/admin-proto` 同步发布 0.7.1。重做登录验证码的界面，内置滑块不再依赖 go-captcha；补充说明 0.7.0 中一项未标明的破坏性变化。没有数据库迁移，Proto 没有变化。

升级时注意：滑块的 `Payload` 格式不变，先升级后端时，0.7.0 前端的内置组件仍能正确显示。在测试里按旧文案（如「向右拖动滑块，使拼图与缺口对齐」「已完成，正在登录」）查找验证码的，需要更新。

### Added

- `slidecaptcha.WithBackgrounds` 与 `slidecaptcha.LoadBackgrounds`：内置滑块可以使用应用自己的背景图（例如 `embed.FS` 中的图片），自己构造 Verifier 后传给 `adminapp.Options.Captcha`。见[登录安全](docs/configuration.md#登录安全)。

### Changed

- 登录验证码重做：
  - 不再把验证码嵌在表单里。需要验证时，点“登录”会先在表单的位置显示“安全验证”面板，宽度与表单一致；验证通过后面板收起，“登录”按钮进入加载并自动提交。面板标题右侧是“换一张”和“关闭”，关闭后回到表单，填过的内容保留。宽屏时面板标题顶替“登录”标题。打开面板时清掉密码字段上一次的错误。
  - 内置滑块的图片和滑轨与输入框等宽，滑轨与大号输入框等高、圆角一致；拖过的部分用主色浅底填充，完成状态不再变绿。提示只在起点显示，不会被滑块挡住。键盘支持方向键（Shift 加速）、Home、End、回车。
  - `captcha/slidecaptcha` 不再依赖 go-captcha 和它的示例图库，默认背景按随机种子即时生成（中性色抽象图形），内置 server 的二进制约小 1 MB；拼图块去掉白色光晕，改为细描边。图片尺寸从 300×220 改为 400×200，`Payload` 字段不变，`@abysstract/admin-core` 0.7.0 的内置组件仍能正确显示。
  - 文案：`slideHint` 改为「向右拖动，补全拼图」，`slideDone` 改为「验证通过」（只供读屏软件朗读），`captchaInvalid` 改为「验证未通过，再试一次」，`captchaLoadFailed` 改为「无法加载验证图片」。`captchaRequired` 不再显示，保留字段以兼容已有的覆盖。新增 `action.close`（「关闭」）；图片加载失败时的按钮改用 `action.retry`（「重试」）。
- 补充说明 0.7.0：**破坏性**：`ResourceSearchBar` 清空关键词后会立即用空字符串调用 `onSearch` 重新查询（原来要再点“查询”）。在 `onSearch` 里把空关键词当作无效输入、提示或忽略的页面需要调整。0.7.0 的更新日志写了这项变化，但没有标为破坏性。

## [0.7.0] - 2026-11-16

Go 模块与 `@abysstract/admin-core`、`@abysstract/admin-proto` 同步发布 0.7.0。新增后台设计规范，自带页面与 `ResourceTable`、`ResourceForm` 按规范整改（多项破坏性默认值，见 Changed）；登录页改版，登录失败后要求验证码；请求日志可以在单独的进程中写入。

升级时注意：

- **安全修复**：`ADMIN_TRUST_PROXY=true` 时客户端 IP 改取 `X-Forwarded-For` 最右侧的地址。多层反向代理的部署需要设置 `ADMIN_TRUSTED_PROXIES`。
- 新增迁移 `20261116090000_login_guard`：先执行 `admin-migrations sync`，再执行 `migrate`。
- 业务页面会受到表格、表单、行操作和提示消息默认行为变化的影响，按 Changed 中标为破坏性的条目检查；依赖默认文案的测试需要更新。

### Added

- 后台设计规范 `frontend/packages/admin-core/DESIGN.md`：布局与间距、字体、颜色、反馈、表单、数据列表、按钮、文案、数据格式、详情与概览页模板、空状态与异常状态。随 `@abysstract/admin-core` 一起发布（`node_modules/@abysstract/admin-core/DESIGN.md`），使用方的项目规范只需写本项目的补充。
- 界面文案集中定义：`defaultAdminTexts`、`useAdminTexts()`，宿主通过 `AdminHost.texts` 覆盖其中一部分。见[界面文案](docs/admin-core-usage.md#界面文案)。
- `ResourceFormDrawer`（`@abysstract/admin-core/resource`）：720 宽的新增、编辑抽屉，「取消 保存」在标题栏右侧。`ResourceForm` 新增 `onCancel`、`actionsContainer`；表单项新增 `span`、`sameRowAsPrevious`。
- 登录页可配置：`AdminHost.login` 的 `logo`、`subtitle`、`background`、`footer`；登录页文案放在 `texts.login`。见[登录页](docs/admin-core-usage.md#登录页)。
- 登录失败后要求验证码，并按来源 IP 计数（IPv6 按 /64）。`ADMIN_CAPTCHA_PROVIDER` 可选内置滑块 `slide`（`captcha/slidecaptcha`，基于 go-captcha，不依赖外部服务）、`turnstile`、`hcaptcha`；其他服务实现 `captcha.Verifier` 后传给 `adminapp.Options.Captcha`。新增 `AuthService.GetCaptcha`、`LoginRequest.captcha_id/captcha` 和错误详情 `LoginChallenge`；前端内置滑块，第三方组件通过 `AdminHost.login.captcha` 提供。未启用验证码时，同一 IP 失败过多会暂时拒绝登录。新增迁移 `20261116090000_login_guard`（`sys_login_ip_attempt`、`sys_captcha_used`），升级后执行 `admin-migrations sync` 和 `migrate`。见[登录安全](docs/configuration.md#登录安全)。
- `ADMIN_TRUSTED_PROXIES`：多层反向代理时列出可信代理网段，客户端 IP 从 `X-Forwarded-For` 右侧跳过这些网段取得。`accesslog.ProxyPolicy` 是对应的公开 API。
- `ResourceTable` 新增 `query.syncUrl`、列的 `numeric`、自定义行操作的 `danger`、自定义工具栏项的 `placement: "selection"`。
- `accesslog.CaptureOptions.AssignRequestID`：没有请求 ID 中间件的服务（如单独进程中的 API 服务）一行配置即可获得请求 ID。合法的客户端 `X-Request-Id`（8–128 个字符，字母、数字和 `-_.:+/=@`）沿用，否则生成 96 位随机 ID；写入响应头、context 和 `request_id`。新增 `RequestIDFromContext`、`WithRequestID`、`ValidRequestID`、`NewRequestID`。见[请求 ID 与跳过的请求](docs/access-log.md#64-请求-id-与跳过的请求)。
- `accesslog.DefaultSkip` 与 `accesslog.SkipPaths`：后台跳过的健康检查、指标和 CORS 预检请求可在自己的服务中复用，并可追加路径或路径前缀（如 `/static/`）。
- `ADMIN_ACCESS_LOG_PURGE` 与 `adminconfig.Config.AccessLogPurgeDisabled`：多个进程写同一个 `sys_access_log` 表时，只写的进程关闭过期清理，sink、队列、批量与保留天数保持不变。见[单独的进程](docs/access-log.md#62-单独的进程)。

### Changed

前端：框架自带页面（用户、角色、菜单、部门、字典、参数、通知、三个日志页、登录、个人中心、首页）按设计规范整改。以下默认行为有变化，业务页面同样受影响：

- **破坏性**：`ResourceTable` 默认每页 20 条（原为 10）；只有一页时隐藏分页器，超过 5 页显示快速跳转。需要旧值时设 `query.defaults.pageSize: 10`。
- **破坏性**：`ResourceTable` 默认把关键词、页码、每页条数和排序写进地址栏参数 `q`、`page`、`size`、`sort`，并在打开页面时读取。同一页面有多个表格，或表格放在抽屉里时，设 `query.syncUrl: false`。
- **破坏性**：标准新增、编辑从 Modal 改为 720 宽抽屉；`ResourceForm` 的按钮从「重置 / 提交」改为「取消 保存」，并放到右侧，去掉「重置」；表单改用栅格，每个字段单独一行；布尔字段从 Switch 改为 Checkbox；长文本自动增高；表单顶部不再显示错误块（`error` 属性保留但不再渲染）。
- **破坏性**：没有权限、或没有可用实现的工具栏和行操作不再显示为禁用按钮，而是隐藏。
- **破坏性**：操作列最多露出两个文字链接，其余收进「⋯」下拉菜单，删除排在最后；行操作按钮改为小号、不换行。
- **破坏性**：标准批量删除从顶部工具栏移到选中行后出现的底部操作条，按钮文案为「删除」；没有批量操作、也没有自定义工具栏项时，不再显示行勾选框。
- **破坏性**：`ResourceTable` 自己提示新增、编辑、删除的结果（「已创建」「已保存」「已删除」），删除失败用 `Modal.error` 说明原因；`onError` 只在列表加载失败时调用。原来在 `onSuccess`、`onError` 或 `reload` 里自己弹消息的页面会重复提示，删掉这些提示即可。
- **破坏性**：默认文案调整：「新建」改为「新增」；删除确认为「删除这条记录？」+「删除后无法恢复。」，按钮「删除 / 取消」；必填提示改为「请填写{标签}」「请选择{标签}」；长度提示改为「最多 N 个字」；`formatErrorMessage` 的默认兜底文案从「操作失败」改为「无法完成操作，请稍后重试」。依赖这些文字的测试需要更新。
- 空状态写清原因：没有数据时「还没有记录」+「新增」，有关键词时「没有符合条件的记录」+「清除筛选」。
- `ResourceSearchBar` 去掉阴影；关键词可清空，删空后立即重新查询（`onSearch` 收到空字符串）；默认占位文字为「关键词」。
- 时间统一显示为 `YYYY-MM-DD HH:mm:ss`，不再随浏览器语言变化；空值显示次要色的 `-`。「停用」改用默认灰色 Tag；菜单类型、通知类型等普通分类不再用彩色 Tag。
- 操作日志、登录日志、请求日志改为与其他列表一致的筛选面板 + 结果面板，关键词和筛选写进地址栏；请求日志点接口名打开详情，详情为两列，次要字段收进「更多信息」。
- 首页、个人中心、分配菜单加载失败时显示原因和「重试」，不再显示 0 或空白；首页统计卡去掉装饰图标。
- 登录页改为左右分栏：宽屏左侧是品牌区，默认背景是跟随主题 token 的浅色色块；右侧表单的标签放在输入框上方；窄屏时背景铺满整页，产品名和表单放在卡片里，不再出现横向滚动。用户名或密码不对时，错误标在密码字段下方，不再误报「登录已过期」；账号锁定提示改为「账号已锁定，请 N 分钟后再试」。输入框不再有占位文字，按占位文字查找登录框的测试改用标签。
- `module new` 生成的列表页不再自己弹成功、失败消息，也不再覆盖批量删除的文案。

后端与部署：

- **安全修复**：`ADMIN_TRUST_PROXY=true` 时客户端 IP 改取 `X-Forwarded-For` **最右侧**的地址（原为最左侧，客户端可以伪造）。操作日志、请求日志、登录日志和 `accesslog.ClientIP` 都受影响。多层代理的部署需要设置 `ADMIN_TRUSTED_PROXIES`，否则记录到的是上一层代理的地址。自定义 HTTP 路由的操作日志记录 IPv6 客户端地址时不再被截断。
- `ADMIN_LOGIN_MAX_FAILURES` 不设置时自动取值：未启用验证码为 5（与原默认相同），启用验证码为 20。`adminconfig.Config.LoginMaxFailures` 为 0 表示自动，用 `LoginMaxFailuresFor` 取实际阈值。`.env.example` 不再写死 5。
- 未设置 `CaptureOptions.RequestID` 时，`request_id` 先取 context 中的请求 ID，再取 `X-Request-Id` 请求头；取自请求头的值截断到 128 个字符。
- `adminapp.AccessLogCapture` 改用 `accesslog.DefaultSkip`，跳过的请求不变。
- 「系统管理」的排序从 1 改为 1000，排到侧栏最后，1000 以下留给业务模块的一级菜单。登录后默认进入第一个有权限的页面，装了业务模块时会落在业务页而不是「用户管理」。只影响新建的库；已有的库需要时手动执行 `UPDATE sys_menu SET order_num = 1000 WHERE id = 1 AND parent_id = 0 AND order_num = 1;`。

## [0.6.0] - 2026-11-12

Go 模块与 `@abysstract/admin-core`、`@abysstract/admin-proto` 同步发布 0.6.0。前端迁移到 antd 6、pro-components 3 与 React 19（破坏性，见 Changed）；新增 `ResourceSearchBar` 与 `AdminHost.configProvider`。后端与数据库没有变化。

### Added

- `ResourceSearchBar`（`@abysstract/admin-core/resource`）：`ResourceTable` 顶部的搜索卡片，手写列表页可直接使用；支持回车查询、可选的刷新按钮和额外筛选控件。见[自定义列表页的搜索栏](docs/admin-core-usage.md#自定义列表页的搜索栏)。
- `AdminHost.configProvider`：透传给根 `ConfigProvider` 的其余全局配置（`button`、`form`、`componentSize` 等）。见[全局 antd 配置](docs/admin-core-usage.md#全局-antd-配置)。

### Changed

- **破坏性**：`@abysstract/admin-core` 迁移到 antd 6。peer 依赖改为 `antd ^6.7.0`、`@ant-design/icons ^6.3.4`、`@ant-design/pro-components 3.1.15-9`（pro-components 3 尚无正式版，精确锁定）；React 放宽为 `^18.3.0 || ^19.0.0`，自带应用改用 React 19。前端开发与 CI 需要 Node.js 22.12+（pro-components 3 的要求）；仓库根目录新增 `.node-version`，CI 改为读取该文件。
- 业务应用升级时同时升级 antd、icons 和 pro-components，并按 [antd v6 迁移指南](https://ant.design/docs/react/migration-v6-cn) 处理自己页面中的弃用 API；pro-components 3 中 `ProColumns.hideInSearch` 改为 `search: false`。antd 6 默认启用 CSS 变量，自定义主题和依赖内部 DOM 的样式需要复查。
- admin-core 内部改用 antd 6 的新写法（Drawer `size`、Space `orientation`、Alert `title`、Modal `mask.closable`、Descriptions `items`），公共组件的返回类型由全局 `JSX.Element` 改为 `ReactElement`。
- Resource 页面的搜索卡片和表格卡片、登录页与加载页背景改用 antd token（原为固定的 `#fff`、`#f0f0f0`、`#f0f2f5`），跟随应用主题和暗色算法。
- 「不在两个汉字的按钮间插空格」改为在根 `ConfigProvider` 上默认设置，作用于整个后台（包括业务页面和 antd 默认的「取消」「确定」），宿主可通过 `host.configProvider.button.autoInsertSpace` 改回；admin-core 不再在单个按钮上设置 `autoInsertSpace`。

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

[Unreleased]: https://github.com/abysstract/abysstract-admin/compare/v0.7.1...HEAD
[0.7.1]: https://github.com/abysstract/abysstract-admin/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/abysstract/abysstract-admin/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/abysstract/abysstract-admin/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/abysstract/abysstract-admin/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/abysstract/abysstract-admin/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/abysstract/abysstract-admin/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/abysstract/abysstract-admin/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/abysstract/abysstract-admin/releases/tag/v0.1.0
