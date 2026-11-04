# 自定义请求与多表列表

同一个模块可以同时提供自动生成的 CRUD 和手写接口。沿用[笔记模块教程](new-module-tutorial.md)的目录，后端文件放在 `backend/internal/modules/notes/`，前端文件放在 `frontend/apps/reference/src/modules/notes/`。以下命令在 `backend/` 下执行。

## 给现有 CRUD 添加操作

例如给笔记增加检查、批量发布或自定义查询，可以通过 `resources/note.json` 的 `operations` 添加方法。请求和响应的字段由你定义。列表页的标准工具栏仍是新增、编辑和删除，新方法要在页面里调用生成的客户端。

1. 在模块的 `proto/company/notes/v1/` 下新建 Proto 文件，定义请求和响应消息。
2. 在 `note.json` 的 `operations` 中填写 `method`、`request`、`response`、`action`、`title`、`proto` 和 `scope: "exempt"`。权限标识为 `biz:<模块>:<资源>:<action>`。
3. 执行 `go run ./cmd/admin module generate notes`，生成消息类型、服务方法、TypeScript 客户端和权限代码。
4. 在手写 `note.go` 的 `configureNote` 中设置 `deps.Overrides`，由该对象实现新增方法。查询代码可单独放在 `note_query.go`，通过 `module.Runtime.DB` 使用数据库。
5. 在页面中调用 `operations.custom.<方法名>`，再执行 `module update notes` 把权限写入数据库。

