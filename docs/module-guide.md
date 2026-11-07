# 模块开发与生命周期

完整步骤见[笔记模块教程](new-module-tutorial.md)。`admin.modules.json` 选择应用加载和公共生成的模块。`module generate NAME` 直接读取指定模块的 `module.json`，支持安装前生成代码。`system` 始终是应用基础模块。

这些命令在框架仓库和独立应用中通用。生成的 Go 代码按应用 `go.mod` 的模块路径导入应用自己的包，只从 `github.com/abysstract/abysstract-admin/backend/` 导入框架公共包。独立应用的搭建步骤和差异见[新建独立应用](standalone-app.md)。

## 应用清单

`admin.modules.json` 位于仓库根目录：

| 字段 | 说明 |
| --- | --- |
| `modules` | 已安装模块的 `module.json` 路径，由 `update` 和 `uninstall` 维护。独立应用用 `"system"` 选择框架的系统模块 |
| `frontendRoot` | 可选，前端源码目录，默认 `frontend/apps/reference/src`。`new` 在 `<frontendRoot>/modules/<名称>` 创建前端模块，`sync`、`update` 在这里写入 `modules.gen.ts` |

## 按任务选择命令

新建模块：`new` → 创建并执行业务表迁移 → `resource` 创建 JSON → 修改字段规则 → `generate` → `update` → 启动应用。

已有模块增加表或字段：业务迁移 → 新表执行 `resource`，已有 JSON 直接改字段 → `generate`。菜单、权限或示例角色有变化时执行 `update`，把声明写入数据库。

`new` 创建目录、入口、空菜单声明和 `resources/` 目录。`resources/` 专门存放该模块的 CRUD 配置；每个 JSON 由 `resource MODULE TABLE` 根据已有表创建，之后由开发者维护，`module generate` 读取它。模块安装前即可开发。`update` 会接入尚未安装的模块，卸载后执行它也会重新安装；开发期间可以只执行 `generate`。

| 命令 | 文件与数据库行为 |
| --- | --- |
| `new NAME` | 创建模块约定；已有目录时报错 |
| `resource MODULE TABLE` | 从数据库创建初始 CRUD JSON；已有文件时报错，保留原内容 |
| `generate NAME` | 读取 `resources/*.json` 和数据库，生成 Proto、Model/query、Resource 和页面；保持安装选择，不改写 JSON。Model/query 在命令进程内生成，协议代码通过 `buf` 生成（需要在 `PATH` 中），Resource 实现用 `go run` 编译模块的 `definition` 包生成 |
| `update NAME` | 首次使用时接入模块，已接入时更新菜单和权限；保存相应迁移并执行应用待执行迁移 |
| `uninstall NAME` | 从已安装模块列表移除，执行拥有对象清理迁移；保留业务表、业务数据和普通角色 |
| `status NAME` | 查询安装状态、应用和模块待执行迁移、已执行快照及待更新声明差异 |
| `sync` | 同步已安装模块的加载代码、开发任务清单、Buf 配置和历史迁移函数列表 |
| `check` | 检查已安装模块的加载代码、开发任务清单、Buf 配置和迁移函数列表是否与 `admin.modules.json` 一致 |

这些命令支持 `--root`；写入命令支持 `--dry-run`。`generate --dry-run` 读取数据库验证声明并列出文件计划，实际编译与下游生成器在正式生成时执行。`update/uninstall --dry-run` 只展示文件和执行计划。

生命周期命令执行**目标数据库全部待执行应用迁移**。数据库由 `ADMIN_DB_DRIVER` 和 `ADMIN_DB_DSN` 指定。模块迁移快照属于应用迁移历史；菜单变化和执行状态由 Goose 管理。重复执行 `update` 时，声明相同则复用快照，重复卸载继续执行已有待执行迁移。

模块列表及生成文件先保存，数据库迁移随后执行。迁移失败时命令返回错误，保留模块列表及迁移文件；使用 `status NAME` 查看实际状态，修复失败原因后重跑同一命令。每个菜单迁移在事务中执行，先前成功的应用迁移保留。本地开发时，重新执行 `go run ./cmd/server` 加载后端修改，前端保持 `npm run dev` 运行。部署时重新构建并发布应用。

## 持续开发与文件归属

每个资源在 `resources/<name>.json` 声明业务规则。首次执行 `go run ./cmd/admin module resource notes notes_note --title 笔记` 创建 `resources/note.json`，已有模块增加表也使用此入口。`--name` 指定 Go 风格资源名，默认从表名去掉模块前缀后转换；文件名由资源名转为 snake_case。`--title` 默认使用资源名，`--updated-at` 默认使用 `update_time`。`--dry-run` 展示路径和 JSON，命令只读取数据库。

