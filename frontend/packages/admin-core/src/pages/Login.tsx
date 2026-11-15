import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { Button, Card, Flex, Form, Grid, Input, Typography, theme, type GlobalToken } from "antd";
import { Code, ConnectError } from "@connectrpc/connect";
import {
  CaptchaError,
  LoginChallengeSchema,
} from "@abysstract/admin-proto/abysstract/admin/v1/auth_pb";
import { BuiltinLoginCaptcha } from "../components/LoginCaptcha";
import type { AdminLoginCaptchaChallenge, AdminLoginCaptchaValue } from "../runtime/types";
import { useAdminHost, useAdminRuntimeState, useSessionSnapshot } from "../runtime/context";
import { formatErrorMessage, useAppMessage } from "../runtime/message";
import { useAdminTexts } from "../runtime/texts";

interface LoginValues {
  username: string;
  password: string;
}

// 宽屏左侧品牌区的默认背景：几块由主色系 token 生成的柔和色块，
// 宿主换 colorPrimary 或切到暗色算法时跟着变。宿主可用 AdminHost.login.background 替换。
function defaultBackground(token: GlobalToken): string {
  return [
    `radial-gradient(circle at 15% 85%, ${token.colorPrimaryBg} 0, transparent 45%)`,
    `radial-gradient(circle at 85% 20%, ${token.colorInfoBgHover} 0, transparent 40%)`,
    `radial-gradient(circle at 60% 60%, ${token.colorPrimaryBgHover} 0, transparent 35%)`,
    token.colorBgLayout,
  ].join(", ");
}

