import { useSystemClients } from "../../system/hooks";
import { useEffect, useState } from "react";
import { Card, Form, Button, Avatar, Upload, type UploadProps } from "antd";
import { UserOutlined } from "@ant-design/icons";
import { PageContainer, ProFormText } from "@ant-design/pro-components";

import { formatErrorMessage, useAppMessage } from "../../runtime/message";
export default function ProfilePage() {
  const { profile: profileClient, upload: uploadClient } = useSystemClients();
  const message = useAppMessage();
  const [form] = Form.useForm();
  const [avatar, setAvatar] = useState("");
  const [loading, setLoading] = useState(false);
  const [initialLoading, setInitialLoading] = useState(true);

  useEffect(() => {
    profileClient
      .getProfile({})
      .then((res) => {
        form.setFieldsValue({
          username: res.username,
          nickname: res.nickname,
          email: res.email,
          phone: res.phone,
        });
        setAvatar(res.avatar);
      })
      .catch((e: unknown) => message.error(formatErrorMessage(e, "加载失败")))
      .finally(() => setInitialLoading(false));
  }, [form, message]);

  const uploadProps: UploadProps = {
    accept: ".jpg,.jpeg,.png,.gif,.webp",
    showUploadList: false,
    customRequest: async (options) => {
      const { file, onSuccess, onError } = options;
      try {
        const f = file as File;
        const buf = new Uint8Array(await f.arrayBuffer());
        const resp = await uploadClient.uploadAvatar({
          filename: f.name,
          mimeType: f.type,
          content: buf,
        });
        setAvatar(resp.url);
        message.success("头像已更新");
        onSuccess?.({}, new XMLHttpRequest());
      } catch (e) {
        message.error(formatErrorMessage(e, "上传失败"));
        onError?.(e as Error);
      }
    },
    beforeUpload: (f) => {
      if (f.size > 5 * 1024 * 1024) {
        message.error("头像不能超过 5MB");
        return Upload.LIST_IGNORE;
      }
      return true;
    },
  };

  const save = async (v: { nickname?: string; email?: string; phone?: string }) => {
    setLoading(true);
    try {
      await profileClient.updateProfile({
        nickname: v.nickname ?? "",
        email: v.email ?? "",
        phone: v.phone ?? "",
      });
      message.success("已保存");
    } catch (e) {
      message.error(formatErrorMessage(e, "保存失败"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <PageContainer>
      <Card title="个人资料" loading={initialLoading} style={{ maxWidth: 640 }}>
        <div style={{ marginBottom: 24, display: "flex", alignItems: "center", gap: 16 }}>
          <Avatar size={72} src={avatar || undefined} icon={<UserOutlined />} />
          <Upload {...uploadProps}>
            <Button>更换头像</Button>
          </Upload>
        </div>
        <Form form={form} layout="vertical" onFinish={save}>
          <ProFormText name="username" label="用户名" disabled tooltip="用户名不可自行修改" />
          <ProFormText name="nickname" label="昵称" rules={[{ max: 64 }]} />
          <ProFormText
            name="email"
            label="邮箱"
            rules={[{ type: "email", message: "邮箱格式不正确" }]}
          />
          <ProFormText name="phone" label="手机号" rules={[{ max: 32 }]} />
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              保存
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </PageContainer>
  );
}
