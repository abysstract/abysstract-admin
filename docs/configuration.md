# 配置说明

后端从环境变量读取配置。本地开发在应用根目录保存 `.env`。README 使用框架仓库根目录；业务项目使用自己的根目录。命令从当前目录和上一级目录读取 `.env`，进程环境变量优先。

## 数据库配置

`ADMIN_DB_DRIVER` 和 `ADMIN_DB_DSN` 必须同时设置。应用要求以下最低版本，并接受更高版本：

| 数据库 | 最低版本 | `ADMIN_DB_DRIVER` | 使用要求 |
| --- | --- | --- | --- |
| PostgreSQL | 13 | `postgres` | 应用专用数据库 |
| MySQL | 8.0.16 | `mysql` | InnoDB |
| MariaDB | 10.5 | `mariadb` | InnoDB |
| SQLite | 3.35.0 | `sqlite` | 单机 Unix 本地文件；构建启用 CGO 并安装 C 编译器 |

已有数据库服务时，创建应用专用数据库后填写连接信息。需要启动本地数据库时，按[本地数据库准备](database-setup.md)操作。业务接入教程沿用 README 中已配置的数据库。

### 连接示例

在 `.env` 中选择一组配置，并替换为实际连接信息：

```dotenv
# PostgreSQL
ADMIN_DB_DRIVER=postgres
ADMIN_DB_DSN=host=localhost port=5433 user=admin password=admin dbname=abysstract_admin sslmode=disable
```

```dotenv
# MySQL
ADMIN_DB_DRIVER=mysql
ADMIN_DB_DSN=app:password@tcp(127.0.0.1:3306)/admin_app
```

```dotenv
# MariaDB
ADMIN_DB_DRIVER=mariadb
ADMIN_DB_DSN=app:password@tcp(127.0.0.1:3306)/admin_app
```

MySQL 和 MariaDB 使用相同的连接串格式，通过 `ADMIN_DB_DRIVER` 区分产品。应用自动配置时间解析和 UTC 时区。

### SQLite 本机文件

SQLite 随 Go 驱动编译进应用。构建时设置 `CGO_ENABLED=1`，并安装 C 编译器：macOS 可安装 Xcode Command Line Tools，Linux 可安装 GCC。

先创建数据库目录，再使用绝对文件路径配置连接：

```dotenv
ADMIN_DB_DRIVER=sqlite
ADMIN_DB_DSN=file:/absolute/path/admin-data/admin.sqlite
```

应用首次连接时创建数据库文件。SQLite 适合单机本地部署，写入按事务串行执行。备份时先停止应用，并将数据库文件及其 WAL 侧文件一起保存。

### 执行迁移

首次启动及更新数据库结构后，在应用的后端目录执行：

```sh
go run ./cmd/migrate
go run ./cmd/server
```

迁移和启动命令都在应用的 `backend/` 中执行，读取应用根目录的 `.env`。迁移会记录已执行版本，后续运行只应用新增版本。

业务应用升级 Core 依赖后，先将新增的系统迁移同步到本地，再审阅并执行迁移。以下命令以 PostgreSQL 为例，其他数据库使用对应的 `ADMIN_DB_DRIVER`：

```sh
ADMIN_DB_DRIVER=postgres go run github.com/abysstract/abysstract-admin/backend/cmd/admin-migrations sync --dir migrations
```

迁移文件保存在应用的 `migrations/<driver>/` 目录。新增业务表和生成代码的步骤见[开发说明](development.md)。

