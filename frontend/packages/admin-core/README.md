# @abysstract/admin-core

公共前端包。根入口提供应用创建与认证存储，`/resource` 提供 CRUD 表格、表单和类型，`/system` 提供系统模块、客户端与 operations hooks。

```ts
import {
  bindAdminAppPageLifecycle,
  createAdminApp,
  createLocalAuthStorage,
} from "@abysstract/admin-core";
import { ResourceTable } from "@abysstract/admin-core/resource";
import { useSystemOperations } from "@abysstract/admin-core/system";
import "@abysstract/admin-core/styles.css";
```

宿主入口挂载 React 后，调用 `bindAdminAppPageLifecycle({ application, root })`，管理浏览器前进后退缓存与页面离开时的资源释放。

安装方式见[公共包用法](https://github.com/abysstract/abysstract-admin/blob/main/docs/admin-core-usage.md#安装前端包)，页面、权限和字段配置见[业务接入教程](https://github.com/abysstract/abysstract-admin/blob/main/docs/admin-core-getting-started.md)。
