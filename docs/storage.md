# 文件存储

后端通过 `backend/storage` 的 `storage.Storage` 读写文件。默认使用本地目录，可切换为 S3 或 S3 兼容服务（MinIO、Cloudflare R2 等），也可以由应用注入自己的实现。

上传有两种方式：

- **经后端上传**：`UploadFile` / `UploadAvatar` 把文件内容放在请求体中，后端写入存储。
- **预签名直传**：`CreateUpload` 返回一个短时有效的上传地址，浏览器把文件直接 `PUT` 到存储。上传方拿不到存储凭据，文件也不经过后端。

两种方式都返回**对象键**（key），例如 `files/2026/11/3f9c…a1.png`。业务表保存 key，输出时用 `Storage.URL(key)` 解析为访问地址。更换域名或接入 CDN 时只改配置，不改数据。

## 1. 使用默认的本地存储

不需要额外配置。文件保存在 `ADMIN_UPLOAD_DIR`（默认 `./uploads`），由后端在 `/static/` 下提供下载，并接收带签名的 `PUT` 上传。

启动后端后，按下面的步骤走一遍直传流程。

登录并取得令牌：

```bash
TOKEN=$(curl -s http://localhost:8080/abysstract.admin.v1.AuthService/Login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')
```

申请上传地址。`size` 是文件的准确字节数：

```bash
printf 'hello' > /tmp/hello.txt
curl -s http://localhost:8080/abysstract.admin.v1.UploadService/CreateUpload \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" \
  -d '{"filename":"hello.txt","size":"5"}'
```

返回内容类似：

```json
{
  "key": "files/2026/11/0c5e…9d.txt",
  "url": "/static/files/2026/11/0c5e…9d.txt",
  "uploadUrl": "/static/files/2026/11/0c5e…9d.txt?X-Expires=…&X-Signature=…&X-Size=5",
  "method": "PUT",
  "headers": { "Content-Type": "text/plain; charset=utf-8" },
  "expiresAt": "2026-11-08T12:15:00Z"
}
```

按 `method` 和 `headers` 上传文件，再下载确认：

```bash
curl -i -X PUT "http://localhost:8080<uploadUrl>" \
  -H 'Content-Type: text/plain; charset=utf-8' --data-binary @/tmp/hello.txt
curl http://localhost:8080<url>
```

`PUT` 返回 `200`，下载得到 `hello`。签名覆盖对象键、`Content-Type`、文件大小和过期时间；改动其中任何一项，或在过期后上传，后端返回 `403`；请求体长度与签名不一致时返回 `400`。

本地驱动的签名密钥默认由 `ADMIN_JWT_SECRET` 派生，轮换 JWT 密钥后，未使用的上传地址随之失效。需要独立密钥时设置 `ADMIN_STORAGE_SIGNING_SECRET`（至少 16 字节）。

## 2. 切换到 S3 或兼容服务

设置以下变量后重启后端：

```dotenv
ADMIN_STORAGE_DRIVER=s3
ADMIN_STORAGE_S3_BUCKET=media
ADMIN_STORAGE_S3_REGION=us-east-1
# S3 兼容服务填写地址；MinIO 通常还需要路径风格寻址
ADMIN_STORAGE_S3_ENDPOINT=http://localhost:9000
ADMIN_STORAGE_S3_PATH_STYLE=true
# 文件的公开访问前缀（CDN、自定义域名或 R2 公开地址）
ADMIN_STORAGE_PUBLIC_BASE_URL=https://img.example.com/
```

凭据通过 AWS 标准凭据链读取：`AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` 环境变量、共享配置文件，或实例、容器角色。凭据只在后端使用，`CreateUpload` 返回的是 S3 预签名地址。

`ADMIN_STORAGE_PUBLIC_BASE_URL` 留空时，访问地址按存储服务地址推导：设置了 endpoint 时为 `<endpoint>/<bucket>/`（路径风格）或 `<scheme>://<bucket>.<host>/`；未设置时为 AWS 的桶地址。R2 的 S3 接口地址不能公开访问，需要填写 R2 公开域名。

浏览器直接向存储服务发送 `PUT`，存储桶需要允许后台前端的来源跨域上传。以 S3 的 CORS 配置为例：

