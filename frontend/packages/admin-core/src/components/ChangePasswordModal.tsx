import { useEffect, useState } from "react";
import { Form, Input, Modal } from "antd";
import { useSystemClients } from "../system/hooks";
import { formatErrorMessage, useAppMessage } from "../runtime/message";
type Props = {
  open: boolean;
  // 若为 true，关闭按钮被隐藏、取消被禁用，必须完成改密。
  forced: boolean;
  onClose: () => void;
  onSuccess?: () => void;
};

// 密码复杂度前端预检（与后端 validatePasswordPolicy 一致）：
// 长度 8–64，且大写/小写/数字三类中至少两类。
function policyCheck(_rule: unknown, value: string): Promise<void> {
  if (!value) return Promise.reject(new Error("请输入新密码"));
  if (value.length < 8) return Promise.reject(new Error("密码长度至少 8 位"));
  if (value.length > 64) return Promise.reject(new Error("密码长度不得超过 64 位"));
  let classes = 0;
  if (/[A-Z]/.test(value)) classes++;
  if (/[a-z]/.test(value)) classes++;
  if (/[0-9]/.test(value)) classes++;
  if (classes < 2) return Promise.reject(new Error("需包含大写/小写/数字中至少两类"));
  return Promise.resolve();
}

// ChangePasswordModal 用户改密对话框。
// forced=true（首次登录/被重置）时禁止取消，必须完成。
export default function ChangePasswordModal({ open, forced, onClose, onSuccess }: Props) {
  const message = useAppMessage();
  const { auth } = useSystemClients();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (open) {
      form.resetFields();
    }
  }, [open, form]);

  const submit = async () => {
    try {
      const v = await form.validateFields();
      setLoading(true);
      await auth.changePassword({
        oldPassword: v.oldPassword,
        newPassword: v.newPassword,
      });
      message.success("密码修改成功，请使用新密码重新登录");
      onSuccess?.();
      onClose();
    } catch (e) {
      if (e instanceof Error && e.message) {
        // 后端错误（如旧密码不正确）通过 message 提示；表单校验错误由 antd 自动显示。
        if (!(e as { errorFields?: unknown }).errorFields) {
          message.error(formatErrorMessage(e));
        }
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={forced ? "首次登录，请修改初始密码" : "修改密码"}
      open={open}
      onOk={submit}
      onCancel={forced ? undefined : onClose}
      confirmLoading={loading}
      closable={!forced}
      mask={{ closable: !forced }}
      keyboard={!forced}
      okText="确认修改"
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="oldPassword"
          label="当前密码"
          rules={[{ required: true, message: "请输入当前密码" }]}
        >
          <Input.Password placeholder="当前密码" autoComplete="current-password" />
        </Form.Item>
        <Form.Item
          name="newPassword"
          label="新密码"
          rules={[{ required: true, validator: policyCheck }]}
          extra="长度 8–64，含大写/小写/数字中至少两类"
        >
          <Input.Password placeholder="新密码" autoComplete="new-password" />
        </Form.Item>
        <Form.Item
          name="confirmPassword"
          label="确认新密码"
          dependencies={["newPassword"]}
          rules={[
            { required: true, message: "请再次输入新密码" },
            ({ getFieldValue }) => ({
              validator(_, value) {
                if (!value || getFieldValue("newPassword") === value) {
                  return Promise.resolve();
                }
                return Promise.reject(new Error("两次输入的新密码不一致"));
              },
            }),
          ]}
        >
          <Input.Password placeholder="再次输入新密码" autoComplete="new-password" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
