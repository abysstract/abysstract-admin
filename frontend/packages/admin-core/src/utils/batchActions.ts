import type { Client } from "@connectrpc/connect";
import type { MessageInstance } from "antd/es/message/interface";
import type { CommonExportService } from "@abysstract/admin-proto/abysstract/admin/v1/common_export_pb";
import { formatErrorMessage } from "../runtime/message";
import { confirmAction } from "./confirmAction";

type ExportClient = Client<typeof CommonExportService>;
type ActionDeps = { message: MessageInstance; client: ExportClient };

// 批量删除封装（需求 9.7/9.8）。
// 上限 1000 由后端兜底；前端先校验避免无谓请求。
// 成功后弹提示并返回 true，失败返回 false。
export async function batchDelete(
  resource: string,
  ids: bigint[],
  opts: { resourceName?: string; onSuccess?: () => void } & ActionDeps,
): Promise<boolean> {
  const name = opts.resourceName ?? resource;
  if (ids.length === 0) {
    opts.message.warning("请先选择要删除的记录");
    return false;
  }
  if (ids.length > 1000) {
    opts.message.error("单次批量删除不能超过 1000 条");
    return false;
  }
  const ok = await confirmAction({
    title: `确认删除选中的 ${ids.length} 条${name}？`,
    content: "此操作不可恢复，关联数据将一并清理",
    danger: true,
    okText: "删除",
    onOk: async () => {
      try {
        const resp = await opts.client.batchDelete({ resource, ids });
        opts.message.success(`已删除 ${resp.deleted} 条`);
        opts.onSuccess?.();
      } catch (e) {
        opts.message.error(formatErrorMessage(e, "批量删除失败"));
        throw e;
      }
    },
  });
  return ok;
}

// 导出 Excel（需求 9.5）。
// 触发浏览器下载：构造 Blob + a 标签 click。
export async function exportExcel(
  resource: string,
  opts: { keyword?: string; status?: number; filename?: string } & ActionDeps,
): Promise<void> {
  try {
    const resp = await opts.client.exportExcel({
      resource,
      keyword: opts.keyword ?? "",
      status: opts.status,
    });
    // Uint8Array → ArrayBuffer 视图，避免 Blob 接受 SharedArrayBuffer 的类型冲突。
    const bytes = resp.content;
    const blob = new Blob([bytes.slice()], {
      type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = resp.filename || opts.filename || `${resource}.xlsx`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    opts.message.success(`已导出 ${resp.rowCount} 行`);
  } catch (e) {
    opts.message.error(formatErrorMessage(e, "导出失败"));
  }
}

// 导入 Excel（需求 9.6）。
// 接受 File，读取为 ArrayBuffer 转 Uint8Array 后上传。
// 成功后展示「成功 N 条 / 失败 M 条」与失败明细列表。
export async function importExcel(
  resource: string,
  file: File,
  opts: { onSuccess?: () => void } & ActionDeps,
): Promise<void> {
  try {
    const buf = new Uint8Array(await file.arrayBuffer());
    const resp = await opts.client.importExcel({ resource, content: buf });
    const failed = resp.failures.length;
    if (failed === 0) {
      opts.message.success(`导入完成：成功 ${resp.successCount} 条`);
    } else {
      opts.message.warning(
        `导入完成：成功 ${resp.successCount} 条 / 失败 ${failed} 条（共 ${resp.total} 行）`,
      );
      // 失败明细打印到控制台便于排查
      console.warn("导入失败行明细：", resp.failures);
    }
    opts.onSuccess?.();
  } catch (e) {
    opts.message.error(formatErrorMessage(e, "导入失败"));
  }
}
