# 部署

## 1. 构建应用

以下命令构建自带后台，在仓库根目录执行。将 `VITE_API_BASE` 替换为实际后端地址：

```bash
repo_root="$(git rev-parse --show-toplevel)"
( cd "$repo_root/backend" && go build -o bin/admin ./cmd/server && go build -o bin/migrate ./cmd/migrate )
( cd "$repo_root/frontend" && npm ci && VITE_API_BASE="https://admin-api.example.com" npm run build )
```

后端产物为 `backend/bin/admin` 和 `backend/bin/migrate`。前端产物为 `frontend/apps/reference/dist/`，由静态服务器或 CDN 托管。

## 2. 配置生产环境

为后端设置 `ADMIN_ENV=prod`，填写数据库连接、JWT 密钥和允许的前端来源。变量含义和要求统一见[生产环境配置](configuration.md#生产环境)。通过反向代理部署时，同时按[客户端 IP 配置](configuration.md#客户端-ip-与审计)设置代理信任。

在反向代理或网关配置 HTTPS，并将前端请求转发至后端监听地址。

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