```json
[
  {
    "AllowedOrigins": ["https://admin.example.com"],
    "AllowedMethods": ["PUT", "GET"],
    "AllowedHeaders": ["Content-Type"],
    "MaxAgeSeconds": 3600
  }
]
```

S3 预签名同样签入 `Content-Type` 和 `Content-Length`，存储服务拒绝类型或大小不一致的上传。

切换驱动后，`/static/` 不再由后端提供。已经保存的本地文件需要先复制到存储桶，保持相同的相对路径作为 key。

## 3. 配置项

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `ADMIN_STORAGE_DRIVER` | `local`、`s3`，或应用注册的驱动名 | `local` |
| `ADMIN_STORAGE_PUBLIC_BASE_URL` | 访问 URL 前缀：`http(s)://` 地址或以 `/` 开头的路径 | 本地为 `/static/`；S3 按桶地址推导 |
| `ADMIN_STORAGE_UPLOAD_TTL_SECONDS` | 预签名上传有效期，1–604800 秒 | `900` |
| `ADMIN_STORAGE_SIGNING_SECRET` | 本地驱动的签名密钥，至少 16 字节 | 由 `ADMIN_JWT_SECRET` 派生 |
| `ADMIN_UPLOAD_DIR` | 本地驱动的根目录 | `./uploads` |
| `ADMIN_STORAGE_S3_BUCKET` | 存储桶，`s3` 驱动必填 | 空 |
| `ADMIN_STORAGE_S3_REGION` | 区域，`s3` 驱动必填；R2 使用 `auto` | 空 |
| `ADMIN_STORAGE_S3_ENDPOINT` | S3 兼容服务地址 | 空 |
| `ADMIN_STORAGE_S3_PATH_STYLE` | `true` 时使用 `endpoint/bucket/key` 寻址 | `false` |

`ADMIN_UPLOAD_MAX_BYTES` 与 `ADMIN_AVATAR_MAX_BYTES` 同时约束经后端上传和预签名直传。启动时校验上述配置，`s3` 驱动缺少存储桶或区域时拒绝启动。

## 4. 前端表单中上传图片

`@abysstract/admin-core/upload` 提供图片表单项 `DirectImageUpload`。它完成“申请地址 → 直传 → 回填 key”，表单值是对象键：

```tsx
import { Form } from "antd";
import { DirectImageUpload } from "@abysstract/admin-core/upload";

<Form.Item name="coverKey" label="封面">
  <DirectImageUpload maxBytes={5 * 1024 * 1024} resolveUrl={(key) => record?.coverUrl} />
</Form.Item>;
```

- `resolveUrl` 把已保存的 key 解析为预览地址，通常直接使用接口随记录返回的 URL。
- 本地驱动返回的相对地址会拼接到 `host.apiBaseUrl` 之后。
- 上传请求不携带登录令牌和 Cookie。

其他文件或自定义界面使用 `useDirectUpload`：

```tsx
import { useDirectUpload } from "@abysstract/admin-core/upload";

const { upload } = useDirectUpload();
const { key, url } = await upload(file, { onProgress: (loaded, total) => setPercent(loaded / total) });
```

业务有自己的预签名接口时，把它包装成 `presign` 传入组件或 Hook：

```tsx
<DirectImageUpload presign={(file, { signal }) => myClient.presignImage({ filename: file.name, size: BigInt(file.size) }, { signal })} />
```

## 5. 在业务模块中使用存储

模块在 `Create` 中从 `module.Runtime.Storage` 取得应用共享的存储实例，保存到服务结构体（下例的 `s.store`）。业务接口按自己的规则生成 key，用 `storage.Presign` 校验并签发上传地址。以下示例中的 `v1.PresignRequest` 等类型来自业务自己的 Proto：

