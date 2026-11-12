# 请求日志

请求日志为每个请求记录一条访问记录：时间、请求 ID、接口、调用方、客户端 IP、结果码、耗时，以及业务自行补充的字段（例如批量接口的成功与失败条数）。后端通过公共包 `backend/accesslog` 采集记录，再交给一个或多个可替换的 **sink** 保存。内置两个 sink：

- `stdout`：每个请求输出一行 JSON，可由 Filebeat、Fluent Bit、Promtail 等采集。
- `db`：写入 `sys_access_log` 表，按保留天数自动清理，后台「系统管理 → 请求日志」页面从这里查询。

请求日志与操作日志（`sys_oper_log`）用途不同：

| | 请求日志 | 操作日志 |
| --- | --- | --- |
| 范围 | 所有 RPC 与 HTTP 请求，包括失败的鉴权 | 已通过鉴权的写操作 |
| 写入 | 内存队列异步批量写入，队列满时丢弃 | 请求内同步写入 |
| 用途 | 排查问题、统计调用量与延迟 | 审计 |

请求只把记录放入有界队列，不等待写入，也不会因为 sink 失败而失败。因此请求日志可能丢失少量记录，不能代替审计日志。

## 1. 使用默认配置

自带后台默认启用 `db,stdout`，不需要额外配置。执行迁移（创建 `sys_access_log` 表和「请求日志」菜单）后启动后端：

```bash
cd backend
go run ./cmd/migrate
go run ./cmd/server
```

在另一个终端登录并调用一个接口：

```bash
TOKEN=$(curl -s http://localhost:8080/abysstract.admin.v1.AuthService/Login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')
curl -s http://localhost:8080/abysstract.admin.v1.UserService/ListUsers \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" -d '{}' > /dev/null
```

后端终端在 1 秒内输出两行 `"type":"access_log"` 的 JSON，第二行类似：

```json
{"type":"access_log","time":"2026-11-10T08:00:00.123456Z","request_id":"3f0c…","kind":"rpc","service":"abysstract.admin.v1.UserService","procedure":"/abysstract.admin.v1.UserService/ListUsers","method":"POST","path":"/abysstract.admin.v1.UserService/ListUsers","principal_type":"user","principal_id":"1","principal_name":"admin","client_ip":"127.0.0.1","user_agent":"curl/8.5.0","code":"ok","status":200,"success":true,"duration_ms":3.214}
```

用 `admin` 登录后台，打开「系统管理 → 请求日志」，可以按时间范围、调用方、接口、结果、错误码和请求 ID 筛选，「详情」中显示业务字段。也可以直接调用页面使用的接口：

```bash
curl -s http://localhost:8080/abysstract.admin.v1.LogService/ListAccessLogs \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" \
  -d '{"procedure":"ListUsers","page":{"pageNum":1,"pageSize":5}}'
```

`/healthz`、`/readyz`、`/metrics`、`/version` 和 CORS 预检请求不记录。

## 2. 配置

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `ADMIN_ACCESS_LOG_SINKS` | sink 名，逗号分隔，按顺序写入。设置为空或 `none` 关闭请求日志 | `db,stdout` |
| `ADMIN_ACCESS_LOG_QUEUE_SIZE` | 待写队列容量，1–1000000。队列满时丢弃新记录并计数 | `4096` |
| `ADMIN_ACCESS_LOG_BATCH_SIZE` | 每批交给 sink 的最大条数，1–10000 | `200` |
| `ADMIN_ACCESS_LOG_FLUSH_INTERVAL_MS` | 不足一批时的最长等待（毫秒），10–60000 | `1000` |
| `ADMIN_ACCESS_LOG_RETENTION_DAYS` | `db` sink 保留天数，0 为永久保留，最大 3650 | `30` |

客户端 IP 与操作日志一致：设置 `ADMIN_TRUST_PROXY=true` 后取 `X-Forwarded-For` 最左侧的地址，否则取直连地址。

