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
| `ADMIN_TRUST_PROXY` | 是否信任反向代理的 `X-Forwarded-For` | `false` |
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

### 上传文件

`/static/` 下的上传文件不校验登录，持有链接的人都可以下载。服务端关闭目录列表，并通过 `X-Content-Type-Options: nosniff` 和 CSP 沙箱阻止上传的 HTML、SVG 在后台域名下执行脚本。PDF 不加沙箱，以便浏览器内置阅读器正常显示。

### 生产环境

设置 `ADMIN_ENV=prod`，并提供以下配置，启动时会检查这些要求：

- `ADMIN_JWT_SECRET`：设置为强随机密钥。
- `ADMIN_DB_DSN`：填写实际部署数据库的连接串。
- `ADMIN_CORS_ORIGINS`：显式列出允许的前端来源。

本地示例密钥、示例 PostgreSQL 连接串和通配 CORS 来源仅用于开发环境。开发模式会为这些配置打印告警。

### 客户端 IP 与审计

操作日志默认记录直连客户端的 IP。通过可信反向代理（如 Nginx）部署时，设置 `ADMIN_TRUST_PROXY=true`，应用将使用 `X-Forwarded-For` 中最左侧的地址作为客户端 IP。
