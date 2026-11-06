# @abysstract/admin-proto

abysstract-admin 的公共协议包：系统 Protobuf-ES 消息与 Connect 服务描述，提供 ESM 与 TypeScript 声明。

```bash
npm install @abysstract/admin-proto @bufbuild/protobuf
```

```ts
import { PageRequestSchema } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";
import { create } from "@bufbuild/protobuf";
const page = create(PageRequestSchema, { pageNum: 1 });
```

系统消息描述由本包维护；业务协议生成时使用 `--consumer` 选择业务输入，生成的业务代码从本包的相同子路径导入系统消息。

源码与生成命令见 [abysstract-admin](https://github.com/abysstract/abysstract-admin)，安装方式见[公共包用法](https://github.com/abysstract/abysstract-admin/blob/main/docs/admin-core-usage.md#安装前端包)。