配置了未注册的 sink 名时，后端启动失败并列出可用的名称。只配置 `stdout` 时，「请求日志」菜单不显示，`ListAccessLogs` 返回 `failed_precondition`，提示需要包含 `db`。

关闭时，后端先停止接收 HTTP 请求，再把队列中的记录写完，最后关闭数据库。写入时间受 `ADMIN_SHUTDOWN_TIMEOUT_SECONDS` 限制。

## 3. stdout 字段

每行是一个 JSON 对象，字段名保持稳定，可直接作为日志平台的索引字段：

| 字段 | 说明 |
| --- | --- |
| `type` | 固定为 `access_log`，用于和应用日志区分 |
| `time` | 请求开始时间，RFC 3339 UTC |
| `request_id` | 与响应头 `X-Request-Id`、应用日志中的 `request_id` 相同 |
| `kind` | `rpc`（Connect 调用）或 `http`（其他 HTTP 路由） |
| `service`、`procedure` | Connect 服务名与过程，如 `/abysstract.admin.v1.UserService/ListUsers` |
| `method`、`path` | HTTP 方法与路径 |
| `principal_type`、`principal_id`、`principal_name` | 调用方类型（`user`、`api_credential` 或应用自定义）、ID 与名称；匿名请求为空 |
| `client_ip`、`user_agent` | 客户端地址与 `User-Agent` |
| `code` | Connect 错误码（`ok`、`unauthenticated`、`not_found` 等）；普通 HTTP 请求为空 |
| `status` | HTTP 状态码 |
| `success` | Connect 错误码为 `ok`，或普通 HTTP 请求状态码小于 400 |
| `error` | 错误信息，最长 1000 字节；成功时省略 |
| `duration_ms` | 耗时（毫秒，精确到微秒） |
| `attrs` | 业务字段；没有时省略 |

## 4. db sink 与 sys_access_log

`sys_access_log` 由系统迁移创建，支持 PostgreSQL、MySQL、MariaDB 和 SQLite。主要列：

| 列 | 说明 |
| --- | --- |
| `log_time` | 请求开始时间（UTC） |
| `request_id`、`kind`、`service`、`procedure_name`、`http_method`、`path` | 请求标识与接口 |
| `principal_type`、`principal_id`、`principal_name` | 调用方 |
| `client_ip`、`user_agent` | 客户端 |
| `code`、`http_status`、`status`、`error_msg` | 结果；`status` 为 1 成功、2 失败 |
| `duration_us` | 耗时（微秒） |
| `attrs` | 业务字段的 JSON 文本 |

索引覆盖 `log_time`、`(principal_id, log_time)`、`(principal_name, log_time)` 和 `(procedure_name, log_time)`。超出列宽的值按列宽截断；业务字段的 JSON 超过约 60 KB 时只保留 `{"_truncated":true}`。

启动时和之后每小时删除早于保留天数的记录，每次最多删除 5000 行一批，避免长时间锁表。多实例部署时每个实例都会执行清理，删除操作互不冲突。

