import { useSystemClients } from "../../system/hooks";
import { useEffect, useState } from "react";
import { Card, Form, Button, Avatar, Input, Result, Upload, theme, type UploadProps } from "antd";
import { UserOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-components";

import { formatErrorMessage, useAppMessage } from "../../runtime/message";
import { useAdminTexts } from "../../runtime/texts";
export default function ProfilePage() {
  const { profile: profileClient, upload: uploadClient } = useSystemClients();
  const message = useAppMessage();
  const [initialValues, setInitialValues] = useState<Record<string, string>>();
  const [avatar, setAvatar] = useState("");
  const [loading, setLoading] = useState(false);
  const [initialLoading, setInitialLoading] = useState(true);
  const [loadError, setLoadError] = useState<unknown>();
  const [attempt, setAttempt] = useState(0);
  const { token } = theme.useToken();
  const texts = useAdminTexts();

  useEffect(() => {
    setInitialLoading(true);
    setLoadError(undefined);
    profileClient
      .getProfile({})
      .then((res) => {
        setInitialValues({
          username: res.username,
          nickname: res.nickname,
          email: res.email,
          phone: res.phone,
        });
        setAvatar(res.avatar);
      })
      .catch((e: unknown) => setLoadError(e))
      .finally(() => setInitialLoading(false));
  }, [profileClient, attempt]);

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
        message.error(formatErrorMessage(e, "无法上传头像，请稍后重试"));
        onError?.(e as Error);
      }
    },
    beforeUpload: (f) => {
      if (f.size > 5 * 1024 * 1024) {
        message.error("无法上传：头像超过 5MB");
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
      message.error(formatErrorMessage(e, "无法保存，请稍后重试"));
    } finally {
      setLoading(false);
    }
  };

  if (loadError !== undefined) {
    return (
      <PageContainer>
        <Result
          status="error"
          title="无法加载个人资料"
          subTitle={formatErrorMessage(loadError)}
          extra={
            <Button onClick={() => setAttempt((value) => value + 1)}>{texts.action.retry}</Button>
          }
        />
      </PageContainer>
    );
  }

  return (
    <PageContainer>
      <Card title="个人资料" loading={initialLoading} style={{ maxWidth: 720 }}>
        <div
          style={{
            marginBottom: token.marginLG,
            display: "flex",
            alignItems: "center",
            gap: token.margin,
          }}
        >
          <Avatar size={64} src={avatar || undefined} icon={<UserOutlined />} />
          <Upload {...uploadProps}>
            <Button>更换头像</Button>
          </Upload>
        </div>
        {/* Card 加载时不渲染子元素，表单在数据到达后挂载，用 initialValues 填值。 */}
        <Form layout="vertical" initialValues={initialValues} onFinish={save}>
          <Form.Item name="username" label="用户名">
            <Input disabled />
          </Form.Item>
          <Form.Item name="nickname" label="昵称" rules={[{ max: 64, message: "最多 64 个字" }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="email"
            label="邮箱"
            rules={[{ type: "email", message: "邮箱格式不对，例如 name@example.com" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="phone" label="手机号" rules={[{ max: 32, message: "最多 32 个字" }]}>
            <Input />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              {texts.action.save}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </PageContainer>
  );
}