```go
import "github.com/abysstract/abysstract-admin/backend/storage"

var (
	imagePolicy = storage.UploadPolicy{MaxBytes: 10 << 20, AllowedExts: storage.ImageExts()}
	sha256Hex   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func (s *ImageService) Presign(ctx context.Context, req *v1.PresignRequest) (*v1.PresignResponse, error) {
	// 按内容哈希生成 key，相同图片复用同一对象。
	if !sha256Hex.MatchString(req.Sha256) {
		return nil, connect.NewError(connect.CodeInvalidArgument, "sha256 格式错误")
	}
	key, err := storage.JoinKey("images", req.Sha256[:2], req.Sha256+storage.Ext(req.Filename))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	res, err := storage.Presign(ctx, s.store, imagePolicy, storage.PresignRequest{
		Key: key, Filename: req.Filename, Size: req.Size,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	return &v1.PresignResponse{
		Key: res.Key, Url: res.URL, UploadUrl: res.Upload.URL,
		Method: res.Upload.Method, Headers: res.Upload.Headers,
	}, nil
}
```

客户端上传完成后提交 key。保存前用 `storage.VerifyUpload` 确认对象已经存在且大小一致：

```go
if _, err := storage.VerifyUpload(ctx, s.store, req.ImageKey, req.Size); err != nil {
	return nil, connect.NewError(connect.CodeFailedPrecondition, "图片尚未上传").WithCause(err)
}
```

`VerifyUpload` 检查对象是否存在及其大小，不校验内容哈希。需要校验内容时，由业务读取对象自行计算。

| 函数 | 用途 |
| --- | --- |
| `storage.ValidateKey` | 拒绝空 key、绝对路径、`\`、控制字符、空段和以 `.` 开头的段（含 `..`） |
| `storage.JoinKey` | 拼接并校验 key |
| `storage.RandomKey` | 生成 `<prefix>/<32 位随机 hex><扩展名>` |
| `storage.UploadPolicy` | 大小与扩展名白名单，`Check` 返回 `ErrEmpty`、`ErrTooLarge`、`ErrExtNotAllowed` |
| `storage.ContentTypeByExt` | 按扩展名推断 MIME，不信任客户端声明 |
| `storage.Presign` | 校验后签发预签名上传 |
| `storage.VerifyUpload` | 上传后确认对象，缺失时返回 `ErrNotFound` |

## 6. 注入自定义存储

应用需要在后台之外复用同一实例（例如定时任务）时，自己打开存储并通过 `adminapp.Options.Storage` 传入：

```go
import (
	"github.com/abysstract/abysstract-admin/backend/adminapp"
	_ "github.com/abysstract/abysstract-admin/backend/storage/s3storage"
)

objects, err := adminapp.OpenStorage(ctx, *cfg)
if err != nil {
	log.Fatal(err)
}
server, err := adminapp.New(adminapp.Options{Config: *cfg, Modules: modules, Storage: objects})
```

传入的实例由应用管理，后台关闭时不会关闭它。未传入时，`Start` 按配置打开存储并在关闭时释放。

驱动按需编译：`localstorage` 始终可用；`s3storage` 只有被导入时才会编译 AWS SDK 并注册 `s3` 驱动。自带后台在 `backend/internal/app/app.go` 中导入了它；独立应用使用 `ADMIN_STORAGE_DRIVER=s3` 时，在 `main.go` 中加入同样的空白导入。

实现 `storage.Storage` 接口并调用 `storage.Register("name", factory)` 即可接入其他存储，再用 `ADMIN_STORAGE_DRIVER=name` 选择。

## 安全说明

- 文件可公开读取：持有 URL 的人都可以下载。本地驱动关闭目录列表，并通过 `X-Content-Type-Options: nosniff` 和 CSP 沙箱阻止上传的 HTML、SVG 在后台域名下执行脚本；PDF 不加沙箱，以便浏览器内置阅读器显示。使用 S3 时，按需在存储桶或 CDN 上设置同样的响应头。
- 上传地址只对签发时的 key、类型和大小有效，并在有效期后失效。有效期内可以重复上传同一 key。
- `CreateUpload` 和 `UploadFile` 都要求登录，使用相同的大小上限和扩展名白名单；内容类型由服务端按扩展名决定。
- 头像仍把访问 URL 写入 `sys_user.avatar`，与之前版本一致。

本版本不包含文件登记表和存储配置页面。它们可以作为可选模块在之后加入：对象键与 URL 解析方式保持不变，已保存的数据不需要迁移。
