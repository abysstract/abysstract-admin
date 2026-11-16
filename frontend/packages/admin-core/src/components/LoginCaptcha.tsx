import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from "react";
import { Button, Flex, Skeleton, Typography, theme } from "antd";
import { ArrowRightOutlined, CheckOutlined } from "@ant-design/icons";
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

/** 方向键每次移动的距离（原图像素）；按住 Shift 时乘 5。 */
const KEY_STEP = 2;

/**
 * 内置验证码：向服务端取挑战，支持内置滑块（kind = "slide"）。
 * 宽度跟随容器（登录表单），图片按比例缩放；滑轨与大号输入框等高、圆角一致。
 * 其他 kind 需要宿主通过 AdminHost.login.captcha 提供组件。
 */
export function BuiltinLoginCaptcha({ onChange, resetKey, loadChallenge }: AdminLoginCaptchaProps) {
  const allTexts = useAdminTexts();
  const texts = allTexts.login;
  const actionTexts = allTexts.action;
  const { token } = theme.useToken();
  const [state, setState] = useState<State>({ status: "loading" });
  const [reload, setReload] = useState(0);
  const [x, setX] = useState(0);
  const [done, setDone] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [focused, setFocused] = useState(false);
  const [boxWidth, setBoxWidth] = useState(0);
  const boxRef = useRef<HTMLDivElement>(null);
  const sliderRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<{ startX: number; startLeft: number } | null>(null);

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

  // 图片和滑轨都按容器宽度缩放。
  useLayoutEffect(() => {
    const el = boxRef.current;
    if (!el) return;
    const measure = () => setBoxWidth(el.clientWidth);
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [state.status]);

  const ready = state.status === "ready" ? state : undefined;

  // 挑战就绪后把焦点放到滑块上，键盘可以直接操作。
  useEffect(() => {
    if (ready) sliderRef.current?.focus();
  }, [ready]);

  if (state.status === "unsupported") {
    return <Typography.Text type="danger">{texts.captchaUnsupported}</Typography.Text>;
  }
  if (state.status === "failed") {
    return (
      <Flex align="center" gap={token.marginXS} wrap>
        <Typography.Text type="danger">{texts.captchaLoadFailed}</Typography.Text>
        <Button onClick={() => setReload((n) => n + 1)}>{actionTexts.retry}</Button>
      </Flex>
    );
  }

  const slide = ready?.slide;
  const scale = slide && boxWidth > 0 ? boxWidth / slide.width : 1;
  const maxX = slide ? slide.width - slide.thumbWidth : 0;
  const clamp = (v: number) => Math.max(0, Math.min(maxX, v));
  const atStart = !slide || x === slide.thumbX;

  // 滑块与拼图同宽、同步移动，拖到滑轨尽头时拼图正好到图片右边。
  const trackHeight = token.controlHeightLG;
  const handleWidth = slide ? slide.thumbWidth * scale : trackHeight;
  const handleLeft = x * scale;
  const radius = token.borderRadiusLG;
  const interactive = !!ready && !done;

  const confirm = (finalX: number) => {
    if (!ready || done) return;
    setDone(true);
    onChange({ id: ready.challenge.id, token: JSON.stringify({ x: Math.round(finalX) }) });
  };

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (!interactive) return;
    e.currentTarget.setPointerCapture?.(e.pointerId);
    dragRef.current = { startX: e.clientX, startLeft: x };
    setDragging(true);
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
    setDragging(false);
    const finalX = clamp(drag.startLeft + (e.clientX - drag.startX) / scale);
    setX(finalX);
    if (finalX !== drag.startLeft) confirm(finalX);
  };
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!interactive) return;
    const step = e.shiftKey ? KEY_STEP * 5 : KEY_STEP;
    const moves: Record<string, (v: number) => number> = {
      ArrowRight: (v) => v + step,
      ArrowUp: (v) => v + step,
      ArrowLeft: (v) => v - step,
      ArrowDown: (v) => v - step,
      Home: () => 0,
      End: () => maxX,
    };
    const move = moves[e.key];
    if (move) {
      e.preventDefault();
      setX((v) => clamp(move(v)));
    } else if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      if (!atStart) confirm(x);
    }
  };

  return (
    <div ref={boxRef} data-testid="login-captcha" style={{ width: "100%" }}>
      <div
        style={{
          position: "relative",
          width: "100%",
          aspectRatio: slide ? `${slide.width} / ${slide.height}` : "2 / 1",
          borderRadius: radius,
          overflow: "hidden",
          background: token.colorFillTertiary,
          userSelect: "none",
        }}
      >
        {!slide ? (
          <Skeleton.Node active style={{ width: "100%", height: "100%" }} />
        ) : (
          <>
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
                left: handleLeft,
                top: slide.thumbY * scale,
                width: slide.thumbWidth * scale,
                height: slide.thumbHeight * scale,
              }}
            />
          </>
        )}
      </div>
      <div
        style={{
          position: "relative",
          height: trackHeight,
          marginTop: token.marginXS,
          borderRadius: radius,
          background: token.colorFillTertiary,
          boxSizing: "border-box",
          border: `${token.lineWidth}px ${token.lineType} ${token.colorBorder}`,
          userSelect: "none",
          touchAction: "none",
        }}
      >
        {/* 拖过的部分用主色浅底填充，长度跟着滑块走。 */}
        {!atStart && (
          <div
            aria-hidden
            style={{
              position: "absolute",
              top: -token.lineWidth,
              left: -token.lineWidth,
              bottom: -token.lineWidth,
              width: handleLeft + handleWidth,
              borderRadius: radius,
              boxSizing: "border-box",
              background: token.colorPrimaryBg,
              border: `${token.lineWidth}px ${token.lineType} ${token.colorPrimaryBorder}`,
            }}
          />
        )}
        {/* 提示只在起点显示，放在滑块右侧的空白里，不会被滑块挡住。 */}
        {atStart && !done && (
          <Typography.Text
            type="secondary"
            aria-hidden
            ellipsis
            style={{
              position: "absolute",
              top: 0,
              bottom: 0,
              left: handleWidth,
              right: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              padding: `0 ${token.paddingXS}px`,
              pointerEvents: "none",
            }}
          >
            {texts.slideHint}
          </Typography.Text>
        )}
        <div
          ref={sliderRef}
          role="slider"
          tabIndex={interactive ? 0 : -1}
          aria-label={texts.slideHint}
          aria-valuemin={0}
          aria-valuemax={maxX}
          aria-valuenow={Math.round(x)}
          aria-valuetext={done ? texts.slideDone : undefined}
          aria-disabled={!interactive}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={() => {
            dragRef.current = null;
            setDragging(false);
          }}
          onKeyDown={onKeyDown}
          onFocus={(e) => {
            // 和 antd 一致：只有键盘聚焦才显示焦点框。
            try {
              setFocused(e.currentTarget.matches(":focus-visible"));
            } catch {
              setFocused(true);
            }
          }}
          onBlur={() => setFocused(false)}
          style={{
            position: "absolute",
            top: -token.lineWidth,
            left: handleLeft - token.lineWidth,
            width: handleWidth,
            height: trackHeight,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            borderRadius: radius,
            background: interactive || done ? token.colorPrimary : token.colorBgContainerDisabled,
            color: interactive || done ? token.colorTextLightSolid : token.colorTextDisabled,
            outline: focused
              ? `${token.lineWidthFocus}px ${token.lineType} ${token.colorPrimaryBorder}`
              : "none",
            outlineOffset: 1,
            cursor: interactive ? (dragging ? "grabbing" : "grab") : "default",
          }}
        >
          {done ? <CheckOutlined /> : <ArrowRightOutlined />}
        </div>
      </div>
    </div>
  );
}