已有应用升级后，按[配置说明](configuration.md#执行迁移)用 `admin-migrations sync` 导入新的系统迁移，再执行迁移。

## 5. 在业务代码中补充记录

请求进入时，框架为当前请求创建记录并放入 `context`。Handler、Hook 和拦截器用以下函数补充：

```go
import "github.com/abysstract/abysstract-admin/backend/accesslog"

func (s *SyncService) PushItems(ctx context.Context, req *syncv1.PushItemsRequest) (*syncv1.PushItemsResponse, error) {
	ok, failed := s.save(ctx, req.GetItems())
	accesslog.AddAttrs(ctx, map[string]any{"items_ok": ok, "items_failed": failed})
	accesslog.SetAttr(ctx, "batch_id", req.GetBatchId())
	return &syncv1.PushItemsResponse{Ok: int32(ok), Failed: int32(failed)}, nil
}
```

| 函数 | 作用 |
| --- | --- |
| `SetAttr(ctx, key, value)` / `AddAttrs(ctx, map)` | 添加业务字段；值需能编码为 JSON |
| `SetPrincipal(ctx, accesslog.Principal{Type, ID, Name})` | 记录已认证的调用方，由认证代码在校验通过后调用 |
| `SetError(ctx, msg)` | 为没有返回 Connect 错误的 HTTP 处理器记录错误信息 |
| `Skip(ctx)` | 不记录当前请求，例如高频轮询接口 |

没有请求日志时（未启用或在请求之外调用），这些函数什么都不做并返回 `false`，不需要判空。

后台自带的登录与 JWT 鉴权已经设置调用方：类型 `user`，ID 为用户 ID，名称为用户名。登录成功的请求也会记录登录的用户。

## 6. 在独立的 API 服务中复用

应用可以在后台之外运行自己的 Connect 服务，例如供外部系统调用、用 API 凭证认证的同步接口。这类服务使用同一个 `accesslog.Interceptor`，再在自己的认证拦截器中调用 `SetPrincipal`。

### 6.1 与后台在同一进程

用 `adminapp.OpenAccessLog` 打开一个分发器，同时交给后台和自己的服务，两边的记录写入同一个表，在同一个页面查询：

```go
access, err := adminapp.OpenAccessLog(ctx, *cfg, db, logger)
if err != nil {
	log.Fatal(err)
}
defer access.Close(context.Background()) // 在两个服务都停止之后

admin, err := adminapp.New(adminapp.Options{Config: *cfg, DB: db, Modules: modules, AccessLog: access})
// ...

capture := accesslog.CaptureOptions{TrustProxy: cfg.TrustProxy}
api := connect.NewServer(
	accesslog.Interceptor(access, capture), // 放在最前面，认证失败也会记录
	apiKeyInterceptor(keys),
)
syncv1connect.RegisterSyncServiceHandler(api, syncHandler)
mux := http.NewServeMux()
connecthttp.Mount(mux, api)
apiServer := &http.Server{Addr: ":9090", Handler: accesslog.Middleware(access, capture)(mux)}
```

`Options.AccessLog` 由应用传入，后台不会关闭它；`OpenAccessLog` 在 `ADMIN_ACCESS_LOG_SINKS` 为空时返回 `nil`，`nil` 分发器上的所有调用都是空操作。

### 6.2 单独的进程

单独的进程不启动后台，直接打开 sink：

```go
import (
	"github.com/abysstract/abysstract-admin/backend/accesslog"
	_ "github.com/abysstract/abysstract-admin/backend/accesslog/dbsink"
	_ "github.com/abysstract/abysstract-admin/backend/accesslog/stdoutsink"
)

sinks, err := accesslog.Open(ctx, []string{"db", "stdout"}, accesslog.Config{DB: db, Retention: 30 * 24 * time.Hour})
if err != nil {
	log.Fatal(err)
}
access := accesslog.New(accesslog.Options{Sinks: sinks})
defer access.Close(context.Background())
```

也可以继续用 `adminapp.OpenAccessLog(ctx, *cfg, db, logger)` 按 `ADMIN_ACCESS_LOG_*` 配置打开。多个进程写同一个 `sys_access_log` 表时，只需要其中一个启用保留清理（其他进程把 `Retention` 设为 0）。

### 6.3 认证拦截器

```go
func apiKeyInterceptor(keys KeyStore) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			info, _ := connect.CallInfoForServerContext(ctx)
			key, err := keys.Verify(ctx, info.RequestHeader().Get("Authorization"))
			if err != nil {
				return connect.NewError(connect.CodeUnauthenticated, "invalid api key")
			}
			accesslog.SetPrincipal(ctx, accesslog.Principal{
				Type: accesslog.PrincipalAPICredential,
				ID:   key.ID,
				Name: key.Name,
			})
			return next(ctx, spec, stream)
		}
	}
}
```

只使用 `Interceptor`、不包裹 `Middleware` 时也会记录每个调用，此时 `status` 为 0，客户端 IP 取自 Connect 的对端地址。包裹 `Middleware` 后同一个请求只产生一条记录，并带上 HTTP 状态码和非 Connect 路由。`CaptureOptions.RequestID` 可以接入应用自己的请求 ID；未设置时读取 `X-Request-Id` 请求头。

## 7. 自定义 sink

sink 实现两个方法：

```go
type Sink interface {
	Write(ctx context.Context, entries []accesslog.Entry) error
	Close(ctx context.Context) error
}
```

- 每个 sink 由独立的 goroutine 按批调用，不会并发调用同一个 sink 的 `Write`。
- `entries` 和其中的 `Attrs` 由所有 sink 共享，只读，`Write` 返回后不要保留。
- `Write` 返回错误时分发器记录错误并继续处理下一批，不重试；一个 sink 失败、变慢或 panic 不影响其他 sink。某个 sink 积压超过约一个队列的记录时，它会跳过后续批次并计入自己的 `Dropped`。
- 实现 `Name() string` 可以在统计中显示名称。
- 同时实现 `accesslog.Reader`（`Query(ctx, accesslog.Query) (accesslog.Page, error)`）的 sink 可以供「请求日志」页面查询，页面使用第一个实现了 `Reader` 的 sink。

按名称启用：在包的 `init` 中注册，再在 `ADMIN_ACCESS_LOG_SINKS` 中使用该名称，并在 `main` 中导入这个包：

```go
func init() {
	accesslog.Register("kafka", func(ctx context.Context, cfg accesslog.Config) (accesslog.Sink, error) {
		return newKafkaSink(os.Getenv("KAFKA_BROKERS"))
	})
}
```

也可以不注册，直接把实例交给后台，与配置的 sink 一起写入，后台关闭时一并关闭：

```go
adminapp.New(adminapp.Options{Config: *cfg, Modules: modules, AccessLogSinks: []accesslog.Sink{mySink}})
```

`AccessLogSinks` 与 `AccessLog` 不能同时设置；共享分发器时，把自定义 sink 作为 `OpenAccessLog` 的最后几个参数传入。

## 8. 运行统计

`Logger.Stats()` 返回队列长度与容量、已接收、已丢弃、写入失败次数，以及每个 sink 的写入、失败、跳过条数和最近一次错误。后台通过 `LogService.GetAccessLogStatus` 提供同样的数据，「请求日志」页面在有丢弃时显示提示。之后接入 Prometheus 时可以直接把这些计数导出为指标。

## 9. 后续可接入的 sink

以下 sink 尚未提供，可以按第 7 节在应用中实现，接口不需要改变：

- **Prometheus 指标**：按 `procedure`、`code` 统计请求数和耗时直方图，不保存单条记录。
- **Loki**：按批推送到 Loki 的 push API；`kind`、`code` 适合作为标签，`principal_id`、`request_id` 留在日志内容中。在已有 Promtail 的环境中，也可以继续使用 `stdout` sink 由 Promtail 采集。
- **Elasticsearch / OpenSearch**：用 bulk API 按批写入，并实现 `Reader` 让后台页面直接查询。
- **OpenTelemetry**：把记录转为 OTLP 日志记录，`request_id` 对应 trace 关联字段。

## 常见问题

- **启动报 `access log sink "xxx" is not registered`**：检查 `ADMIN_ACCESS_LOG_SINKS` 的拼写；自定义 sink 需要在 `main` 中导入注册它的包。
- **启动报 `table sys_access_log is missing`**：执行迁移；业务应用先用 `admin-migrations sync` 导入新的系统迁移。
- **看不到「请求日志」菜单**：确认 `ADMIN_ACCESS_LOG_SINKS` 包含 `db` 并重启；非超级管理员需要在角色中分配「请求日志」菜单（权限标识 `system:accesslog:list`）。
- **页面提示有记录被丢弃**：请求量超过了写入速度。增大 `ADMIN_ACCESS_LOG_QUEUE_SIZE` 或 `ADMIN_ACCESS_LOG_BATCH_SIZE`，或检查数据库写入延迟。
