# 公共 API 用法

本页说明业务应用使用的公共包、生成器配置和前端接入方式。首次接入请先完成[模块教程](new-module-tutorial.md)。

## 引用 Go 模块

Go 模块路径是 `github.com/abysstract/abysstract-admin`，`go.mod` 位于仓库根目录，代码位于 `backend/`。在业务应用中添加依赖：

```bash
go get github.com/abysstract/abysstract-admin@v0.7.0
```

Go 模块与 npm 包共用同一个版本 tag（`vX.Y.Z`），升级时把 Go 依赖和两个前端包调整到同一版本。Go 模块的首个版本是 `v0.2.0`；`v0.1.0` 只发布了前端包。

在本地联调未发布的改动时，用 `replace` 指向本仓库根目录：

```bash
go mod edit -replace github.com/abysstract/abysstract-admin=/absolute/path/to/abysstract-admin
```

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
| `storage`、`storage/localstorage`、`storage/s3storage` | 文件存储接口、对象键工具与驱动，见[文件存储](storage.md) |
| `accesslog`、`accesslog/stdoutsink`、`accesslog/dbsink` | 请求日志的采集、分发与 sink，见[请求日志](access-log.md) |
| `captcha`、`captcha/siteverify`、`captcha/slidecaptcha` | 登录验证码接口与实现（Turnstile/hCaptcha、内置滑块），见[登录安全](configuration.md#登录安全) |

前端从 `@abysstract/admin-core` 导入应用与组件（图片直传组件在 `@abysstract/admin-core/upload`），从 `@abysstract/admin-proto` 导入系统协议类型。包的安装方式见下文[安装前端包](#安装前端包)。

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

两个包发布在 npm 上。在业务前端目录安装公共包和它们要求的 peer 依赖：

```bash
npm install @abysstract/admin-core @abysstract/admin-proto \
  @bufbuild/protobuf@^2 @connectrpc/connect@^2 @connectrpc/connect-web@^2 \
  antd@^6 @ant-design/icons@^6 @ant-design/pro-components@3.1.15-9 \
  react@^19 react-dom@^19 react-router-dom@^6
```

peer 依赖要带上版本号。admin-core 要求 antd 6、`@ant-design/icons` 6、React 18 或 19、React Router 6。`@ant-design/pro-components` 3 目前只有预发布版，admin-core 精确锁定一个版本，不写版本时 npm 会装 2.x，安装会因 `ERESOLVE` 失败；pro-components 3 要求 Node.js 22.12+。精确的版本范围以 `@abysstract/admin-core` 的 `package.json` 中 `peerDependencies` 为准。

`@abysstract/admin-core` 固定依赖同版本的 `@abysstract/admin-proto`，升级时两个包一起升级。

两个包发布的是 ES 模块，类型声明按打包器方式解析。TypeScript 项目请使用 `"moduleResolution": "bundler"`（Vite 模板的默认值）。

需要试用尚未发布的改动时，可以在本仓库根目录打包，再安装生成的文件：

```bash
( cd frontend && npm ci && npm run build -w @abysstract/admin-proto -w @abysstract/admin-core )
( cd frontend && npm pack --workspace @abysstract/admin-proto && npm pack --workspace @abysstract/admin-core )
npm install /absolute/path/to/abysstract-admin-proto-VERSION.tgz /absolute/path/to/abysstract-admin-core-VERSION.tgz
```

当前仓库的应用通过 npm workspace 使用公共包。完成 README 后，可直接按[模块教程](new-module-tutorial.md)继续开发。

## 应用启动与关闭

`backend/internal/app/` 是当前应用的组合入口：`modules.go` 选择模块，`server.go` 组合路由、中间件和 HTTP 服务，`app.go` 管理启停顺序。

`adminapp.New` 校验配置和模块声明。`Start(ctx)` 打开数据库、初始化模块并返回 `http.Handler`。应用将 handler 挂载到自己的服务器，添加全局中间件，并启动监听。

`module.Runtime.Storage` 是所有模块共享的文件存储。未传入 `Options.Storage` 时，`Start` 按 `ADMIN_STORAGE_*` 配置打开存储；需要在后台之外复用同一实例时，用 `adminapp.OpenStorage` 打开后传入，见[注入自定义存储](storage.md#6-注入自定义存储)。

`module.Runtime.AccessLog` 是请求日志分发器，未启用时为 `nil`。未传入 `Options.AccessLog` 时，`Start` 按 `ADMIN_ACCESS_LOG_*` 配置打开；应用自己的 API 服务需要写入同一请求日志时，用 `adminapp.OpenAccessLog` 打开后传入，见[在独立的 API 服务中复用](access-log.md#6-在独立的-api-服务中复用)。

关闭时先调用应用服务器的 `Shutdown` 等待请求完成，再调用 Core 的 `Shutdown` 写完队列中的请求日志并关闭模块与数据库。应用传入的 `Options.DB`、`Options.Storage` 和 `Options.AccessLog` 由应用关闭。`module.Runtime.Context` 持续到 Core 关闭，适合模块后台任务。

启动或关闭失败时返回错误。`Start` 失败会释放已经创建的模块和 Core 打开的数据库。每个 Core 实例启动一次。

只需默认 HTTP 服务时，可以调用 `Run(ctx)`。它使用同一套模块初始化和关闭流程，并负责监听与请求排空。

## HTTP 路由与中间件

在 `backend/internal/app/server.go` 的 `NewServer` 中添加应用路由。

全局中间件也在这里配置。例如，将 HTTP 服务器的 `Handler: mux` 改为 `Handler: yourMiddleware(mux)`，其中 `yourMiddleware` 是应用自己实现的中间件。它会处理经过该服务器的系统接口和业务接口请求。

需要 Core 认证和权限校验的业务接口，通过模块的 `module.Definition.Services` 或 `HTTP` 注册，接入方式见[模块指南](module-guide.md#后端服务与前端页面)。直接挂到外层 mux 的路由由应用管理访问规则。

## 普通字段的页面默认行为

在 Go Resource 声明共享标签，例如 `schema.F("Brand", schema.Label("品牌"), schema.MaxLen(60))`。列表标题和表单标签依次采用页面 Overrides、共享标签、字段名。

标准创建表单为普通可创建字段提供类型零值：string 为 `""`、number 为 `0`、bigint 为 `0n`、boolean 为 `false`。可空列（创建请求中为 Proto `optional`，生成的 schema 字段带 `optional: true`）不填初值，用户未填写或清空数字时，创建请求省略该字段，数据库保存 NULL；可选外键因此不会被填成 `0`。页面 `initialValues` 可按字段覆盖这些值，显式 `undefined` 同样保留。关联字段和 unknown 类型需要显式配置。

`visible` 使用“类型零值 + 页面显式初值”判断创建字段的可见性。隐藏字段只保留页面显式提供的业务初值。服务端维护及当前模式不可写字段由表单自动过滤。

编辑表单按实体同名属性回填可更新字段。`editConfig.initialValues(entity)` 可按字段覆盖回填值。未操作的字段在更新请求中省略；操作过的字段按当前值提交，包括改动后恢复原值的情况。清空字符串会提交 `""`。

`CreateOnly()` 字段仅在创建表单填写；`NotListable()` 控制列表展示。数据库默认值由迁移定义，业务表单初值通过页面配置。

需要特殊展示时，使用列的 `hidden`、`render`、`title`；需要特殊输入时，使用表单的 `component`、`valueType`、`label`、`fieldProps`、`rules`、`visible`、`disabled`。附加 `rules` 在 `Required()` 和 `MaxLen()` 标准校验之后执行，长度按 Unicode 码点计数；bigint 保持完整精度。

页面的布局和文案按[后台设计规范](../frontend/packages/admin-core/DESIGN.md)。`ResourceTable` 和 `ResourceForm` 默认就是规范里的做法：

| 行为 | 默认 | 调整方式 |
|---|---|---|
| 每页条数 | 20；只有一页时隐藏分页器，超过 5 页显示快速跳转 | `query.defaults.pageSize` |
| 地址栏 | 关键词、页码、每页条数、排序写进 `q`、`page`、`size`、`sort` 参数，刷新或分享链接后是同一画面 | 同一页面有多个表格，或表格在抽屉里时，设 `query.syncUrl: false` |
| 新增、编辑 | 720 宽抽屉，标题栏右侧是「取消 保存」，成功后提示「已创建」「已保存」，失败时抽屉不关、提示原因 | `form.createConfig` / `form.editConfig` |
| 表单排布 | 标签在上；每个字段单独一行，宽度按类型（数字 6、文本和下拉 12、长文本 24）；布尔字段用 Checkbox；长文本自动增高 | 表单项的 `span`、`sameRowAsPrevious` |
| 操作列 | 最多露出两个文字链接，其余收进「⋯」，删除排在最后 | 自定义行操作设 `danger: true` 会排到最后 |
| 没有权限的操作 | 不显示 | — |
| 删除 | 确认框写清后果，按钮是「删除 / 取消」；成功提示「已删除」，失败用 `Modal.error` 说明原因 | — |
| 批量操作 | 选中行后出现在表格底部 | 自定义工具栏项设 `placement: "selection"` |
| 空状态 | 「还没有记录」+「新增」；有关键词时「没有符合条件的记录」+「清除筛选」 | `emptyText` |
| 数字列 | — | 列的 `numeric: true`：右对齐、等宽数字 |

`onError` 只在列表加载失败时调用；新增、编辑、删除的结果由 `ResourceTable` 自己提示，页面不用再在 `onSuccess` 或 `onError` 里弹消息。

手写的新增、编辑抽屉用 `ResourceFormDrawer`，把它传给子组件的 `actionsContainer` 交给 `ResourceForm`，「取消 保存」就会出现在抽屉标题栏：

```tsx
import { ResourceForm, ResourceFormDrawer } from "@abysstract/admin-core/resource";

<ResourceFormDrawer open={open} title="新增笔记" onClose={close}>
  {(actionsContainer) => (
    <ResourceForm
      schema={noteSchema}
      mode="create"
      onCreate={submit}
      onCancel={close}
      actionsContainer={actionsContainer}
    />
  )}
</ResourceFormDrawer>;
```

## 界面文案

框架组件和系统页面的按钮、提示、空状态文案集中在 `defaultAdminTexts`（`@abysstract/admin-core`）。通过 `host.texts` 覆盖其中一部分，未覆盖的沿用默认值：

```tsx
const application = createAdminApp({
  host: {
    // …
    texts: {
      action: { create: "添加" },
      table: { empty: "这里还没有内容" },
    },
  },
  modules,
});
```

业务页面用 `useAdminTexts()` 读取当前文案，与框架页面保持一致。

## 登录页

宽屏时登录页左侧是品牌区，右侧是登录表单；窄屏（手机、平板竖屏）时背景铺满整页，产品名和表单放在一张卡片里。产品名用 `host.title`，其余部分通过 `host.login` 配置，都可以不传：

```tsx
const application = createAdminApp({
  host: {
    // apiBaseUrl、mode、authStorage、title …
    login: {
      logo: <img src="/logo.svg" alt="" width={32} height={32} />,
      subtitle: "订单与库存管理",
      background: "url(/login-bg.jpg) center / cover",
      footer: "© 2026 示例公司",
    },
  },
  modules,
});
```

- `background` 是 CSS `background` 的值，宽屏用于左侧品牌区，窄屏铺满整页。窄屏时页脚直接显示在背景上，背景图底部宜留浅色区域。不传时是由主题 token 生成的浅色色块，改 `colorPrimary` 或换暗色算法时跟着变。
- 表单的标签、按钮和错误提示在 `texts.login` 里，可以和其他文案一样覆盖。用户名或密码不对时，错误标在密码字段下方；账号锁定、停用、来源 IP 尝试过多时显示服务端返回的原因。

### 登录验证码

服务端启用验证码（`ADMIN_CAPTCHA_PROVIDER`，见[登录安全](configuration.md#登录安全)）后，失败次数达到阈值时，用户点“登录”会先在表单的位置显示“安全验证”面板，宽度与表单一致；验证通过后面板收起，自动提交。面板标题右侧有“换一张”（仅内置滑块）和“关闭”，关闭后回到表单，填过的内容保留。

- `slide`（内置滑块）：前端不需要任何配置。图片和滑轨与输入框等宽，滑轨与大号输入框等高。滑块可以拖动，也可以用方向键移动（按住 Shift 移动得更快）、Home/End 到两端、回车确认。
- 第三方服务：在 `host.login.captcha` 提供组件，渲染对方的验证组件，把 token 交给 `onChange`。`loadChallenge()` 返回服务端配置的 `kind` 和 `payload`（其中有 `siteKey`），`resetKey` 变化时重置组件，因为 token 只能用一次。下面以 Cloudflare Turnstile 为例，页面需先加载 `https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit`：

```tsx
import { useEffect, useRef } from "react";
import type { AdminLoginCaptchaProps } from "@abysstract/admin-core";

declare const turnstile: {
  render(el: HTMLElement, options: { sitekey: string; callback(token: string): void; "expired-callback"(): void }): string;
  remove(id: string): void;
};

function TurnstileCaptcha({ onChange, resetKey, loadChallenge }: AdminLoginCaptchaProps) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let widget: string | undefined;
    let active = true;
    void loadChallenge().then(({ payload }) => {
      if (!active || !ref.current) return;
      const { siteKey } = JSON.parse(payload) as { siteKey: string };
      widget = turnstile.render(ref.current, {
        sitekey: siteKey,
        callback: (token) => onChange({ token }),
        "expired-callback": () => onChange(undefined),
      });
    });
    return () => {
      active = false;
      if (widget) turnstile.remove(widget);
    };
  }, [resetKey, loadChallenge, onChange]);
  return <div ref={ref} />;
}

createAdminApp({ host: { /* … */ login: { captcha: TurnstileCaptcha } }, modules });
```

## 全局 antd 配置

`createAdminApp` 的 Root 用一个 antd `ConfigProvider` 包住整个后台。`host.theme` 和 `host.locale` 传给它的同名属性；其余全局配置放在 `host.configProvider`，例如：

```tsx
const application = createAdminApp({
  host: {
    // apiBaseUrl、mode、authStorage、title …
    theme: { token: { colorPrimary: "#0f766e" } },
    locale: zhCN,
    configProvider: { componentSize: "middle", form: { requiredMark: "optional" } },
  },
  modules,
});
```

admin-core 默认设置 `button: { autoInsertSpace: false }`，「查询」「取消」等两个汉字的按钮中间不插空格，这一设置同时作用于业务页面的按钮。需要 antd 的默认行为时，传入 `configProvider: { button: { autoInsertSpace: true } }`；`button` 的其他字段与默认值合并。

## 自定义列表页的搜索栏

`ResourceTable` 顶部的搜索卡片以 `ResourceSearchBar` 导出，手写的列表页可以直接使用，与 Resource 页面保持一致：

```tsx
import { ResourceSearchBar } from "@abysstract/admin-core/resource";

<ResourceSearchBar
  keyword={keyword}
  onKeywordChange={setKeyword}
  onSearch={() => void load(keyword)}
  onReload={() => void load(keyword)}
>
  {/* 可选：关键词之后、按钮之前的额外筛选控件 */}
</ResourceSearchBar>;
```

回车、点击「查询」或把关键词删空时调用 `onSearch`；删空时传入空字符串，以它为准。不传 `onReload` 时不显示「刷新」。占位文字写搜索范围，例如 `placeholder="用户名、昵称"`，默认是「关键词」。卡片的背景、边框和圆角取自 antd token（`colorBgContainer`、`colorBorderSecondary`、`borderRadiusLG`），跟随 `host.theme` 和暗色算法；页面内的卡片不加阴影。

## 故障说明

| 现象 | 处理 |
|---|---|
| 新库缺少系统表 | 配置目标数据库后，执行应用自己的迁移命令 |
| 修改公共前端包后页面仍是旧行为 | 重新构建公共包；独立项目重新打包并安装 |
| 启动报缺少权限声明 | 检查模块 `Permissions` 是否注册生成的 `Register*Permissions` |
| 生成文件写到错误位置 | 检查 `RootDir` 和输出目录配置 |
| 模块或页面 key 重复 | 为模块名称和页面标识使用应用内唯一值 |
| 前进后退返回空白页 | 确认入口调用 `bindAdminAppPageLifecycle` |
