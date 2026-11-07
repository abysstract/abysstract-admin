# Resource 开发手册

Resource 用一份 Go 定义描述字段行为和标准 CRUD 操作，由生成器产生后端读写代码及前端资源配置。本手册说明业务开发时的字段、操作和回调规则。

第一次接入请先完成[笔记模块教程](new-module-tutorial.md)。业务模块的注册方式见[模块指南](module-guide.md)。

在 `resources/*.json` 维护业务字段的模块，按“迁移 → 修改 JSON → `module generate <名称>`”更新 Proto 和 Resource。普通字段的接口、读写代码和页面配置由工具生成。Hook、Overrides 和自定义页面保留在手写文件中。

以下各节解释底层 Go `schema.Resource`、Proto、Hook 和数据范围规则。系统、现有商品模块及专用接口直接维护这些定义；商品字段的手写练习见[商品扩展参考](admin-core-getting-started.md#2-为商品增加品牌)。JSON 配置模块的生成定义由工具维护，业务扩展写在模块的手写扩展入口中。

## 1. 先明确各层职责

| 内容 | 维护位置 | 例子 |
|---|---|---|
| 表、列、索引和默认值 | 数据库迁移 | 商品品牌列 `brand` |
| Go 字段与数据库列的映射 | Model | `Brand` 映射到 `brand` |
| 请求、响应、字段编号和省略语义 | Proto | `optional string brand = 5` |
| 必填、长度、读写、搜索和排序能力 | Go `schema.Resource` | `schema.F("Brand", schema.MaxLen(60))` |
| 转换、CRUD 编排、前端类型和操作 | 生成代码 | Controller、Repository、operations |
| 共享标签 | Go `schema.Resource` | `schema.Label("品牌")` |
| 特殊展示、组件和专用操作 | React 页面 Overrides | bigint 输入、格式化、备注文本域 |
| 特殊业务校验及副作用 | Hook 或专用服务 | 编号规范化、提交后缓存失效 |

Proto 管接口，Resource 管行为，迁移管数据库。生成文件从这些源定义产生，修改源定义后重新生成。

## 2. 手写接口的修改顺序

1. **数据库迁移**：为新结构增加迁移版本，保留已有版本的内容。
2. **模型与查询**：迁移完成后生成 Model 和 Gen query；共用入口 `backend/cmd/modelgen` 读取 `backend/internal/codegen/modules.json` 中配置的表前缀与模型输出位置，连接来自 `ADMIN_DB_DSN`。
3. **Proto**：定义消息与 RPC，保持已有字段编号；可更新字段使用 `optional`。
4. **接口生成**：先生成 Go 和 TypeScript 协议代码。
5. **Resource 定义**：声明字段、标准操作、主键、权限和数据范围，并加入生成入口的资源列表。
6. **Resource 生成**：生成转换器、Repository、Controller、服务适配器及前端 schema/operations。
7. **业务与页面接入**：按需添加 Hook 或专用实现，组合模块、权限和页面。
8. **验证**：检查迁移、字段更新语义、权限、数据范围和实际页面操作。

系统资源定义位于 `backend/internal/resource/coredefs/`，商品示例位于 `backend/internal/modules/product/definition/`。完整可编译的最小实现以商品示例为准。

## 3. 字段怎样声明

以下片段来自商品示例的声明方式，用于理解常见选项：

```go
schema.F("ID", schema.ServerManaged()),
schema.F("ProductNo", schema.Required(), schema.MaxLen(64), schema.CreateOnly()),
schema.F("Name", schema.Required(), schema.Searchable(), schema.MaxLen(100)),
schema.F("PriceCents", schema.SortableOpt()),
schema.F("DeptID", schema.Filterable(), schema.CreateOnly()),
schema.F("Remark", schema.MaxLen(500)),
```

`F` 的第一个参数是 **Model 的 Go 字段名**。`Build()` 会根据 GORM 元数据解析实际数据库列名。

| 选项 | 含义 |
|---|---|
| `Label("品牌")` | 列表、新建、编辑共享标签，页面 Overrides 优先 |
| `Required()` | 输入必填；字符串要求有非空白内容 |
| `MaxLen(n)` | 字符串长度上限 |
| `Searchable()` | 参与关键词搜索 |
| `Filterable()` | 可作为列表等值筛选条件，还需在 List binding 中配置字段映射 |
| `SortableOpt()` | 允许按该字段排序 |
| `CreateOnly()` | 创建时填写，之后固定 |
| `ReadOnly()` | 客户端读取，写入由服务端承担 |
| `ServerManaged()` | 由服务器维护，例如 ID 和审计时间 |
| `EnumMap(...)` | 声明 Proto 枚举名与模型整数值的映射 |
| `RelationTo(...)` | 声明关联资源及选项的值、标签字段 |

关联选项的标签使用目标资源的字符串字段。生成的关联加载器通过目标资源的 operations 查询，沿用目标接口的权限与数据范围。

标准 List 的筛选字段需要 Proto presence：省略表示保持无筛选条件，显式 `0`、`false` 或空字符串表示按该值筛选。排序和筛选使用声明的字段白名单；默认排序应保持稳定，无排序条件时按资源主键升序。

## 4. 正确处理省略、零值和类型转换

更新请求中的 `optional` 区分两种意图：

- **省略字段**：保留数据库原值。
- **显式传值**：更新为该值，包括 `0`、`false` 和空字符串。

空更新请求返回错误。同值更新读取并返回当前实体，已有更新时间与回调状态保持不变。

| 转换方向 | 使用的输入 | 关注点 |
|---|---|---|
| Model → Entity | 数据库模型 | 时间戳、枚举和字段类型映射 |
| Create → Model | Create 请求 | 必填、长度、枚举、整数范围及创建权限 |
| Update → Model | 旧模型和 Update 请求 | optional presence、可更新字段及变更列 |
| List → Query | List 请求 | 筛选值类型、枚举范围及允许的排序字段 |

Create、Update 和 Entity 使用各自的消息定义。整数溢出、未知枚举及越界时间戳会返回输入错误。

Proto 字段名和 JSON 字段名用于接口；SQL、变更列集合 `ColumnSet`、排序和筛选使用从模型解析的数据库列名。

## 5. 标准操作与专用业务逻辑

标准操作包括 List、Get、Create、Update、Delete 和 BatchDelete。普通 CRUD 使用生成的 Controller 和 Repository。

| 需求 | 选择 |
|---|---|
| 标准字段校验、分页、转换和读写 | Resource 声明与生成代码 |
| 写入前规范数据、同事务记录事件 | 事务内 Hook |
| 提交后缓存失效或通知 | `AfterCommit*` 回调 |
| 密码、授权关系等独立业务流程 | 强类型 CUSTOM 或专用服务 |
| 对现有 Repository 增加缓存等行为 | typed injected Repository |

生成 Controller 持有 `*resource.Store`，Repository 使用 `resource.Session`，事务内 Hook 和创建数据范围校验接收 `resource.Tx`。事务提交边界由 Controller 管理；事务内逻辑使用传入的事务。

组合回调使用 `resource.ComposeHooks(system, business)`：Before 按系统→业务执行，After 按业务→系统执行。事务内回调返回错误时回滚。提交后回调按系统→业务执行。

业务 Hook 使用 `identity.FromContext(ctx)` 读取当前用户。同事务事件使用传入的事务写入；外部通知和缓存失效放在 `AfterCommit*` 中，事务成功提交后执行。创建结果包含数据库生成的主键、默认值和审计字段。

密码、授权等流程的实现方式见 [Resource 扩展方式](resource-migration.md)。

### 数据库错误

生成的 Controller 先保留 Hook 返回的 Connect 错误，再按生成的 Repository 用 `resource.NormalizeDBError` 归类的结果返回接口错误。Hook 自己执行的语句和事务提交返回的数据库错误不做归类，按“其他错误”处理：

| 数据库错误 | 接口错误 |
|---|---|
| 记录不存在 | `not_found` |
| 唯一约束冲突 | `already_exists`，`resource.ErrDuplicate` |
| 数值越界（MySQL/MariaDB） | `invalid_argument`，`resource.ErrOutOfRange` |
| CHECK 约束 | `invalid_argument`，`resource.ErrCheckViolation` |
| 外键约束：引用的记录不存在或仍被引用 | `failed_precondition`，`resource.ErrForeignKey` |
| 触发器等主动拒绝：MySQL/MariaDB `SIGNAL SQLSTATE '45xxx'`、PostgreSQL `RAISE EXCEPTION` | `failed_precondition`，消息为 `MESSAGE_TEXT` |
| 其他错误 | `internal`，消息固定为“服务内部错误” |

接口消息不包含驱动返回的库名、表名和约束定义；原始错误写入服务日志（`resource request failed`）。需要把数据库规则展示给用户时，在触发器中用 `SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '...'` 或 `RAISE EXCEPTION '...'` 写出面向用户的提示。

## 6. 权限与数据范围

### 权限注册

模块通过生成的 `Register*Permissions` 注册标准操作权限。前端依据同一权限标识显示操作，后端执行授权检查。

### 数据范围

数据范围决定记录可见性，与关键词、筛选和排序分别处理。角色 `data_scope` 取值见[权限说明](permission.md#数据范围)。`Auth` 中的部门、创建者字段填写 Model Go 字段名，由 `Build()` 解析数据库列。

JSON 配置的标准 CRUD 默认不启用数据范围。下面以笔记为例：新建笔记时，将记录归到当前用户及其部门；查询、修改和删除时，使用角色的数据范围筛选记录。以下命令在 `backend/` 执行。

**准备记录归属字段**

先通过业务迁移为 `notes_note` 增加非空 BIGINT 列 `dept_id` 和 `create_by`。已有笔记的部门和创建者由迁移按实际业务归属回填。这两列由后端填写，`note.json` 的 `fields` 继续只列出向用户开放的业务字段。

执行迁移，更新数据库表：

```sh
go run ./cmd/migrate
```

更新模型和查询代码，模型中会出现 `DeptID` 和 `CreateBy` 两个 int64 字段：

```sh
go run ./cmd/admin module generate notes
```

**配置 Resource 并重新生成**

新建手写文件 `internal/modules/notes/definition/note_scope.go`，配置部门和创建者对应的模型字段：

```go
package definition

import "github.com/abysstract/abysstract-admin/backend/resource/schema"

func init() {
    NoteResource.Auth = schema.AuthConfig{
        DataScope:    true,
        DeptField:    "DeptID",
        CreatorField: "CreateBy",
    }
}
```

这个文件在生成工具和应用加载 `definition` 包时都会生效。重新生成 Resource 实现，使 Controller 支持数据范围依赖：

```sh
go run ./cmd/admin module generate notes
```

**设置归属并校验新增记录的数据范围**

新建手写文件 `internal/modules/notes/note_scope.go`。下面的实现根据数据库选择整数转换类型，可用于 PostgreSQL、MySQL、MariaDB 和 SQLite：

```go
package notes

import (
    "context"

    "connectrpc.com/connect/v2"
    gen "github.com/abysstract/abysstract-admin/backend/internal/modules/notes/gen/resource"
    "github.com/abysstract/abysstract-admin/backend/internal/modules/notes/model"
    "github.com/abysstract/abysstract-admin/backend/database"
    "github.com/abysstract/abysstract-admin/backend/identity"
    "github.com/abysstract/abysstract-admin/backend/module"
    "github.com/abysstract/abysstract-admin/backend/resource"
)

func configureNoteScope(rt module.Runtime, deps gen.NoteControllerDeps) gen.NoteControllerDeps {
    deps.DataScopeProvider = rt.DataScope
    deps.Hooks = resource.ComposeHooks(&resource.HookSet[model.NotesNote]{
        BeforeCreate: func(ctx context.Context, tx resource.Tx, note *model.NotesNote) error {
            actor, ok := identity.FromContext(ctx)
            if !ok {
                return connect.NewError(connect.CodeUnauthenticated, "请先登录")
            }
            var user struct {
                DeptID *int64
            }
            if err := tx.DB().WithContext(ctx).Table("sys_user").
                Select("dept_id").Where("id = ?", actor.UserID).Take(&user).Error; err != nil {
                return err
            }
            if user.DeptID == nil {
                return connect.NewError(connect.CodeFailedPrecondition, "请先为当前用户分配部门")
            }
            note.DeptID = *user.DeptID
            note.CreateBy = actor.UserID
            return nil
        },
    }, deps.Hooks)
    deps.CreateScopeValidator = func(ctx context.Context, tx resource.Tx, note *model.NotesNote) error {
        cast := "BIGINT"
        driver := database.DriverOf(tx.DB())
        if driver == "mysql" || driver == "mariadb" {
            cast = "SIGNED"
        }
        candidate := tx.DB().WithContext(ctx).Table(
            "(SELECT CAST(? AS "+cast+") AS dept_id, CAST(? AS "+cast+") AS create_by) AS candidate",
            note.DeptID, note.CreateBy,
        )
        var visible int64
        scope := rt.DataScope.DataScope(ctx, "dept_id", "create_by")
        if err := scope.Apply(candidate).Count(&visible).Error; err != nil {
            return err
        }
        if visible == 0 {
            return resource.ErrScopeDenied
        }
        return nil
    }
    return deps
}
```

`BeforeCreate` 在保存前填写归属，用户需要已分配部门。`CreateScopeValidator` 随后在同一事务内检查这条待新增记录是否属于调用者的数据范围，越界时返回权限错误。`ComposeHooks` 保留已有标题处理等 Hook。

在 `internal/modules/notes/note.go` 的 `configureNote` 中保留原有 Hook、Overrides 配置，将末尾的 `return deps` 改为：

```go
return configureNoteScope(rt, deps)
```

**运行并确认效果**

重启后端。为测试用户分配笔记页面、查询和写入权限，将角色的数据范围设为“仅本人”。用该用户新建笔记并刷新，应能看到自己的笔记；其他用户创建的笔记应被过滤。切换角色到“本部门”后，应能看到同部门的笔记。角色的数据范围与接口操作权限分别配置。

读取、更新、删除及批量删除沿用相同的数据范围。批量删除中任一目标缺失、越权或删除失败时，整个批次回滚。商品模块的同类实现见 [scope.go](../backend/internal/modules/product/scope.go)。

## 7. 生成与使用

完成迁移、Proto 和 Resource 修改后，按[开发说明](development.md#修改与完整生成)运行对应应用的生成命令。生成器配置见[公共 API 用法](admin-core-usage.md#生成器配置)。

前端通过生成的 schema、operations 和 `ResourceTable` / `ResourceForm` 使用标准 CRUD。普通字段的标签、初值和编辑回填规则见[页面默认行为](admin-core-usage.md#普通字段的页面默认行为)。

生成后启动应用，确认字段可读写、已有数据保持正确，账号权限与数据范围符合业务要求。商品品牌字段的[完整练习](admin-core-getting-started.md#2-为商品增加品牌)给出了操作与预期结果。