完整代码见[添加业务逻辑](new-module-tutorial.md#6-添加业务逻辑与修改菜单)。框架检查登录和操作权限；`scope: "exempt"` 时，查询中的记录访问范围由业务代码处理。

标准 CRUD 的请求由生成器维护。需要额外的请求字段和返回结果时，按上面的步骤增加自己的方法。

## 添加独立查询服务

多表列表也可以使用独立服务。下面给笔记增加一个可选标签，列表同时显示笔记标题和标签名称，并支持按标签筛选。它与 Note、Tag 的 CRUD 放在同一个 notes 模块中。

本例接着笔记教程第 4 节操作，数据库已有 `notes_note` 和 `notes_tag`，模块已安装。SQL 示例沿用笔记教程的数据库选择方式。

### 1. 增加标签关联

#### PostgreSQL

创建迁移文件：

```sh
"$(go env GOPATH)/bin/goose" -dir migrations/postgres create notes_note_tag_id sql
```

填写 SQL：

```sql
-- +goose Up
ALTER TABLE notes_note ADD COLUMN tag_id BIGINT REFERENCES notes_tag(id);
-- +goose Down
ALTER TABLE notes_note DROP COLUMN tag_id;
```

#### MySQL

创建迁移文件：

```sh
"$(go env GOPATH)/bin/goose" -dir migrations/mysql create notes_note_tag_id sql
```

填写 SQL：

```sql
-- +goose Up
ALTER TABLE notes_note
  ADD COLUMN tag_id BIGINT,
  ADD CONSTRAINT fk_notes_note_tag FOREIGN KEY (tag_id) REFERENCES notes_tag(id);
-- +goose Down
ALTER TABLE notes_note DROP FOREIGN KEY fk_notes_note_tag;
ALTER TABLE notes_note DROP COLUMN tag_id;
```

#### 继续生成接口

执行迁移：

```sh
go run ./cmd/migrate
```

在 `internal/modules/notes/resources/note.json` 的 `fields` 中增加这一项，让笔记编辑表单能够填写标签 ID：

```json
{ "column": "tag_id", "label": "标签ID" }
```

### 2. 定义查询请求和返回结果

创建 `internal/modules/notes/proto/company/notes/v1/note_query.proto`：

```proto
syntax = "proto3";

package company.notes.v1;

service NoteQueryService {
  rpc ListNoteTags(ListNoteTagsRequest) returns (ListNoteTagsResponse);
}

message ListNoteTagsRequest {
  int32 page = 1;
  int32 page_size = 2;
  optional int64 tag_id = 3;
}

message NoteTagRow {
  int64 id = 1;
  string title = 2;
  string tag_title = 3;
}

message ListNoteTagsResponse {
  repeated NoteTagRow items = 1;
  int64 total = 2;
}
```

`tag_id` 省略时查询全部笔记，填写时查询该标签下的笔记。`NoteTagRow` 包含两张表的数据。

生成更新后的 CRUD 和这个 Proto 的 Go、TypeScript 类型：

```sh
go run ./cmd/admin module generate notes
```

### 3. 编写多表查询

创建 `internal/modules/notes/note_query.go`：

```go
package notes

import (
    "context"
    "errors"

    "connectrpc.com/connect/v2"
    "gorm.io/gorm"

    api "github.com/abysstract/abysstract-admin/backend/internal/modules/notes/gen/proto/company/notes/v1"
)

type noteQueryService struct {
    db *gorm.DB
}

func (s *noteQueryService) ListNoteTags(ctx context.Context, req *api.ListNoteTagsRequest) (*api.ListNoteTagsResponse, error) {
    if req.GetPage() < 1 || req.GetPageSize() < 1 || req.GetPageSize() > 100 {
        return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("页码从 1 开始，每页数量为 1 到 100"))
    }
    if req.TagId != nil && req.GetTagId() < 1 {
        return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("标签ID应大于 0"))
    }

    q := s.db.WithContext(ctx).
        Table("notes_note AS n").
        Joins("LEFT JOIN notes_tag AS t ON t.id = n.tag_id")
    if req.TagId != nil {
        q = q.Where("n.tag_id = ?", req.GetTagId())
    }

    var total int64
    if err := q.Count(&total).Error; err != nil {
        return nil, err
    }

    var rows []struct {
        ID       int64
        Title    string
        TagTitle string
    }
    offset := (int64(req.GetPage()) - 1) * int64(req.GetPageSize())
    if err := q.Select("n.id, n.title, COALESCE(t.title, '') AS tag_title").
        Order("n.id ASC").Offset(int(offset)).Limit(int(req.GetPageSize())).
        Scan(&rows).Error; err != nil {
        return nil, err
    }

    items := make([]*api.NoteTagRow, 0, len(rows))
    for _, row := range rows {
        items = append(items, &api.NoteTagRow{
            Id: row.ID, Title: row.Title, TagTitle: row.TagTitle,
        })
    }
    return &api.ListNoteTagsResponse{Items: items, Total: total}, nil
}
```

这个查询使用 LEFT JOIN，因此没有标签的笔记也会显示，标签名称为空。本例的笔记和标签按模块权限共享；有部门、创建者等记录访问规则的业务，应在查询总数和列表之前添加相同的范围条件。商品模块的[服务创建代码](../backend/internal/modules/product/module.go)展示了如何传入数据范围提供者和创建校验，可作为参考。

### 4. 在模块中接入服务

在手写 `internal/modules/notes/module.go` 中接入查询服务，保留已有生成模块的服务、权限和创建过程：

```go
package notes

import (
    "context"

    "connectrpc.com/connect/v2"

    api "github.com/abysstract/abysstract-admin/backend/internal/modules/notes/gen/proto/company/notes/v1"
    apiconnect "github.com/abysstract/abysstract-admin/backend/internal/modules/notes/gen/proto/company/notes/v1/notesv1connect"
    "github.com/abysstract/abysstract-admin/backend/module"
    "github.com/abysstract/abysstract-admin/backend/permission"
)

func NewModule() (module.Definition, error) {
    def, err := generatedModule()
    if err != nil {
        return module.Definition{}, err
    }
    desc := api.File_company_notes_v1_note_query_proto.Services().ByName("NoteQueryService")
    def.Services = append(def.Services, module.ConnectService{Descriptor: desc})

    generatedPermissions := def.Permissions
    def.Permissions = func(r permission.Registrar) error {
        if err := generatedPermissions(r); err != nil {
            return err
        }
        return r.RegisterGenerated(permission.Entry{
            Procedure: "/company.notes.v1.NoteQueryService/ListNoteTags",
            Permission: "biz:notes:query:list",
        })
    }

    generatedCreate := def.Create
    def.Create = func(ctx context.Context, rt module.Runtime) (module.Instance, error) {
        instance, err := generatedCreate(ctx, rt)
        if err != nil {
            return module.Instance{}, err
        }
        handler := &noteQueryService{db: rt.DB}
        instance.Services = append(instance.Services, module.ConnectService{
            Descriptor: desc,
            Register: func(server *connect.Server) {
                apiconnect.RegisterNoteQueryServiceHandler(server, handler)
            },
        })
        return instance, nil
    }
    return def, nil
}
```

`Services` 说明模块提供哪些服务，`Permissions` 为接口配置权限，`Create` 创建实际处理请求的对象。应用沿用已有模块列表加载 notes。

### 5. 编写查询页面

创建 `frontend/apps/reference/src/modules/notes/pages/NoteTagList.tsx`：

```tsx
import { useMemo, useState } from "react";
import { Button, Table } from "antd";
import { PageContainer } from "@ant-design/pro-components";
import { createClient } from "@connectrpc/connect";
import {
  formatErrorMessage, useAccess, useAdminRuntime, useAppMessage,
} from "@abysstract/admin-core";
import { NoteQueryService, type NoteTagRow } from "../gen/proto/company/notes/v1/note_query_pb";

export default function NoteTagList() {
  const { transport } = useAdminRuntime();
  const client = useMemo(() => createClient(NoteQueryService, transport), [transport]);
  const { has } = useAccess();
  const message = useAppMessage();
  const [rows, setRows] = useState<NoteTagRow[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);

  async function load(pageNum: number) {
    try {
      const result = await client.listNoteTags({ page: pageNum, pageSize: 20 });
      setRows(result.items);
      setTotal(Number(result.total));
      setPage(pageNum);
    } catch (error) {
      message.error(formatErrorMessage(error));
    }
  }

  return (
    <PageContainer title="笔记与标签">
      <Button disabled={!has("biz:notes:query:list")} onClick={() => void load(1)}>
        查询
      </Button>
      <Table<NoteTagRow>
        rowKey={(row) => row.id.toString()}
        dataSource={rows}
        columns={[
          { title: "标题", dataIndex: "title" },
          { title: "标签", dataIndex: "tagTitle" },
        ]}
        pagination={{ current: page, pageSize: 20, total, showSizeChanger: false, onChange: (next) => void load(next) }}
      />
    </PageContainer>
  );
}
```

页面使用应用提供的 transport，沿用登录信息和错误处理。需要按标签查询时，在客户端请求中填写 `tagId`，例如 `tagId: 1n`；页面可按业务需要增加标签选择控件。

在手写 `frontend/apps/reference/src/modules/notes/index.tsx` 中添加页面，同时保留生成的页面：

```tsx
import { lazy } from "react";
import type { AdminFrontendModule } from "@abysstract/admin-core";
import { generatedPages } from "./pages.gen";

export const notesModule: AdminFrontendModule = {
  name: "notes",
  pages: {
    ...generatedPages,
    "biz/notes/tagged": lazy(() => import("./pages/NoteTagList")),
  },
};
```

### 6. 添加菜单并运行

在模块 `seed.json` 的 `menus` 数组中追加页面菜单：

```json
{
  "key": "notes.tagged",
  "parent": "notes.root",
  "name": "笔记与标签",
  "path": "tagged",
  "component": "biz/notes/tagged",
  "type": "C",
  "permission": "biz:notes:query:list"
}
```

更新菜单权限：

```sh
go run ./cmd/admin module update notes
```

在后端终端按 `Ctrl+C`，重新执行 `go run ./cmd/server`，前端保持 `npm run dev` 运行。在浏览器中用管理员打开标签列表，新建一个标签并记下 ID。编辑一条笔记，填写该标签 ID，再打开“笔记与标签”页面点击查询，应显示笔记标题和标签名称。普通角色需要分配对应目录、页面权限。

## 后续修改与重新生成

请求字段在手写 Proto 中修改，查询逻辑在 `note_query.go` 中修改，页面在 `NoteTagList.tsx` 中修改。Proto 修改后运行 `module generate notes` 更新类型，再重启后端并查看前端页面。新增服务方法时同步补充服务实现和接口权限。

`module.go`、`index.tsx` 和自定义页面由开发者维护。后续生成会更新 CRUD 代码、协议类型和生成的页面列表，保留上述手写文件。模块中原有的 Hook、Overrides 和 Note、Tag CRUD 继续使用。

提交前的编译、类型检查和测试见[笔记模块教程](new-module-tutorial.md#8-提交前检查)。