工具沿用模型生成器的忽略字段和覆盖规则，为普通字段填写列名和同名标签，主键和更新时间由服务管理。数据库类型和可空性由后续生成时读取；`required`、搜索、排序和长度等业务规则由开发者调整。`NOT NULL` 与字符串非空白校验含义不同，工具保留开发者对 `required` 的选择。JSON 创建后由开发者维护，`module generate` 读取它。

数据库决定字段存储类型及可空性，业务声明决定暴露列、显示标签、必填、搜索、排序及长度规则。增加表或字段后，按“迁移 → 修改资源声明 → generate”执行；菜单或权限有变化时再执行 `update`，然后启动应用。

生成器维护 Proto、`definition/*.gen.go`、`definition/codegen.go`、模块 `*.gen.go`、前端 `pages.gen.ts` 与 `resources.lock.json`。锁文件保存字段编号、类型、presence、退役编号和产物摘要，应一起提交。编辑生成产物会在下次生成时报错；将业务改动移到手写入口或扩展文件后重新生成。

以下文件首次生成后由开发者维护：

- `<resource>.go` 的 `configure<Resource>`：配置 ControllerDeps 中的 Hook；声明专用操作后注入强类型 Overrides。JSON 配置的标准 CRUD 默认不启用数据范围；启用步骤见 [Resource 手册](ai-cookbook.md#数据范围)。
- `module.go`：组合生成模块与专用业务服务。
- `definition/resources.go` 及独立定义扩展文件：配置专用 Resource 行为。
- `pages/<Resource>List.tsx`、前端 `index.tsx`：自定义页面、操作及页面配置。
- `seed.json`：菜单与示例角色声明，可增加、修改或移除菜单/权限。

生成会追加新资源页面及菜单。已有菜单声明修改得到保留；生成器只更新仍与上一生成默认值一致的菜单。每个资源的权限前缀为 `biz:<module>:<resource>`，页面标识为 `biz/<module>/<resource>`。

字段编号按资源持久化；重排字段保持编号，删除字段保留编号与名称。类型、可空性或服务器管理归属变化返回兼容性错误；通过显式的新版本 API 迁移处理。恢复已退役字段名和删除整项公共服务同样需要显式 API 版本设计。标准字段类型为 string、bool、int16、int32、int64 和 time.Time；string/bool/int32/int64 支持可空存储。可空 int16/time.Time 及其他类型使用专用转换。当前标准 CRUD 要求自增主键 `id` 生成 int64 模型字段（PostgreSQL `BIGSERIAL`/`BIGINT`、MySQL/MariaDB `BIGINT`、`BIGINT UNSIGNED` 或 `INT UNSIGNED`、SQLite `INTEGER PRIMARY KEY`），以及非空时间更新审计列，其他结构采用手写 Proto/Resource。无符号整数的映射见下节。

资源的 `operations` 数组可配置同包手写 Proto 中的专用请求/响应，工具生成 Partial Resource、强类型 Overrides、权限和菜单。每项包含 `method`、`request`、`response`、`action`、`title`、`proto` 和显式的 `scope: "exempt"`。exempt 表示业务实现负责该操作的数据范围检查；通用认证和动作权限仍由框架执行。生成的列表页工具栏仍是标准新增、编辑和删除，新方法要在页面中调用 `operations.custom.<方法名>`。手写消息的字段编号与兼容性由其 Proto 作者维护，生成器检查专用方法签名变更及移除。示例见教程第 6 节。

已有商品模块保持手写 Proto、Resource、Hook 和页面，`generate product` 执行现有生成链路。已有一次性 scaffold 模块也可保持手写模式；改用 JSON 配置时，将业务扩展移到上述手写文件，并审查现有协议编号。工具对生成目标冲突直接报错。

## MySQL/MariaDB 无符号整数

`BIGINT UNSIGNED AUTO_INCREMENT` 主键和无符号外键列可以直接走 `module resource` → `module generate` 链路。生成器把无符号列映射为能容纳其取值范围的**有符号**类型，Model、Proto、Resource 和前端使用同一种类型：

| 数据库列 | Go Model | Proto | TypeScript |
| --- | --- | --- | --- |
| `BIGINT UNSIGNED` | `int64` | `int64` | `bigint` |
| `INT UNSIGNED` | `int64` | `int64` | `bigint` |
| `MEDIUMINT UNSIGNED`、`SMALLINT UNSIGNED` | `int32` | `int32` | `number` |
| `TINYINT UNSIGNED` | `int32` | `int32` | `number` |

可空列生成 `*int64`/`*int32` 和 Proto `optional`，presence 语义与有符号列相同。PostgreSQL 和 SQLite 没有无符号整数，映射保持不变。

选择 `int64` 而不是 Proto `uint64` 的原因：

- 框架的主键契约只有一种类型。`resource.Repository`、生成的 Controller/Adapter、批量删除和 AccessScope 数据范围都按 `int64` 主键工作，系统表的用户、部门 ID 同样是 `int64`；业务表引用系统表或彼此引用时，Go 代码可以直接比较和赋值，无需转换。
- 前端精度不受影响。Protobuf-ES 把 `int64` 和 `uint64` 都映射为 `bigint`，Connect JSON 把两者都编码为字符串，admin-core 表格和表单全程保留 `bigint`。
- 线上兼容。非负值在 `int64` 和 `uint64` 下的二进制编码与 JSON 文本相同，确有需要时可以通过显式的新版本 API 改为 `uint64`。

取舍是取值上限：`BIGINT UNSIGNED` 列只能存取不超过 `9223372036854775807`（2^63−1）的值。读取更大的值时驱动返回范围错误，请求失败，不会静默截断；自增主键在实际使用中不会达到这个上限。写入负数或超出列范围的值时，MySQL/MariaDB 严格模式（默认 `sql_mode` 含 `STRICT_TRANS_TABLES`）拒绝写入，生成的服务返回 `invalid_argument`。需要完整 64 位无符号范围的列，在模块 `codegen/modelcfg` 中用 `gen.FieldType` 覆盖模型类型，并为它编写专用 Proto/Resource。

## 菜单升级与卸载

升级逐字段比较数据库当前值与旧快照：仍为旧默认值的字段更新到新声明，管理员编辑过的字段保留。新增拥有对象、删除退役对象并清理授权关系。已有示例角色保留管理员修改和额外授权，声明中新增加或移除的授权按差异处理。删除菜单仍被保留对象引用时事务失败，先调整父子归属再重跑。

卸载清理模块拥有的菜单、示例角色和关联，保留普通角色及业务数据。删除业务数据由明确的应用迁移承担。卸载同时移除公共生成任务，之后可保留或删除前后端源码；历史迁移内嵌声明，支持源码删除后的新库初始化。恢复源码并执行 `update` 可复用保留的业务数据。

## 模块声明

`new <名称>` 自动写入 `module.json`。以下字段的约定路径均从仓库根目录计算，普通开发沿用生成值。`generate` 按这些字段写入文件：修改路径后，生成物写到新位置，页面和协议之间的相对导入随之计算。`module.json` 本身固定在 `backend/internal/modules/<名称>/`，`resources/` 和 `resources.lock.json` 与它放在一起：

| 字段 | 用途与默认值（以 notes 为例） |
| --- | --- |
| `name` | 模块标识 `notes` |
| `requires` | 模块依赖，默认 `["system"]`；依赖其他模块时在此填写模块名 |
| `backend.package` / `constructor` | `backend/internal/modules/notes` / `NewModule` |
| `frontend.entry` / `export` | `<frontendRoot>/modules/notes/index.tsx` / `notesModule`；页面写在入口所在目录的 `pages/` 下 |
| `generation.name` / `tablePrefix` | `notes` / `notes_`，选择模型生成范围。按 `<前缀>_` 边界匹配：`notes` 与 `notes_` 都只选择 `notes_*` 表，不会选中 `notesx_*`。模块之间的前缀不能相同或互为前缀（如 `shop_` 与 `shop_order_`），`new`、`check`、`sync`、`update`、`generate` 和 `modelgen` 遇到这种情况会报错并指出冲突的模块 |
| `generation.modelOutput` | `backend/internal/modules/notes`，其下生成 model/query |
| `generation.prefix` / `protoSource` | `company/notes/v1/` / `backend/internal/modules/notes/proto` |
| `generation.goOutput` / `tsOutput` | 后端模块的 `gen/proto` / 前端模块的 `gen/proto`；`goOutput` 和 `modelOutput` 须位于 `backend/` 下 |
| `resource.package` / `config` / `resources` | 模块 `definition` 包 / `GeneratorConfig` / `Resources` |
| `seed` | `backend/internal/modules/notes/seed.json` |

`resources/*.json` 的 `name` 是 Go 风格资源名，`table` 是该模块前缀的表，`title` 是页面和菜单标题，`updatedAt` 指向非空时间更新列。`fields` 每项用 `column` 选择数据库列，`label` 指定显示名称；可选规则为 `required`、`searchable`、`sortable`、`maxLength`。字段存储类型由数据库读取。

## 后端服务与前端页面

后端模块通过 `NewModule() (module.Definition, error)` 返回定义。`Services` 声明服务描述，`Permissions` 注册操作权限；`Create` 使用 `module.Runtime` 创建 Controller、Adapter 并提供实际服务注册。手写 `module.go` 调用生成的模块创建函数，专用服务在此组合。应用的 `internal/app/modules.go` 调用生成的 `selectedModules()` 取得已安装模块。

前端模块导出 `AdminFrontendModule`，其中 `pages` 将页面 key 映射到 React 懒加载页面。生成器维护 `pages.gen.ts`，手写 `index.tsx` 合入 `generatedPages` 并可追加专用页面。安装清单生成 `src/modules.gen.ts`，应用入口将这些模块与系统模块交给 `createAdminApp`。后端菜单的 `component` 使用相同页面 key，登录后的菜单决定可见路由。

## Seed 声明

生成器为标准资源填写目录、页面及动作菜单。业务菜单和示例角色在模块 `seed.json` 中维护，例如：

```json
{
  "module": "notes",
  "menus": [
    {"key": "notes.root", "name": "笔记管理", "path": "notes", "type": "M"},
    {"key": "notes.note", "parent": "notes.root", "name": "笔记", "path": "note", "component": "biz/notes/note", "type": "C", "permission": "biz:notes:note:list"}
  ],
  "roles": [
    {"key": "notes.reader", "name": "笔记只读", "dataScope": 1, "menus": ["notes.root", "notes.note"]}
  ]
}
```

父菜单排在子菜单前，`key` 是稳定的归属标识。页面 `component` 对应前端模块配置的页面 key，`permission` 对应后端动作权限。示例角色的 `dataScope: 1` 表示全部数据范围，取值见[权限说明](permission.md#数据范围)。普通角色通过系统角色管理授权。

首次安装和编辑 Seed 后都执行 `update`，命令根据安装状态创建相应迁移。归属记录存放在 `sys_module_binding`。快照保存到 `backend/migrations/modules/`，各方言 Go 迁移内嵌声明并调用公共 `moduleseed`；删除业务源码后这些历史文件继续保留。

## 模块外哪些文件会变化

| 操作 | 模块目录之外的文件 |
| --- | --- |
| 新建模块 `new` | 创建内容均在模块前后端目录内 |
| 创建业务表或改列 | 开发者在 `backend/migrations/<driver>/` 新增 SQL 迁移 |
| 同步 `sync` | 框架仓库（根目录有 `buf.base.yaml`）中为 `backend/internal/codegen/modules.json`、`backend/internal/codegen/resources/tasks.gen.go`、`buf.yaml`；sync 同时维护下面列出的模块加载代码及迁移函数列表，内容相同时保留原文件 |
| 安装或卸载 | `admin.modules.json`、`backend/internal/app/modules.gen.go`、`<frontendRoot>/modules.gen.ts`，以及上述公共生成配置 |
| 安装、升级或卸载产生新快照 | `backend/migrations/modules/<version>_<module>_<action>.json`、`backend/migrations/` 下每个已有方言目录的 Go 迁移及 `backend/migrations/go_registry.gen.go` |
| 删除已卸载模块源码 | 模块外文件已在卸载时更新，历史快照与迁移函数列表保留 |

命令列出实际变化路径，`--dry-run` 可预览。框架公共包、应用手写模块入口及前端手写模块入口调用生成的模块加载代码；模块增减通过以上文件完成。公共 Proto 生成刷新已选模块；`module generate NAME` 刷新已选模块及指定模块的协议与依赖，提交前审查实际 diff。

## 当前需要手写的部分

当前 SQL 迁移由开发者创建；初始资源 JSON 使用 `module resource` 从已有表创建。迁移创建命令和已有手写模块迁入 JSON 配置属于后续工作。商品模块仍需手写 Proto 和 Go Resource。

`resources/*.json` 填写暴露列和必填、搜索、排序、长度规则。数据范围、关联和枚举通过手写 Resource 扩展配置，接入方式见 [Resource 手册](ai-cookbook.md)。标准 JSON CRUD 要求生成 int64 模型字段的自增主键 `id`，MySQL/MariaDB 的 `BIGINT UNSIGNED` 主键也在其中（见[无符号整数](#mysqlmariadb-无符号整数)）；其他主键结构使用手写 Proto 和 Go Resource。

通过 `operations` 配置的自定义操作，工具生成服务方法、客户端、权限和菜单；业务实现和页面按钮由开发者编写。`scope` 固定为 `exempt`，涉及业务记录时由业务实现检查数据范围。独立服务的 Proto、业务实现、服务接入、权限、菜单和页面由开发者维护，协议类型和客户端通过生成命令更新。步骤见[自定义请求与多表列表](custom-module-api.md)。
