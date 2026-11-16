import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from "react";
import { Button, Flex, Skeleton, Typography, theme } from "antd";
import { ArrowRightOutlined, CheckOutlined, ReloadOutlined } from "@ant-design/icons";
import type { AdminLoginCaptchaChallenge, AdminLoginCaptchaProps } from "../runtime/types";
import { useAdminTexts } from "../runtime/texts";

/** 服务端内置滑块（captcha/slidecaptcha）的 payload。 */
interface SlidePayload {
  image: string;
  thumb: string;
  thumbX: number;
  thumbY: number;
  thumbWidth: number;
  thumbHeight: number;
  width: number;
  height: number;
}

type State =
  | { status: "loading" }
  | { status: "failed" }
  | { status: "unsupported" }
  | { status: "ready"; challenge: AdminLoginCaptchaChallenge; slide: SlidePayload };

const KEY_STEP = 2;

/**
 * 内置验证码：向服务端取挑战，支持内置滑块（kind = "slide"）。
 * 其他 kind 需要宿主通过 AdminHost.login.captcha 提供组件。
 */
export function BuiltinLoginCaptcha({ onChange, resetKey, loadChallenge }: AdminLoginCaptchaProps) {
  const texts = useAdminTexts().login;
  const { token } = theme.useToken();
  const [state, setState] = useState<State>({ status: "loading" });
  const [reload, setReload] = useState(0);
  const [x, setX] = useState(0);
  const [done, setDone] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<{ startX: number; startLeft: number } | null>(null);
  const [scale, setScale] = useState(1);

  useEffect(() => {
    let active = true;
    setState({ status: "loading" });
    setDone(false);
    setX(0);
    onChange(undefined);
    loadChallenge().then(
      (challenge) => {
        if (!active) return;
        if (challenge.kind !== "slide") {
          setState({ status: "unsupported" });
          console.error(
            `[admin-core] 服务端验证码类型为 "${challenge.kind}"，请通过 AdminHost.login.captcha 提供组件。`,
          );
          return;
        }
        try {
          const slide = JSON.parse(challenge.payload) as SlidePayload;
          setX(slide.thumbX);
          setState({ status: "ready", challenge, slide });
        } catch {
          setState({ status: "failed" });
        }
      },
      () => active && setState({ status: "failed" }),
    );
    return () => {
      active = false;
    };
    // onChange 和 loadChallenge 由登录页提供，换挑战只看 resetKey 和“换一张”。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resetKey, reload]);

  useEffect(() => {
    const el = boxRef.current;
    if (!el || state.status !== "ready") return;
    const measure = () => {
      const width = el.clientWidth;
      setScale(width > 0 ? Math.min(1, width / state.slide.width) : 1);
    };
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [state]);

  const ready = state.status === "ready" ? state : undefined;
  const maxX = ready ? ready.slide.width - ready.slide.thumbWidth : 0;
  const clamp = useCallback((v: number) => Math.max(0, Math.min(maxX, v)), [maxX]);

  const confirm = (finalX: number) => {
    if (!ready || done) return;
    setDone(true);
    onChange({ id: ready.challenge.id, token: JSON.stringify({ x: Math.round(finalX) }) });
  };

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (!ready || done) return;
    e.currentTarget.setPointerCapture?.(e.pointerId);
    dragRef.current = { startX: e.clientX, startLeft: x };
  };
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag) return;
    setX(clamp(drag.startLeft + (e.clientX - drag.startX) / scale));
  };
  const onPointerUp = (e: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag) return;
    dragRef.current = null;
    const finalX = clamp(drag.startLeft + (e.clientX - drag.startX) / scale);
    setX(finalX);
    if (finalX !== drag.startLeft) confirm(finalX);
  };
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!ready || done) return;
    if (e.key === "ArrowRight" || e.key === "ArrowLeft") {
      e.preventDefault();
      setX((v) => clamp(v + (e.key === "ArrowRight" ? KEY_STEP : -KEY_STEP)));
    } else if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      confirm(x);
    }
  };

  if (state.status === "unsupported") {
    return <Typography.Text type="danger">{texts.captchaUnsupported}</Typography.Text>;
  }
  if (state.status === "failed") {
    return (
      <Flex align="center" gap={token.marginXS}>
        <Typography.Text type="danger">{texts.captchaLoadFailed}</Typography.Text>
        <Button size="small" icon={<ReloadOutlined />} onClick={() => setReload((n) => n + 1)}>
          {texts.captchaRefresh}
        </Button>
      </Flex>
    );
  }

  const slide = ready?.slide;
  const width = slide ? slide.width * scale : undefined;
  const handleSize = token.controlHeight;
  return (
    <div
      ref={boxRef}
      data-testid="login-captcha"
      style={{ width: "100%", maxWidth: slide?.width ?? 300 }}
    >
      {!slide ? (
        <Skeleton.Node active style={{ width: "100%", height: 220 * scale }} />
      ) : (
        <div
          style={{
            position: "relative",
            width,
            height: slide.height * scale,
            borderRadius: token.borderRadius,
            overflow: "hidden",
            userSelect: "none",
          }}
        >
          <img
            src={slide.image}
            alt=""
            draggable={false}
            style={{ display: "block", width: "100%", height: "100%" }}
          />
          <img
            src={slide.thumb}
            alt=""
            draggable={false}
            style={{
              position: "absolute",
              left: x * scale,
              top: slide.thumbY * scale,
              width: slide.thumbWidth * scale,
              height: slide.thumbHeight * scale,
            }}
          />
          <Button
            size="small"
            type="text"
            icon={<ReloadOutlined />}
            aria-label={texts.captchaRefresh}
            title={texts.captchaRefresh}
            onClick={() => setReload((n) => n + 1)}
            style={{
              position: "absolute",
              top: token.paddingXXS,
              right: token.paddingXXS,
              background: token.colorBgMask,
              color: token.colorTextLightSolid,
            }}
          />
        </div>
      )}
      <div
        style={{
          position: "relative",
          width: width ?? "100%",
          height: handleSize,
          marginTop: token.marginXS,
          borderRadius: token.borderRadius,
          background: token.colorFillTertiary,
          border: `${token.lineWidth}px ${token.lineType} ${token.colorBorderSecondary}`,
          userSelect: "none",
          touchAction: "none",
        }}
      >
        <Typography.Text
          type="secondary"
          style={{
            position: "absolute",
            inset: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: token.fontSizeSM,
            paddingLeft: handleSize,
            pointerEvents: "none",
          }}
        >
          {/* 拖动后提示会被滑块挡住，只在起点和完成时显示。 */}
          {done ? texts.slideDone : slide && x !== slide.thumbX ? null : texts.slideHint}
        </Typography.Text>
        <div
          role="slider"
          tabIndex={ready && !done ? 0 : -1}
          aria-label={texts.slideHint}
          aria-valuemin={0}
          aria-valuemax={maxX}
          aria-valuenow={Math.round(x)}
          aria-disabled={!ready || done}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={() => (dragRef.current = null)}
          onKeyDown={onKeyDown}
          style={{
            position: "absolute",
            top: -token.lineWidth,
            // 滑块和拼图走同样的距离，拖动时两者对齐。
            left: x * scale,
            width: handleSize,
            height: handleSize,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            borderRadius: token.borderRadius,
            background: done ? token.colorSuccess : token.colorPrimary,
            color: token.colorTextLightSolid,
            cursor: ready && !done ? "grab" : "default",
          }}
        >
          {done ? <CheckOutlined /> : <ArrowRightOutlined />}
        </div>
      </div>
    </div>
  );
}
