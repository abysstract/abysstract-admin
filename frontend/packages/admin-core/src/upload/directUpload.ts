import type { Client } from "@connectrpc/connect";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { UploadService } from "@abysstract/admin-proto/abysstract/admin/v1/upload_pb";

/** 预签名直传信息：客户端按 method / headers 把文件发到 uploadUrl。 */
export interface PresignedUpload {
  /** 存储对象键；上传成功后提交给业务接口保存。 */
  readonly key: string;
  /** 上传完成后的访问 URL（可能是相对 API 源站的路径）。 */
  readonly url: string;
  /** 上传地址（本地存储驱动返回相对 API 源站的路径）。 */
  readonly uploadUrl: string;
  readonly method: string;
  /** 上传时必须原样携带的请求头。 */
  readonly headers: Readonly<Record<string, string>>;
  readonly expiresAt?: Date;
}

/**
 * 申请预签名上传。默认使用系统的 UploadService.CreateUpload；
 * 业务可传入自己的实现（例如按内容哈希生成对象键的业务接口）。
 */
export type PresignUpload = (
  file: File,
  context: { readonly signal?: AbortSignal },
) => Promise<PresignedUpload>;

export interface DirectUploadResult {
  readonly key: string;
  /** 已解析为绝对地址的访问 URL。 */
  readonly url: string;
  readonly size: number;
}

export interface DirectUploadOptions {
  readonly signal?: AbortSignal;
  readonly onProgress?: (loaded: number, total: number) => void;
}

/** 把文件发到预签名地址；默认实现基于 XMLHttpRequest 以支持上传进度。 */
export type SendUpload = (
  target: {
    readonly url: string;
    readonly method: string;
    readonly headers: Readonly<Record<string, string>>;
  },
  file: Blob,
  options: DirectUploadOptions,
) => Promise<void>;

export class DirectUploadError extends Error {
  readonly status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "DirectUploadError";
    this.status = status;
  }
}

/**
 * 把以 "/" 开头的相对地址拼到 API 基础地址之后（保留 apiBaseUrl 的路径前缀）；
 * 绝对地址与其他值原样返回。
 */
export function resolveApiUrl(url: string, apiBaseUrl: string): string {
  if (!url.startsWith("/") || url.startsWith("//")) return url;
  const base = apiBaseUrl.endsWith("/") ? apiBaseUrl.slice(0, -1) : apiBaseUrl;
  return base + url;
}

/** 基于系统 UploadService.CreateUpload 的预签名实现。 */
export function createSystemPresign(client: Client<typeof UploadService>): PresignUpload {
  return async (file, { signal }) => {
    const res = await client.createUpload(
      { filename: file.name, contentType: file.type, size: BigInt(file.size) },
      { signal },
    );
    return {
      key: res.key,
      url: res.url,
      uploadUrl: res.uploadUrl,
      method: res.method || "PUT",
      headers: res.headers,
      expiresAt: res.expiresAt ? timestampDate(res.expiresAt) : undefined,
    };
  };
}

function abortError(signal: AbortSignal): unknown {
  if (signal.reason !== undefined) return signal.reason;
  const error = new Error("This operation was aborted");
  error.name = "AbortError";
  return error;
}

/** 默认的上传发送实现。不携带 Cookie 与 Authorization，避免把登录凭据发给存储服务。 */
export const sendWithXhr: SendUpload = (target, file, { signal, onProgress }) =>
  new Promise<void>((resolve, reject) => {
    if (signal?.aborted) {
      reject(abortError(signal));
      return;
    }
    const xhr = new XMLHttpRequest();
    xhr.open(target.method, target.url);
    for (const [name, value] of Object.entries(target.headers)) {
      xhr.setRequestHeader(name, value);
    }
    const onAbort = () => xhr.abort();
    signal?.addEventListener("abort", onAbort, { once: true });
    const done = () => signal?.removeEventListener("abort", onAbort);
    if (onProgress) {
      xhr.upload.onprogress = (e) => onProgress(e.loaded, e.lengthComputable ? e.total : file.size);
    }
    xhr.onload = () => {
      done();
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve();
      } else {
        reject(new DirectUploadError(`上传失败（HTTP ${xhr.status}）`, xhr.status));
      }
    };
    xhr.onerror = () => {
      done();
      reject(new DirectUploadError("上传失败：网络错误或存储服务拒绝跨域请求", 0));
    };
    xhr.onabort = () => {
      done();
      reject(signal ? abortError(signal) : new DirectUploadError("上传已取消", 0));
    };
    xhr.send(file);
  });

export interface DirectUploadDeps {
  readonly presign: PresignUpload;
  readonly apiBaseUrl: string;
  readonly send?: SendUpload;
}

/** 预签名 → 直传 → 返回对象键与访问 URL。 */
export async function directUpload(
  file: File,
  deps: DirectUploadDeps,
  options: DirectUploadOptions = {},
): Promise<DirectUploadResult> {
  const presigned = await deps.presign(file, { signal: options.signal });
  const send = deps.send ?? sendWithXhr;
  await send(
    {
      url: resolveApiUrl(presigned.uploadUrl, deps.apiBaseUrl),
      method: presigned.method,
      headers: presigned.headers,
    },
    file,
    options,
  );
  return {
    key: presigned.key,
    url: resolveApiUrl(presigned.url, deps.apiBaseUrl),
    size: file.size,
  };
}
