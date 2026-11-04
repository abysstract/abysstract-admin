import { useEffect, useRef, useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { Button, Card, Form, Input, Typography } from "antd";
import { UserOutlined, LockOutlined } from "@ant-design/icons";
import { useAdminHost, useAdminRuntimeState, useSessionSnapshot } from "../runtime/context";
import { formatErrorMessage, useAppMessage } from "../runtime/message";

export default function Login() {
  const message = useAppMessage();
  const navigate = useNavigate();
  const host = useAdminHost();
  const runtime = useAdminRuntimeState();
  const session = useSessionSnapshot();
  const [loading, setLoading] = useState(false);
  const attemptRef = useRef(0);

  useEffect(() => {
    return () => {
      attemptRef.current += 1;
    };
  }, []);

  if (session.token) {
    return <Navigate to="/" replace />;
  }

  const initialValues = host.developmentLogin
    ? { username: host.developmentLogin.username, password: host.developmentLogin.password }
    : undefined;

  const onFinish = async (values: { username: string; password: string }) => {
    const attempt = ++attemptRef.current;
    setLoading(true);
    try {
      const res = await runtime.clients.auth.login(values);
      if (attempt !== attemptRef.current || runtime.session.isDisposed()) return;
      runtime.session.applyToken(res.token);
      message.success("登录成功");
      navigate("/", { replace: true });
    } catch (e) {
      if (attempt !== attemptRef.current || runtime.session.isDisposed()) return;
      message.error(formatErrorMessage(e, "登录失败"));
    } finally {
      if (attempt === attemptRef.current) setLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "#f0f2f5",
      }}
    >
      <Card style={{ width: 380 }}>
        <Typography.Title level={3} style={{ textAlign: "center" }}>
          {host.title}
        </Typography.Title>
        <Form initialValues={initialValues} onFinish={onFinish}>
          <Form.Item name="username" rules={[{ required: true, message: "请输入用户名" }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" size="large" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: "请输入密码" }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" size="large" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" block size="large" loading={loading}>
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
