import { Modal } from "antd";

// confirmAction：删除等不可逆操作的统一确认封装（需求 9.9）。
// 仅当用户点击「确认」才执行 onOk；点击「取消」不提交。
// 返回 Promise<boolean>：true 表示用户确认并已执行 onOk；false 表示取消。
export async function confirmAction(options: {
  title: string;
  content?: string;
  okText?: string;
  cancelText?: string;
  danger?: boolean;
  onOk: () => Promise<void>;
}): Promise<boolean> {
  return new Promise((resolve) => {
    Modal.confirm({
      title: options.title,
      content: options.content,
      okText: options.okText ?? "确认",
      cancelText: options.cancelText ?? "取消",
      okButtonProps: options.danger ? { danger: true } : undefined,
      okCancel: true,
      onOk: async () => {
        try {
          await options.onOk();
          resolve(true);
        } catch {
          resolve(false);
        }
      },
      onCancel: () => resolve(false),
    });
  });
}