## 服务配置

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `ADMIN_ENV` | 运行模式：`dev` / `prod` | `dev` |
| `ADMIN_ADDR` | 监听地址 | `:8080` |
| `ADMIN_JWT_SECRET` | JWT 签名密钥 | `change-me-in-production` |
| `ADMIN_CORS_ORIGINS` | 允许的跨域来源，逗号分隔 | 空；开发模式使用通配来源 |
| `ADMIN_TRUST_PROXY` | 前面有一层反向代理：客户端 IP 取 `X-Forwarded-For` 最右侧的地址 | `false` |
| `ADMIN_TRUSTED_PROXIES` | 可信代理网段，逗号分隔；多层代理时使用，见[客户端 IP 与审计](#客户端-ip-与审计) | 空 |
| `ADMIN_CAPTCHA_PROVIDER` | 登录验证码：`slide` / `turnstile` / `hcaptcha`，见[登录安全](#登录安全) | 空，不启用 |
| `ADMIN_METRICS_TOKEN` | `/metrics` 访问令牌。设置后请求需带 `Authorization: Bearer <token>`；未设置时开发模式开放，生产模式返回 404 | 空 |

其他配置项及默认值见仓库根目录的 [.env.example](../.env.example)。

### 监控指标

生产模式下，未设置 `ADMIN_METRICS_TOKEN` 时 `/metrics` 返回 404。从旧版本升级且已经用 Prometheus 采集指标的部署，需要设置令牌，并在采集配置中携带：

```yaml
scrape_configs:
  - job_name: abysstract-admin
    metrics_path: /metrics
    authorization:
      type: Bearer
      credentials_file: /etc/prometheus/admin-metrics-token
    static_configs:
      - targets: ["admin.example.com:8080"]
```

`credentials_file` 的内容与 `ADMIN_METRICS_TOKEN` 相同。

### 文件存储

上传文件默认保存在 `ADMIN_UPLOAD_DIR`，由后端在 `/static/` 下提供下载。设置 `ADMIN_STORAGE_DRIVER=s3` 可改用 S3 或 S3 兼容服务。驱动、预签名直传和全部 `ADMIN_STORAGE_*` 变量见[文件存储](storage.md)。

`/static/` 下的上传文件不校验登录，持有链接的人都可以下载。服务端关闭目录列表，并通过 `X-Content-Type-Options: nosniff` 和 CSP 沙箱阻止上传的 HTML、SVG 在后台域名下执行脚本。PDF 不加沙箱，以便浏览器内置阅读器正常显示。

### 请求日志

后端为每个请求记录一条请求日志，默认同时写入 `sys_access_log` 表（后台「系统管理 → 请求日志」）和标准输出（每行一个 JSON）。`ADMIN_ACCESS_LOG_SINKS` 选择 sink，设置为空或 `none` 关闭；`db` sink 默认保留 30 天，`ADMIN_ACCESS_LOG_PURGE=false` 让本进程只写不清理（多个进程写同一个表时使用）。队列、批量和保留相关的全部 `ADMIN_ACCESS_LOG_*` 变量见[请求日志](access-log.md#2-配置)。

### 生产环境

设置 `ADMIN_ENV=prod`，并提供以下配置，启动时会检查这些要求：

- `ADMIN_JWT_SECRET`：设置为强随机密钥。
- `ADMIN_DB_DSN`：填写实际部署数据库的连接串。
- `ADMIN_CORS_ORIGINS`：显式列出允许的前端来源。
- 使用 `s3` 存储驱动时，`ADMIN_STORAGE_S3_BUCKET` 和 `ADMIN_STORAGE_S3_REGION` 必填（开发模式同样检查）。
- `ADMIN_ACCESS_LOG_SINKS` 中的每个名称都必须是已注册的 sink（开发模式同样检查）。

本地示例密钥、示例 PostgreSQL 连接串和通配 CORS 来源仅用于开发环境。开发模式会为这些配置打印告警。

### 客户端 IP 与审计

操作日志、请求日志、登录日志和登录的按 IP 计数使用同一个客户端 IP，默认是直连对端的地址。

代理会把它收到请求的来源地址追加到 `X-Forwarded-For` 的**最右侧**，左侧的内容来自客户端，可以伪造。所以只读可信代理追加的部分：

- 前面只有一层代理（例如同机或内网的 Nginx）：设置 `ADMIN_TRUST_PROXY=true`，取最右侧的地址。
- 多层代理（例如 CDN → Nginx → 应用）：在 `ADMIN_TRUSTED_PROXIES` 列出所有代理的网段，例如 `10.0.0.0/8,203.0.113.0/24`。直连对端不在其中时忽略 `X-Forwarded-For`；在其中时从右向左跳过这些网段，取第一个不在其中的地址。

只设置 `ADMIN_TRUST_PROXY=true` 却有多层代理时，记录到的会是上一层代理的地址，按 IP 计数也会把所有用户算在一起。

## 登录安全

登录失败按两种方式计数：

| 计数 | 范围 | 作用 |
| --- | --- | --- |
| 用户名 | 连续失败，登录成功后清零 | 达到 `ADMIN_LOGIN_MAX_FAILURES` 锁定账号 `ADMIN_LOGIN_LOCKOUT_SECONDS` 秒 |
| 来源 IP | `ADMIN_LOGIN_IP_WINDOW_SECONDS` 窗口内的失败，登录成功**不**清零；IPv6 按 /64 计数 | 见下文 |

**未启用验证码**时，同一 IP 在窗口内失败 `ADMIN_LOGIN_IP_MAX_FAILURES` 次（默认 30）后，窗口剩余时间内来自它的登录直接拒绝，提示“尝试次数过多”。账号锁定阈值默认 5。

**启用验证码**（`ADMIN_CAPTCHA_PROVIDER`）后：

- 同一用户名连续失败 `ADMIN_LOGIN_CAPTCHA_AFTER` 次（默认 3），或同一 IP 在窗口内失败 `ADMIN_LOGIN_IP_CAPTCHA_AFTER` 次（默认 10），之后的登录要先通过验证码。
- 账号锁定阈值默认提高到 20。知道用户名的人仍可能让账号被锁，但每次都要先过验证码。显式设置 `ADMIN_LOGIN_MAX_FAILURES` 时以设置为准。
- 缺少验证码或验证码错误的请求不检查密码，也不计入失败次数。
- 验证服务不可用时拒绝登录，不会放行。

| `ADMIN_CAPTCHA_PROVIDER` | 说明 | 其他配置 |
| --- | --- | --- |
| `slide` | 内置滑块拼图（基于 [go-captcha](https://github.com/wenlng/go-captcha)），不依赖外部服务，内网可用。前端无需配置 | 无 |
| `turnstile` | Cloudflare Turnstile | `ADMIN_CAPTCHA_SECRET`、`ADMIN_CAPTCHA_SITE_KEY`；前端在 `AdminHost.login.captcha` 渲染对方组件，见[登录页](admin-core-usage.md#登录验证码) |
| `hcaptcha` | hCaptcha | 同上 |

`slide` 需要应用导入 `github.com/abysstract/abysstract-admin/backend/captcha/slidecaptcha`（内置的 `cmd/server` 已导入）。图片会打包进二进制，约几 MB；不用滑块的应用去掉这个导入即可。

接其他服务（例如极验）时，实现 `captcha.Verifier`，需要后端出题时再实现 `captcha.Challenger`，然后传给 `adminapp.Options.Captcha`，或用 `captcha.Register` 注册后通过 `ADMIN_CAPTCHA_PROVIDER` 选择：

```go
type geetest struct{ /* ... */ }

// token 是前端组件给出的结果，格式由你的前端组件决定，例如 JSON。
func (g *geetest) Verify(ctx context.Context, sub captcha.Submission) error {
	ok, err := g.validate(ctx, sub.Token, sub.RemoteIP)
	if err != nil {
		return err // 服务不可用：登录返回 Unavailable
	}
	if !ok {
		return captcha.ErrInvalid
	}
	return nil
}

app, err := adminapp.New(adminapp.Options{Config: cfg, Modules: modules, Captcha: &geetest{}})
```

后端出题的验证码，答案经 `GetCaptcha` 加密后放在挑战 ID 里交给前端，服务端不保存；每个挑战只能用一次，有效期 2 分钟，已使用的 ID 记在 `sys_captcha_used`。
