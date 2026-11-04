# @abysstract/admin-proto

试用公共协议包（0.0.0-dev.0），ESM 与 TypeScript 声明。

```ts
import { PageRequestSchema } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";
import { create } from "@bufbuild/protobuf";
const page = create(PageRequestSchema, { pageNum: 1 });
```

从仓库根执行 `node frontend/tools/generate-proto.mjs`，随后在本目录执行
`npm run build` / `npm pack`。业务项目安装实际 tarball，并从相同子路径导入。
系统消息描述由本包维护；业务生成命令使用 `--consumer` 选择业务输入。
