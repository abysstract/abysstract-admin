import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import {
  Button,
  Card,
  Divider,
  Flex,
  Form,
  Grid,
  Input,
  Tooltip,
  Typography,
  theme,
  type GlobalToken,
} from "antd";
import { CloseOutlined, ReloadOutlined } from "@ant-design/icons";
import { Code, ConnectError } from "@connectrpc/connect";
import {
  CaptchaError,
  LoginChallengeSchema,
} from "@abysstract/admin-proto/abysstract/admin/v1/auth_pb";
import { BuiltinLoginCaptcha } from "../components/LoginCaptcha";
import { GoogleSignIn } from "../components/GoogleSignIn";
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
  // 服务端要求验证码后，之后每次点“登录”先在表单位置显示验证面板，验证通过即自动提交。
  // 验证结果只能用一次，每次失败后 resetKey + 1 让组件换一张。
  const [captchaNeeded, setCaptchaNeeded] = useState(false);
  const [captchaOpen, setCaptchaOpen] = useState(false);
  const [captchaError, setCaptchaError] = useState<string>();
  const [resetKey, setResetKey] = useState(0);
  const captchaRef = useRef<AdminLoginCaptchaValue | undefined>(undefined);
  const submitRef = useRef<HTMLButtonElement>(null);

  const loadChallenge = useCallback(async (): Promise<AdminLoginCaptchaChallenge> => {
    const res = await runtime.clients.auth.getCaptcha({});
    return { kind: res.kind, payload: res.payload, id: res.id };
  }, [runtime]);

  const openCaptcha = useCallback(
    (error?: string) => {
      // 验证面板盖住表单，上一次的字段错误不再有意义。
      form.setFields([{ name: "password", errors: [] }]);
      setCaptchaError(error);
      setCaptchaOpen(true);
    },
    [form],
  );

  const closeCaptcha = useCallback(() => {
    setCaptchaOpen(false);
    setCaptchaError(undefined);
    // 回到表单后焦点放回“登录”按钮。
    setTimeout(() => submitRef.current?.focus());
  }, []);

  const onCaptchaChange = useCallback(
    (value: AdminLoginCaptchaValue | undefined) => {
      captchaRef.current = value;
      if (!value) return;
      // 验证通过：收起面板，按钮进入加载，自动提交。
      setCaptchaOpen(false);
      setCaptchaError(undefined);
      form.submit();
    },
    [form],
  );

  useEffect(() => {
    return () => {
      attemptRef.current += 1;
    };
  }, []);

  // 服务端是否启用了 Google 登录。取不到就当没启用，不影响密码登录。
  const [googleClientId, setGoogleClientId] = useState<string>();
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const options = await runtime.clients.auth.getLoginOptions({});
        if (!cancelled) setGoogleClientId(options.googleClientId || undefined);
      } catch {
        // 保持不显示 Google 按钮。
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [runtime]);

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
    if (captchaNeeded && !captcha) {
      openCaptcha();
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
      if (challenge?.captchaRequired) setCaptchaNeeded(true);
      if (challenge?.captchaError === CaptchaError.MISSING) {
        openCaptcha();
      } else if (challenge?.captchaError === CaptchaError.INVALID) {
        openCaptcha(t.captchaInvalid);
      } else if (e instanceof ConnectError && e.code === Code.Unauthenticated) {
        // 能对应到字段：标在密码下方，改正前一直显示。
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

  const onGoogleCredential = async (idToken: string) => {
    const attempt = ++attemptRef.current;
    setLoading(true);
    try {
      const res = await runtime.clients.auth.loginWithGoogle({ idToken });
      if (attempt !== attemptRef.current || runtime.session.isDisposed()) return;
      runtime.session.applyToken(res.token);
      navigate("/", { replace: true });
    } catch (e) {
      if (attempt !== attemptRef.current || runtime.session.isDisposed()) return;
      if (
        e instanceof ConnectError &&
        (e.code === Code.PermissionDenied ||
          e.code === Code.Unavailable ||
          e.code === Code.Unauthenticated)
      ) {
        // 未关联后台用户、账号停用、Google 不可达：服务端文案面向用户，原样显示。
        message.error(e.rawMessage || t.googleFailed);
      } else {
        message.error(formatErrorMessage(e, t.googleFailed));
      }
    } finally {
      if (attempt === attemptRef.current) setLoading(false);
    }
  };

  // 密码表单下方的“或 / Google 按钮”。验证面板打开时表单被隐藏，这一块也一起隐藏。
  const googleSection =
    googleClientId && !captchaOpen ? (
      <div style={{ marginTop: token.marginLG }}>
        <Divider plain style={{ margin: `0 0 ${token.margin}px` }}>
          <Typography.Text type="secondary">{t.or}</Typography.Text>
        </Divider>
        <GoogleSignIn
          clientId={googleClientId}
          onCredential={(idToken) => void onGoogleCredential(idToken)}
          loadFailedText={t.googleLoadFailed}
        />
      </div>
    ) : null;

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

  const captchaTitleId = "admin-login-captcha-title";
  // 验证面板：标题行（标题在左，“换一张”“关闭”在右）+ 验证组件 + 错误。
  // 宽屏时标题顶替“登录”标题，用同一级；窄屏时在产品名下方，用分组标题级。
  const captchaPanel = (level: 4 | 5) =>
    captchaOpen && (
      <div
        role="group"
        aria-labelledby={captchaTitleId}
        onKeyDown={(e) => {
          if (e.key === "Escape") closeCaptcha();
        }}
      >
        <Flex
          align="center"
          justify="space-between"
          gap={token.marginXS}
          style={{ marginBottom: level === 4 ? token.marginLG : token.marginXS }}
        >
          <Typography.Title level={level} id={captchaTitleId} style={{ margin: 0 }}>
            {t.captcha}
          </Typography.Title>
          <Flex gap={token.marginXXS}>
            {!config.captcha && (
              <Tooltip title={t.captchaRefresh}>
                <Button
                  type="text"
                  icon={<ReloadOutlined />}
                  aria-label={t.captchaRefresh}
                  onClick={() => {
                    setCaptchaError(undefined);
                    setResetKey((k) => k + 1);
                  }}
                />
              </Tooltip>
            )}
            <Tooltip title={texts.action.close}>
              <Button
                type="text"
                icon={<CloseOutlined />}
                aria-label={texts.action.close}
                onClick={closeCaptcha}
              />
            </Tooltip>
          </Flex>
        </Flex>
        <Captcha onChange={onCaptchaChange} resetKey={resetKey} loadChallenge={loadChallenge} />
        {captchaError && (
          <Typography.Text
            type="danger"
            role="alert"
            style={{ display: "block", marginTop: token.marginXS }}
          >
            {captchaError}
          </Typography.Text>
        )}
      </div>
    );

  const loginForm = (
    <Form
      form={form}
      // 验证面板显示时表单保留在原位（不卸载，填过的内容还在），只是隐藏。
      style={captchaOpen ? { display: "none" } : undefined}
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
      <Form.Item style={{ marginBottom: 0 }}>
        <Button ref={submitRef} type="primary" htmlType="submit" block loading={loading}>
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
            {captchaPanel(5)}
            {loginForm}
            {googleSection}
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
          {!captchaOpen && (
            <Typography.Title level={4} style={{ marginTop: 0, marginBottom: token.marginLG }}>
              {t.title}
            </Typography.Title>
          )}
          {captchaPanel(4)}
          {loginForm}
          {googleSection}
        </div>
      </Flex>
    </Flex>
  );
}