export default function Login() {
  const message = useAppMessage();
  const texts = useAdminTexts();
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const navigate = useNavigate();
  const host = useAdminHost();
  const runtime = useAdminRuntimeState();
  const session = useSessionSnapshot();
  const [form] = Form.useForm<LoginValues>();
  const [loading, setLoading] = useState(false);
  const attemptRef = useRef(0);
  // 服务端要求验证码后显示验证组件；结果只能用一次，每次失败后 resetKey + 1 让组件重置。
  const [captchaShown, setCaptchaShown] = useState(false);
  const [captchaError, setCaptchaError] = useState<string>();
  const [resetKey, setResetKey] = useState(0);
  const captchaRef = useRef<AdminLoginCaptchaValue | undefined>(undefined);
  // 点了登录才发现需要验证码时，完成验证后自动重新提交。
  const resubmitRef = useRef(false);

  const loadChallenge = useCallback(async (): Promise<AdminLoginCaptchaChallenge> => {
    const res = await runtime.clients.auth.getCaptcha({});
    return { kind: res.kind, payload: res.payload, id: res.id };
  }, [runtime]);

  const onCaptchaChange = useCallback(
    (value: AdminLoginCaptchaValue | undefined) => {
      captchaRef.current = value;
      if (!value) return;
      setCaptchaError(undefined);
      if (resubmitRef.current) {
        resubmitRef.current = false;
        form.submit();
      }
    },
    [form],
  );

  useEffect(() => {
    return () => {
      attemptRef.current += 1;
    };
  }, []);

  if (session.token) {
    return <Navigate to="/" replace />;
  }

  const t = texts.login;
  const config = host.login ?? {};
  const Captcha = config.captcha ?? BuiltinLoginCaptcha;
  const wide = screens.lg === true;

  const initialValues = host.developmentLogin
    ? { username: host.developmentLogin.username, password: host.developmentLogin.password }
    : undefined;

  const onFinish = async (values: LoginValues) => {
    const captcha = captchaRef.current;
    if (captchaShown && !captcha) {
      setCaptchaError(t.captchaRequired);
      resubmitRef.current = true;
      return;
    }
    const attempt = ++attemptRef.current;
    setLoading(true);
    try {
      const res = await runtime.clients.auth.login({
        ...values,
        captchaId: captcha?.id ?? "",
        captcha: captcha?.token ?? "",
      });
      if (attempt !== attemptRef.current || runtime.session.isDisposed()) return;
      runtime.session.applyToken(res.token);
      // 进入首页本身就是结果，不再弹“登录成功”。
      navigate("/", { replace: true });
    } catch (e) {
      if (attempt !== attemptRef.current || runtime.session.isDisposed()) return;
      const challenge =
        e instanceof ConnectError ? e.findDetails(LoginChallengeSchema)[0] : undefined;
      if (captcha) {
        // 验证结果已被这次请求用掉。
        captchaRef.current = undefined;
        setResetKey((k) => k + 1);
      }
      if (challenge?.captchaRequired) setCaptchaShown(true);
      if (challenge?.captchaError === CaptchaError.MISSING) {
        setCaptchaError(captchaShown ? t.captchaRequired : undefined);
        resubmitRef.current = true;
      } else if (challenge?.captchaError === CaptchaError.INVALID) {
        setCaptchaError(t.captchaInvalid);
        resubmitRef.current = true;
      } else if (e instanceof ConnectError && e.code === Code.Unauthenticated) {
        // 能对应到字段：标在密码下方，改正前一直显示。
        setCaptchaError(undefined);
        form.setFields([{ name: "password", errors: [t.invalidCredentials] }]);
      } else if (
        e instanceof ConnectError &&
        (e.code === Code.PermissionDenied ||
          e.code === Code.ResourceExhausted ||
          // 带着验证码时的 Unavailable 来自验证服务，其余是网络问题，走下面的通用文案。
          (e.code === Code.Unavailable && captcha !== undefined))
      ) {
        // 账号锁定、停用、尝试过多、验证服务不可用：服务端文案面向用户，带剩余时间等信息，原样显示。
        message.error(e.rawMessage || t.cannotLogin);
      } else {
        message.error(formatErrorMessage(e, t.cannotLogin));
      }
    } finally {
      if (attempt === attemptRef.current) setLoading(false);
    }
  };

  const brand = (level: 3 | 4): ReactNode => (
    <div>
      <Flex align="center" gap={token.marginXS}>
        {config.logo}
        <Typography.Title level={level} style={{ margin: 0 }}>
          {host.title}
        </Typography.Title>
      </Flex>
      {config.subtitle != null && (
        <Typography.Text type="secondary" style={{ display: "block", marginTop: token.marginXS }}>
          {config.subtitle}
        </Typography.Text>
      )}
    </div>
  );

  const footer =
    config.footer != null ? (
      <Typography.Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
        {config.footer}
      </Typography.Text>
    ) : null;

  const background = config.background ?? defaultBackground(token);

  const loginForm = (
    <Form
      form={form}
      layout="vertical"
      requiredMark={false}
      size="large"
      initialValues={initialValues}
      onFinish={onFinish}
    >
      <Form.Item
        label={t.username}
        name="username"
        rules={[{ required: true, message: texts.form.required(t.username) }]}
      >
        <Input autoComplete="username" autoFocus={!initialValues} />
      </Form.Item>
      <Form.Item
        label={t.password}
        name="password"
        rules={[{ required: true, message: texts.form.required(t.password) }]}
      >
        <Input.Password autoComplete="current-password" />
      </Form.Item>
      {captchaShown && (
        <Form.Item
          label={t.captcha}
          validateStatus={captchaError ? "error" : undefined}
          help={captchaError}
        >
          <Captcha onChange={onCaptchaChange} resetKey={resetKey} loadChallenge={loadChallenge} />
        </Form.Item>
      )}
      <Form.Item style={{ marginBottom: 0 }}>
        <Button type="primary" htmlType="submit" block loading={loading}>
          {t.submit}
        </Button>
      </Form.Item>
    </Form>
  );

  if (!wide) {
    // 窄屏：背景铺满整页，产品名和表单放在卡片里，背景是图片时也看得清。
    return (
      <Flex
        vertical
        align="center"
        style={{ minHeight: "100vh", padding: token.padding, background }}
      >
        <Flex vertical justify="center" style={{ flex: 1, width: "100%", maxWidth: 400 }}>
          <Card>
            <div style={{ marginBottom: token.marginLG }}>{brand(4)}</div>
            {loginForm}
          </Card>
        </Flex>
        {footer && <div style={{ paddingTop: token.marginLG }}>{footer}</div>}
      </Flex>
    );
  }

  return (
    <Flex style={{ minHeight: "100vh", background: token.colorBgContainer }}>
      <Flex
        vertical
        justify="space-between"
        data-testid="login-brand-panel"
        style={{
          width: "45%",
          padding: token.paddingXL,
          background,
          borderRight: `${token.lineWidth}px ${token.lineType} ${token.colorSplit}`,
        }}
      >
        {brand(3)}
        {footer}
      </Flex>
      <Flex
        vertical
        align="center"
        justify="center"
        style={{ flex: 1, minWidth: 0, padding: token.padding }}
      >
        <div style={{ width: "100%", maxWidth: 360 }}>
          <Typography.Title level={4} style={{ marginTop: 0, marginBottom: token.marginLG }}>
            {t.title}
          </Typography.Title>
          {loginForm}
        </div>
      </Flex>
    </Flex>
  );
}
