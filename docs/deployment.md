# 部署

## 1. 构建应用

以下命令构建自带后台，在仓库根目录执行。将 `VITE_API_BASE` 替换为实际后端地址；前端与后端通过反向代理共用一个域名时，用同源路径（如 `/`），见[同源部署](#同源部署反向代理)：

```bash
repo_root="$(git rev-parse --show-toplevel)"
( cd "$repo_root/backend" && go build -o bin/admin ./cmd/server && go build -o bin/migrate ./cmd/migrate )
( cd "$repo_root/frontend" && npm ci && VITE_API_BASE="https://admin-api.example.com" npm run build )
```

后端产物为 `backend/bin/admin` 和 `backend/bin/migrate`。前端产物为 `frontend/apps/reference/dist/`，由静态服务器或 CDN 托管。

## 2. 配置生产环境

为后端设置 `ADMIN_ENV=prod`，填写数据库连接、JWT 密钥和允许的前端来源。变量含义和要求统一见[生产环境配置](configuration.md#生产环境)。通过反向代理部署时，同时按[客户端 IP 配置](configuration.md#客户端-ip-与审计)设置代理信任。

在反向代理或网关配置 HTTPS，并将前端请求转发至后端监听地址。

### 同源部署（反向代理）

`host.apiBaseUrl` 可以是绝对地址，也可以是以 `/` 开头的同源路径（如 `/` 或 `/api`）。同源路径在页面加载时按 `window.location` 解析，同一份构建产物换任何域名访问都能使用，例如经不同名称访问的评审环境。自带后台的入口在生产构建未设置 `VITE_API_BASE` 时使用 `/`。

反向代理需要把以下路径转发给后端（基础路径为 `/api` 时，在这些路径前加 `/api` 并在转发时去掉该前缀）：

| 路径 | 用途 |
| --- | --- |
| `/abysstract.admin.v1.*` | 系统服务（Connect 协议，`POST`） |
| `/<业务 Proto 包>.*`，如 `/company.product.v1.*` | 每个业务模块的服务，包名见模块 `proto/` 下的 `package` |
| `/static/` | 本地存储驱动的文件下载与直传上传（改了挂载路径时以实际值为准；S3 不经过后端） |
| `/healthz`、`/readyz`、`/version` | 可选：探活与版本 |

其余路径交给静态文件服务，未命中的路径回退到 `index.html`（前端使用浏览器路由，刷新、在新标签页打开详情链接都依赖这条回退）。Nginx 示例：

```nginx
location ~ ^/(abysstract\.admin\.v1|company\.product\.v1)\. { proxy_pass http://127.0.0.1:8080; }
location /static/ { proxy_pass http://127.0.0.1:8080; client_max_body_size 50m; }
location / { root /srv/admin/dist; try_files $uri /index.html; }
```

同源请求不触发跨域，但生产模式仍要求 `ADMIN_CORS_ORIGINS`，填写页面的来源（如 `https://admin.example.com`）。前面有代理时同时设置 `ADMIN_TRUST_PROXY`。直传上传计算文件哈希需要 Web Crypto，只在 HTTPS 或 `localhost` 页面可用。

请求日志默认写入数据库并输出到标准输出，数据库中保留 30 天。日志量大或已有日志平台时，按[请求日志](access-log.md#2-配置)调整 `ADMIN_ACCESS_LOG_SINKS` 和保留天数。

使用默认本地存储时，持久化 `ADMIN_UPLOAD_DIR` 目录，多实例部署共享该目录或改用 S3。使用 S3 时为存储桶配置跨域上传，见[文件存储](storage.md#2-切换到-s3-或兼容服务)。

## 3. 执行迁移并启动

在已经配置环境变量的后端目录执行：

```bash
./bin/migrate
./bin/admin
```

迁移成功后启动服务。后续发布包含数据库结构变更时，先执行新版迁移程序。

## 4. 确认部署结果

访问前端地址，完成登录，检查菜单和列表是否正常加载。新数据库使用初始账号 `admin / admin123` 登录，随后修改密码。已有数据库使用已设置的账号密码。

检查数据保存后刷新页面仍然可见，并确认前端请求使用配置的 HTTPS 后端地址。
