import { useCallback, useMemo } from "react";
import { useAdminHost } from "../runtime/context";
import { useSystemClients } from "../system/hooks";
import {
  createSystemPresign,
  directUpload,
  resolveApiUrl,
  type DirectUploadOptions,
  type DirectUploadResult,
  type PresignUpload,
  type SendUpload,
} from "./directUpload";

export interface UseDirectUploadOptions {
  /** 自定义预签名接口；默认使用系统 UploadService.CreateUpload。 */
  readonly presign?: PresignUpload;
  /** 测试或特殊传输时替换上传发送实现。 */
  readonly send?: SendUpload;
}

export interface DirectUploader {
  upload(file: File, options?: DirectUploadOptions): Promise<DirectUploadResult>;
  /** 把服务端返回的相对 URL 解析为基于 apiBaseUrl 的绝对地址。 */
  resolveUrl(url: string): string;
}

/** 预签名直传：文件不经过后端，直接 PUT 到存储（本地驱动或 S3）。 */
export function useDirectUpload(options: UseDirectUploadOptions = {}): DirectUploader {
  const { apiBaseUrl } = useAdminHost();
  const { upload: uploadClient } = useSystemClients();
  const { presign: customPresign, send } = options;
  const presign = useMemo(
    () => customPresign ?? createSystemPresign(uploadClient),
    [customPresign, uploadClient],
  );
  const upload = useCallback(
    (file: File, uploadOptions?: DirectUploadOptions) =>
      directUpload(file, { presign, apiBaseUrl, send }, uploadOptions),
    [presign, apiBaseUrl, send],
  );
  const resolveUrl = useCallback((url: string) => resolveApiUrl(url, apiBaseUrl), [apiBaseUrl]);
  return useMemo(() => ({ upload, resolveUrl }), [upload, resolveUrl]);
}
