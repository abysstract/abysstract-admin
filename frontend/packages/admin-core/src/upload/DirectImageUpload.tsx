import { useEffect, useRef, useState } from "react";
import { Button, Upload, type UploadProps } from "antd";
import { DeleteOutlined, LoadingOutlined, PlusOutlined } from "@ant-design/icons";
import { formatErrorMessage, useAppMessage } from "../runtime/message";
import type { DirectUploadResult } from "./directUpload";
import { useDirectUpload, type UseDirectUploadOptions } from "./useDirectUpload";

export const DEFAULT_IMAGE_ACCEPT = ".jpg,.jpeg,.png,.gif,.webp";

export interface DirectImageUploadProps extends UseDirectUploadOptions {
  /** 表单值：存储对象键。 */
  readonly value?: string;
  readonly onChange?: (key: string | undefined) => void;
  /** 把已保存的对象键解析为预览 URL（通常由接口随记录返回）。 */
  readonly resolveUrl?: (key: string) => string | undefined;
  readonly accept?: string;
  /** 前端预检的大小上限（字节）；服务端仍会按自身配置校验。 */
  readonly maxBytes?: number;
  readonly disabled?: boolean;
  readonly onUploaded?: (result: DirectUploadResult) => void;
  /** 预览尺寸（像素），默认 104。 */
  readonly size?: number;
}

function acceptsFile(accept: string, name: string): boolean {
  const exts = accept
    .split(",")
    .map((s) => s.trim().toLowerCase())
    .filter((s) => s.startsWith("."));
  if (exts.length === 0) return true;
  const lower = name.toLowerCase();
  return exts.some((ext) => lower.endsWith(ext));
}

/**
 * 图片上传表单项：预签名 → 直传 → 以对象键作为表单值。
 * 可直接放进 Form.Item / ProForm.Item 中使用。
 */
export function DirectImageUpload({
  value,
  onChange,
  resolveUrl,
  accept = DEFAULT_IMAGE_ACCEPT,
  maxBytes,
  disabled,
  onUploaded,
  size = 104,
  presign,
  send,
}: DirectImageUploadProps) {
  const message = useAppMessage();
  const uploader = useDirectUpload({ presign, send });
  const [uploaded, setUploaded] = useState<{ key: string; url: string } | null>(null);
  const [percent, setPercent] = useState<number | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => () => abortRef.current?.abort(), []);

  const previewUrl = (() => {
    if (!value) return undefined;
    if (uploaded?.key === value) return uploaded.url;
    const resolved = resolveUrl?.(value);
    return resolved ? uploader.resolveUrl(resolved) : undefined;
  })();

  const customRequest: UploadProps["customRequest"] = async (options) => {
    const file = options.file as File;
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setPercent(0);
    try {
      const result = await uploader.upload(file, {
        signal: controller.signal,
        onProgress: (loaded, total) =>
          setPercent(total > 0 ? Math.round((loaded / total) * 100) : 0),
      });
      setUploaded({ key: result.key, url: result.url });
      onChange?.(result.key);
      onUploaded?.(result);
      options.onSuccess?.(result);
    } catch (e) {
      if (!controller.signal.aborted) {
        message.error(formatErrorMessage(e, "上传失败"));
      }
      options.onError?.(e as Error);
    } finally {
      if (abortRef.current === controller) {
        abortRef.current = null;
        setPercent(null);
      }
    }
  };

  const beforeUpload: UploadProps["beforeUpload"] = (file) => {
    if (!acceptsFile(accept, file.name)) {
      message.error("不支持的文件类型");
      return Upload.LIST_IGNORE;
    }
    if (maxBytes !== undefined && file.size > maxBytes) {
      message.error(`文件不能超过 ${Math.floor(maxBytes / 1024 / 1024) || 1}MB`);
      return Upload.LIST_IGNORE;
    }
    return true;
  };

  const box = { width: size, height: size };
  return (
    <div
      className="abysstract-direct-image-upload"
      style={{ display: "inline-flex", gap: 8, alignItems: "flex-start" }}
    >
      <Upload
        accept={accept}
        listType="picture-card"
        showUploadList={false}
        disabled={disabled || percent !== null}
        beforeUpload={beforeUpload}
        customRequest={customRequest}
      >
        {previewUrl && percent === null ? (
          <img src={previewUrl} alt="" style={{ ...box, objectFit: "cover" }} />
        ) : (
          <div
            style={{
              ...box,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            {percent !== null ? <LoadingOutlined /> : <PlusOutlined />}
            <div style={{ marginTop: 8 }}>{percent !== null ? `${percent}%` : "上传"}</div>
          </div>
        )}
      </Upload>
      {value && !disabled && percent === null ? (
        <Button
          type="text"
          size="small"
          icon={<DeleteOutlined />}
          aria-label="移除图片"
          onClick={() => {
            setUploaded(null);
            onChange?.(undefined);
          }}
        />
      ) : null}
    </div>
  );
}
