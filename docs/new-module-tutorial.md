# 创建并持续开发笔记模块

先完成 [README 快速开始](../README.md#快速开始体验自带后台)，确认 `.env`、数据库和前端依赖已经就绪。若后端仍在运行，先在后端终端按 `Ctrl+C` 停掉，避免占用 `:8080`。

SQL 示例分为 PostgreSQL 和 MySQL 两组，按 `.env` 中的数据库类型选择。MariaDB 使用 MySQL 示例，迁移目录改为 `migrations/mariadb`。SQLite 在 `migrations/sqlite` 中编写对应语法的迁移。

## 1. 创建模块和业务表

从仓库根目录进入后端目录，后续命令在 `backend/` 下执行：

```sh
cd backend
```

创建笔记模块 `notes`，生成前后端目录、模块配置和基础文件：

```sh
go run ./cmd/admin module new notes
```

安装用于创建数据库迁移文件的 Goose 命令行工具：

```sh
go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
```

创建迁移文件，并填写 SQL。

### PostgreSQL

创建迁移文件：

```sh
"$(go env GOPATH)/bin/goose" -dir migrations/postgres create notes_note sql
```

填写 SQL：

```sql
-- +goose Up
CREATE TABLE notes_note (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  update_time TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose Down
DROP TABLE notes_note;
```

### MySQL

创建迁移文件：

```sh
"$(go env GOPATH)/bin/goose" -dir migrations/mysql create notes_note sql
```

填写 SQL：

```sql
-- +goose Up
CREATE TABLE notes_note (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  title VARCHAR(100) NOT NULL,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;
-- +goose Down
DROP TABLE notes_note;
```

已有库统一使用无符号主键时，`id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY` 同样可用，引用它的列也可以是 `BIGINT UNSIGNED`；这些列在 Model、Proto 和页面中都是 `int64`，取值上限见[模块指南](module-guide.md#mysqlmariadb-无符号整数)。

### 继续生成资源

执行数据库迁移，创建 `notes_note` 表：

```sh
go run ./cmd/migrate
```

创建模块时生成的 `internal/modules/notes/resources/` 目录专门存放该模块的 CRUD 配置。每个 JSON 指定业务表、开放字段和字段规则。

根据已创建的表生成笔记配置：

```sh
go run ./cmd/admin module resource notes notes_note --title 笔记
```

命令会创建 `internal/modules/notes/resources/note.json`，并根据表结构填写初始配置：

- 资源名称为 `Note`，表名为 `notes_note`。
- 页面标题为“笔记”，由命令中的 `--title 笔记` 指定。
- 更新时间列为 `update_time`。
- 普通字段使用列名作为初始显示标签，例如 `title`。

主键 `id` 和更新时间由服务维护。

需要指定其他资源名称或更新时间列时，使用 `--name` 或 `--updated-at`。省略 `--title` 时，页面标题使用资源名称。加上 `--dry-run` 可以预览 JSON，文件会在正式执行时创建。

JSON 首次由工具创建，此后由开发者维护，`module generate` 读取它。目标文件已存在时命令报错并保留原内容。

打开生成的 `note.json`，确认开放字段，将 `title` 字段调整为以下业务规则：

```json
{
	"column": "title",
	"label": "标题",
	"required": true,
	"searchable": true,
	"maxLength": 100
}
```

按业务需要调整字段选择、必填、搜索、排序和中文标签。数据库 `NOT NULL` 表示存储列必有值，字符串 `required` 还会校验非空白内容，因此工具将必填规则留给开发者填写。

## 2. 生成模块代码

预览生成文件计划：

```sh
go run ./cmd/admin module generate notes --dry-run
```

生成代码：

```sh
go run ./cmd/admin module generate notes
```

生成完成后可看到 Model/query、Note Proto、Resource、Controller 创建与服务注册代码、菜单和 ResourceTable 页面。此时 `admin.modules.json` 中仍只有原来的安装选择。

## 3. 安装与运行

预览安装文件和迁移计划：

```sh
go run ./cmd/admin module update notes --dry-run
```

执行 update，将模块接入应用并执行数据库迁移：

```sh
go run ./cmd/admin module update notes
```

查询模块状态：

```sh
go run ./cmd/admin module status notes
```

状态输出包括 `registered=true`、已执行安装快照和待执行迁移数量。

启动后端，命令在当前终端持续运行：

```sh
go run ./cmd/server
```

若前端开发服务已经在 http://localhost:5173 运行，保持该终端，刷新页面即可。尚未启动时，另开终端进入仓库根目录：

```sh
cd frontend
npm run dev
```

这条命令会先构建公共包，再启动自带应用。打开 http://localhost:5173，用管理员登录，访问 notes 下的笔记列表。新建一条笔记并刷新，确认数据保留。

后续开发时：

- 另开一个终端，进入 `backend/`，执行后面的命令。
- 修改后端代码或更新已安装模块后，需要重启后端：在运行后端的终端按 `Ctrl+C`，再执行 `go run ./cmd/server`。
- 前端保持 `npm run dev` 运行，修改后的页面会自动更新。

## 4. 新增第二个资源

创建迁移文件，并填写 SQL。

### PostgreSQL

创建迁移文件：

```sh
"$(go env GOPATH)/bin/goose" -dir migrations/postgres create notes_tag sql
```

填写 SQL：

```sql
-- +goose Up
CREATE TABLE notes_tag (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  update_time TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose Down
DROP TABLE notes_tag;
```

### MySQL

创建迁移文件：

```sh
"$(go env GOPATH)/bin/goose" -dir migrations/mysql create notes_tag sql
```

填写 SQL：

```sql
-- +goose Up
CREATE TABLE notes_tag (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  title VARCHAR(100) NOT NULL,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;
-- +goose Down
DROP TABLE notes_tag;
```

### 继续生成资源

执行迁移，创建标签表：

```sh
go run ./cmd/migrate
```

使用相同入口创建 `internal/modules/notes/resources/tag.json`：

```sh
go run ./cmd/admin module resource notes notes_tag --title 标签
```

打开生成的 JSON，将 `title` 字段的 `label` 改为 `标题`，并按业务需要设置 `required` 等规则。

生成两个资源的代码：

```sh
go run ./cmd/admin module generate notes
```

更新已安装模块的菜单和权限：

```sh
go run ./cmd/admin module update notes
```

生成后，模块中会新增 Tag 接口、服务注册代码、标签页面和标签操作权限。

已有的笔记功能会保留，包括 `note.go` 中的 Hook/Overrides 配置，以及 `NoteList.tsx` 中的自定义内容。

重启后端后，在页面中确认：

1. 打开标签列表，新增一个标签。刷新页面后，标签仍在列表中。
2. 打开笔记列表，能查到第 3 节保存的笔记。

## 5. 增加字段

创建迁移，为笔记增加发布状态列。

### PostgreSQL

创建迁移文件：

```sh
"$(go env GOPATH)/bin/goose" -dir migrations/postgres create notes_note_published sql
```

填写 SQL：

```sql
-- +goose Up
ALTER TABLE notes_note ADD COLUMN published BOOLEAN;
-- +goose Down
ALTER TABLE notes_note DROP COLUMN published;
```

### MySQL

创建迁移文件：

```sh
"$(go env GOPATH)/bin/goose" -dir migrations/mysql create notes_note_published sql
```

填写 SQL：

```sql
-- +goose Up
ALTER TABLE notes_note ADD COLUMN published BOOLEAN;
-- +goose Down
ALTER TABLE notes_note DROP COLUMN published;
```

### 继续生成字段

执行迁移，增加发布状态列：

```sh
go run ./cmd/migrate
```

在 `internal/modules/notes/resources/note.json` 的 `fields` 中增加这一项：

```json
{ "column": "published", "label": "发布" }
```

根据修改后的配置重新生成代码：

```sh
go run ./cmd/admin module generate notes
```

重启后端，在笔记页面确认发布状态可以保存：

1. 将一条笔记的发布状态设为 `true`，保存并刷新，确认仍为 `true`。
2. 将发布状态改为 `false`，保存并刷新，确认仍为 `false`。
3. 只修改标题，保存后确认发布状态保持原值。

更新笔记时，后端根据请求中是否包含 `published` 决定如何处理：

- 只提交标题：保留原来的发布状态。
- 提交 `published: false`：将发布状态改为 `false`。

Proto 中的 `optional` 用来区分这两种情况。

重新生成后，`title` 在 Proto 中的字段编号保持原值，新增的 `published` 分配一个新编号。

本次增加字段会保留已有菜单和权限。查看菜单、权限等配置是否有待更新的差异：

```sh
go run ./cmd/admin module status notes
```

需要将这些变化写入数据库时，执行：

```sh
go run ./cmd/admin module update notes
```

**删除字段**

例如，从 `note.json` 中删除 `published`，再执行生成命令，这个字段就会从生成的接口中移除。

它原来的编号和名称会保留在 Proto 的 `reserved` 中，表示这两个位置已经用过。以后增加字段时，需要使用新的编号和名称。

**修改字段类型或 NULL 设置**

下面这两类改动会让生成器报错：

- 改变字段类型，例如把字符串改为整数。
- 改变字段是否允许存储 NULL，例如给已有列加上 `NOT NULL`。

这类改动需要提供新版本的接口，例如从 `v1` 增加到 `v2`，让调用方按新接口调整代码。

## 6. 添加业务逻辑与修改菜单

首次执行 `module generate notes` 时，生成器根据资源配置中的 `"name": "Note"` 创建 `internal/modules/notes/note.go`，其中包含 `configureNote` 函数，供你添加业务逻辑。这个文件创建后由你维护，后续生成会保留已有内容。

### 保存前处理标题

下面给新增笔记加一条规则：保存前去掉标题首尾的空格。在 `internal/modules/notes/note.go` 中写入：

```go
package notes

import (
    "context"
    "strings"

    gen "github.com/abysstract/abysstract-admin/backend/internal/modules/notes/gen/resource"
    "github.com/abysstract/abysstract-admin/backend/internal/modules/notes/model"
    "github.com/abysstract/abysstract-admin/backend/module"
    "github.com/abysstract/abysstract-admin/backend/resource"
)

func configureNote(rt module.Runtime, deps gen.NoteControllerDeps) gen.NoteControllerDeps {
    deps.Hooks = &resource.HookSet[model.NotesNote]{
        BeforeCreate: func(_ context.Context, _ resource.Tx, note *model.NotesNote) error {
            note.Title = strings.TrimSpace(note.Title)
            return nil
        },
    }
    return deps
}
```

重启后端，新建一条笔记：

1. 输入标题时，先按两次空格键，再输入“测试笔记”，最后再按两次空格键。
2. 保存笔记并刷新页面。
3. 确认标题显示为“测试笔记”。前后各两个空格已被 Hook 去掉。

### 添加页面操作按钮

下面在笔记页面右上角添加“填写说明”按钮，点击后显示提示。这是页面手写改动，不修改 `note.json`，也不经过 `generate`。后续生成会保留这个页面文件。

在 `frontend/apps/reference/src/modules/notes/pages/NoteList.tsx` 中增加导入：

```tsx
import { Button } from "antd";
```

将 `<PageContainer title={"笔记"}>` 这一行替换为以下内容，其中 `message` 已在生成的页面中定义：

```tsx
<PageContainer
  title="笔记"
  extra={
    <Button onClick={() => message.info("填写笔记标题后，点击保存。") }>
      填写说明
    </Button>
  }
>
```

保留其中的 `ResourceTable` 和结尾的 `</PageContainer>`。回到笔记页面，点击“填写说明”，确认出现提示。

### 添加独立检查接口

下面增加一个独立的 `InspectNote` 接口，通过 `deps.Overrides` 提供实现。`generate` 会生成服务方法、TypeScript 客户端和 `biz:notes:note:inspect` 权限；列表页的标准工具栏不会出现这个按钮，需要在页面里调用生成的客户端。Hook 和独立接口可按业务需要分别使用。本例通过返回输入标记，确认手写方法已经接入。

先创建请求和响应定义文件 `internal/modules/notes/proto/company/notes/v1/note_actions.proto`：

```proto
syntax = "proto3";
package company.notes.v1;
message InspectNoteRequest { string marker = 1; }
message InspectNoteResponse { string marker = 1; }
```

为 Note 增加 `InspectNote` 接口，让生成器生成对应的服务方法和 `inspect` 权限。在 `internal/modules/notes/resources/note.json` 顶层增加 `operations` 配置：

```json
"operations": [
  {
    "method": "InspectNote",
    "request": "InspectNoteRequest",
    "response": "InspectNoteResponse",
    "action": "inspect",
    "title": "检查笔记",
    "proto": "company/notes/v1/note_actions.proto",
    "scope": "exempt"
  }
]
```

生成新增操作的消息类型、服务方法、TypeScript 客户端和权限代码：

```sh
go run ./cmd/admin module generate notes
```

生成命令创建了接口和调用代码。接下来将 `internal/modules/notes/note.go` 的内容更新为以下代码，提供检查接口的实际实现。

其中 `deps.Overrides = noteOperations{}` 把实现交给 Controller，前面的标题处理也保留在这个函数中。完成这一步后再重启后端：

```go
package notes

import (
    "context"
    "strings"

    api "github.com/abysstract/abysstract-admin/backend/internal/modules/notes/gen/proto/company/notes/v1"
    gen "github.com/abysstract/abysstract-admin/backend/internal/modules/notes/gen/resource"
    "github.com/abysstract/abysstract-admin/backend/internal/modules/notes/model"
    "github.com/abysstract/abysstract-admin/backend/module"
    "github.com/abysstract/abysstract-admin/backend/resource"
)

func configureNote(rt module.Runtime, deps gen.NoteControllerDeps) gen.NoteControllerDeps {
    deps.Hooks = &resource.HookSet[model.NotesNote]{
        BeforeCreate: func(_ context.Context, _ resource.Tx, note *model.NotesNote) error {
            note.Title = strings.TrimSpace(note.Title)
            return nil
        },
    }
    deps.Overrides = noteOperations{}
    return deps
}

type noteOperations struct{}

func (noteOperations) InspectNote(_ context.Context, req *api.InspectNoteRequest) (*api.InspectNoteResponse, error) {
    return &api.InspectNoteResponse{Marker: req.Marker + "-override"}, nil
}
```

请求传入 `marker: "test"` 时，响应返回 `marker: "test-override"`。

`scope: "exempt"` 表示框架将数据范围检查交给这个方法的实现。数据范围决定用户能访问哪些业务记录，例如本部门的笔记。本例只处理传入的字符串，不读写业务记录；接口仍会校验调用者是否拥有 `biz:notes:note:inspect` 权限。

在 `NoteList.tsx` 的 `extra` 中同时放上“填写说明”和“检查笔记”。页面里已有 `operations`、`message` 和 `formatErrorMessage`：

```tsx
extra={
  <>
    <Button onClick={() => message.info("填写笔记标题后，点击保存。")}>
      填写说明
    </Button>
    <Button
      onClick={() => {
        void operations.custom.inspectNote({ marker: "test" }).then(
          (result) => message.success(result.marker),
          (error) => message.error(formatErrorMessage(error)),
        );
      }}
    >
      检查笔记
    </Button>
  </>
}
```

将新增的“检查笔记”权限写入数据库：

```sh
go run ./cmd/admin module update notes
```

在运行后端的终端按 `Ctrl+C`，再执行 `go run ./cmd/server`。

回到笔记页面，点击“检查笔记”，应看到 `test-override`。

后续重新生成代码时，`note.go` 中的手写实现和页面上添加的按钮都会保留。

自定义请求、独立服务和多表列表的步骤见[自定义请求与多表列表](custom-module-api.md)。

### 修改菜单和权限

编辑 `seed.json` 调整模块菜单、权限或示例角色后，查询当前状态：

```sh
go run ./cmd/admin module status notes
```

预览菜单权限的更新计划：

```sh
go run ./cmd/admin module update notes --dry-run
```

执行更新：

```sh
go run ./cmd/admin module update notes
```

升级保留数据库管理员对默认字段的修改，并清理退役菜单及授权。详见 [模块指南](module-guide.md)。

## 7. 卸载与恢复

卸载模块，移除应用加载和公共生成任务，清理它创建的菜单和示例角色，保留业务表和数据：

```sh
go run ./cmd/admin module uninstall notes
```

查询卸载状态：

```sh
go run ./cmd/admin module status notes
```

源码可以继续留在仓库。下面演示移除源码后再恢复：停止后端，将模块源码移到临时目录，供后面恢复使用。以下命令继续在 `backend/` 下执行，`notes_archive` 保存临时目录路径：

```sh
notes_archive=$(mktemp -d)
mv internal/modules/notes "$notes_archive/backend-notes"
mv ../frontend/apps/reference/src/modules/notes "$notes_archive/frontend-notes"
```

执行 `go run ./cmd/server` 启动后端，前端保持开发服务运行。登录后系统管理可用，notes 菜单消失。应用迁移目录的历史文件继续保留，新库仍能执行完整历史。

停止后端，在归档时的终端恢复源码：

```sh
mv "$notes_archive/backend-notes" internal/modules/notes
mv "$notes_archive/frontend-notes" ../frontend/apps/reference/src/modules/notes
```

重新生成模块代码：

```sh
go run ./cmd/admin module generate notes
```

执行 update，重新安装模块：

```sh
go run ./cmd/admin module update notes
```

检查已安装模块的加载代码、公共生成清单和 Buf 配置是否与 `admin.modules.json` 一致：

```sh
go run ./cmd/admin module check
```

执行 `go run ./cmd/server` 启动后端，回到前端页面确认原笔记和标签记录可查询。此前随卸载清理的模块角色与授权按当前需求重新配置。

## 8. 提交前检查

检查后端编译、代码和测试：

```sh
go build ./...
go vet ./...
go test ./...
```

检查已安装模块的加载代码、公共生成清单和 Buf 配置是否与 `admin.modules.json` 一致：

```sh
go run ./cmd/admin module check
```

检查前端类型并运行测试：

```sh
( cd ../frontend && npm run typecheck && npm test )
```

检查前端生产构建：

```sh
( cd ../frontend && npm run build )
```

提交业务迁移、CRUD 配置、手写代码及生成文件。模块外文件影响见[模块指南](module-guide.md#模块外哪些文件会变化)。
