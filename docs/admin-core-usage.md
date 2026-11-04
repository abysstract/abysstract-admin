# 公共 API 用法

本页说明业务应用使用的公共包、生成器配置和前端接入方式。首次接入请先完成[模块教程](new-module-tutorial.md)。

## 公共类型

Go 应用按需导入 `github.com/abysstract/abysstract-admin/backend/` 下的公共包：

| 包 | 用途 |
|---|---|
| `adminapp`、`adminconfig` | 创建应用和加载配置 |
| `module`、`system` | 声明业务模块并组合系统功能 |
| `permission`、`identity` | 注册权限、读取当前用户 |
| `resource`、`resource/schema` | 使用事务、回调和 Resource 字段声明 |
| `resourcegen` | 生成业务 Resource 代码 |
| `coremigrations` | 提供系统迁移来源 |

前端从 `@abysstract/admin-core` 导入应用与组件，从 `@abysstract/admin-proto` 导入系统协议类型。包的安装方式见下文[安装前端包](#安装前端包)。

## Go RPC 接口

Go 后端要求 Go 1.26+，使用 `connectrpc.com/connect/v2`。服务方法直接接收和返回 Proto 消息：

```go
func (s *Service) GetProduct(ctx context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error)
```

模块通过 `ConnectService.Register` 注册服务，见[模块开发指南](module-guide.md#后端服务与前端页面)。

Go 客户端通过 HTTP transport 调用服务。下面使用商品生成包中的 `productv1` 和 `productv1connect`；`connecthttp` 来自 `connectrpc.com/connect/v2/connecthttp`：

```go
client := productv1connect.NewProductServiceClient(
    connect.NewClient(connecthttp.NewTransport(httpClient, baseURL)),
)
ctx, info := connect.NewClientContext(ctx)
info.RequestHeader().Set("Authorization", "Bearer "+token)
response, err := client.GetProduct(ctx, &productv1.GetProductRequest{Id: id})
```

服务端通过 `connect.CallInfoForServerContext(ctx)` 读取请求头和客户端信息。业务错误使用 `connect.NewError(code, message)`，通过 `WithCause(err)` 保留原始错误。

## 生成器配置

在业务项目中调用 `resourcegen.Generate` 时，配置以下字段：

| 字段 | 填写内容 |
|---|---|
| `FormatVersion` | 当前接口格式为 `1`，可使用 `resource.GeneratorFormatV1` |
| `RootDir` | 业务项目的绝对根目录 |
| `GenQueryImport` | 应用生成的 query 包导入路径，当前应用为 `github.com/abysstract/abysstract-admin/backend/internal/modules/product/query` |

其他输入和输出目录可参考商品示例的[生成入口](../backend/internal/codegen/resources/tasks.go)。输出路径相对于 `RootDir`，业务代码生成到业务项目中。

先从迁移后的数据库生成 Model/query，再生成协议与 Resource。具体命令见[开发说明](development.md)。

## 数据访问与回调

生成的 Repository 提供以下操作：

| 操作 | 方法 |
|---|---|
| 列表及总数 | `List` |
| 详情 | `GetByKey` |
| 事务内锁定详情 | `GetByKeyForUpdate` |
| 事务内锁定多条记录 | `GetManyForUpdate` |
| 新建 | `Create` |
| 选择性更新 | `Update` |
| 单条删除 | `DeleteByKey` |
| 批量删除 | `BatchDelete` |

专用查询使用应用的 query 包，传入当前 `resource.Session.DB()`、context 和数据范围。同事务业务逻辑、当前用户和提交后回调的规则见 [Resource 手册](ai-cookbook.md#5-标准操作与专用业务逻辑)。商品的[回调](../backend/internal/modules/product/hooks.go)与[事件写入](../backend/internal/modules/product/events.go)可作为示例。

## 权限声明

模块的 `Permissions` 注册生成的 `Register*Permissions`。例如商品模块使用 `gen.RegisterProductPermissions`；完整声明见[模块指南](module-guide.md#后端服务与前端页面)。

## 页面注册与生命周期

```ts
import {
  bindAdminAppPageLifecycle,
  createAdminApp,
  createLocalAuthStorage,
} from "@abysstract/admin-core";
import { coreSystemModule } from "@abysstract/admin-core/system";
```

在应用入口中，将业务模块与 `coreSystemModule` 交给 `createAdminApp`。页面与菜单的对应方式见[模块指南](module-guide.md#后端服务与前端页面)，完整入口见[应用前端](../frontend/apps/reference/src/main.tsx)。

每个应用使用独立的认证存储，例如 `createLocalAuthStorage("abysstract_product_example_token")`。

挂载 React 后调用 `bindAdminAppPageLifecycle({ application, root })`。页面进入浏览器前进后退缓存时保留界面，返回页面时重新读取本地登录状态；离开页面时释放应用资源。

## 安装前端包

在本仓库根目录打包公共前端包：

```bash
( cd frontend && npm ci && npm run build -w @abysstract/admin-proto -w @abysstract/admin-core )
( cd frontend && npm pack --workspace @abysstract/admin-proto && npm pack --workspace @abysstract/admin-core )
```

命令在 `frontend/` 下生成两个 `.tgz` 文件。在业务前端目录安装这两个文件，将下方路径和 `VERSION` 替换为实际文件名：

```bash
npm install /absolute/path/to/abysstract-admin-proto-VERSION.tgz /absolute/path/to/abysstract-admin-core-VERSION.tgz
```

当前仓库的应用通过 npm workspace 使用公共包。完成 README 后，可直接按[模块教程](new-module-tutorial.md)继续开发。

## 应用启动与关闭

`backend/internal/app/` 是当前应用的组合入口：`modules.go` 选择模块，`server.go` 组合路由、中间件和 HTTP 服务，`app.go` 管理启停顺序。

`adminapp.New` 校验配置和模块声明。`Start(ctx)` 打开数据库、初始化模块并返回 `http.Handler`。应用将 handler 挂载到自己的服务器，添加全局中间件，并启动监听。

关闭时先调用应用服务器的 `Shutdown` 等待请求完成，再调用 Core 的 `Shutdown` 关闭模块与数据库。应用传入的 `Options.DB` 由应用关闭。`module.Runtime.Context` 持续到 Core 关闭，适合模块后台任务。

启动或关闭失败时返回错误。`Start` 失败会释放已经创建的模块和 Core 打开的数据库。每个 Core 实例启动一次。

只需默认 HTTP 服务时，可以调用 `Run(ctx)`。它使用同一套模块初始化和关闭流程，并负责监听与请求排空。

## HTTP 路由与中间件

在 `backend/internal/app/server.go` 的 `NewServer` 中添加应用路由。

全局中间件也在这里配置。例如，将 HTTP 服务器的 `Handler: mux` 改为 `Handler: yourMiddleware(mux)`，其中 `yourMiddleware` 是应用自己实现的中间件。它会处理经过该服务器的系统接口和业务接口请求。

需要 Core 认证和权限校验的业务接口，通过模块的 `module.Definition.Services` 或 `HTTP` 注册，接入方式见[模块指南](module-guide.md#后端服务与前端页面)。直接挂到外层 mux 的路由由应用管理访问规则。

## 普通字段的页面默认行为

在 Go Resource 声明共享标签，例如 `schema.F("Brand", schema.Label("品牌"), schema.MaxLen(60))`。列表标题和表单标签依次采用页面 Overrides、共享标签、字段名。

标准创建表单为普通可创建字段提供类型零值：string 为 `""`、number 为 `0`、bigint 为 `0n`、boolean 为 `false`。页面 `initialValues` 可按字段覆盖这些值，显式 `undefined` 同样保留。关联字段和 unknown 类型需要显式配置。

`visible` 使用“类型零值 + 页面显式初值”判断创建字段的可见性。隐藏字段只保留页面显式提供的业务初值。服务端维护及当前模式不可写字段由表单自动过滤。

编辑表单按实体同名属性回填可更新字段。`editConfig.initialValues(entity)` 可按字段覆盖回填值。未操作的字段在更新请求中省略；操作过的字段按当前值提交，包括改动后恢复原值的情况。清空字符串会提交 `""`。

`CreateOnly()` 字段仅在创建表单填写；`NotListable()` 控制列表展示。数据库默认值由迁移定义，业务表单初值通过页面配置。

需要特殊展示时，使用列的 `hidden`、`render`、`title`；需要特殊输入时，使用表单的 `component`、`valueType`、`label`、`fieldProps`、`rules`、`visible`、`disabled`。附加 `rules` 在 `Required()` 和 `MaxLen()` 标准校验之后执行，长度按 Unicode 码点计数；bigint 保持完整精度。

## 故障说明

| 现象 | 处理 |
|---|---|
| 新库缺少系统表 | 配置目标数据库后，执行应用自己的迁移命令 |
| 修改公共前端包后页面仍是旧行为 | 重新构建公共包；独立项目重新打包并安装 |
| 启动报缺少权限声明 | 检查模块 `Permissions` 是否注册生成的 `Register*Permissions` |
| 生成文件写到错误位置 | 检查 `RootDir` 和输出目录配置 |
| 模块或页面 key 重复 | 为模块名称和页面标识使用应用内唯一值 |
| 前进后退返回空白页 | 确认入口调用 `bindAdminAppPageLifecycle` |
