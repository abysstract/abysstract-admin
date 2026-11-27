import { useEffect, useRef, useState } from "react";
import { Typography } from "antd";

const GIS_SRC = "https://accounts.google.com/gsi/client";

interface GoogleIdApi {
  initialize(config: {
    client_id: string;
    callback: (response: { credential?: string }) => void;
    use_fedcm_for_prompt?: boolean;
  }): void;
  renderButton(parent: HTMLElement, options: Record<string, unknown>): void;
}

declare global {
  interface Window {
    google?: { accounts?: { id?: GoogleIdApi } };
  }
}

let gisLoading: Promise<GoogleIdApi> | undefined;

/** 只加载一次 Google Identity Services；加载失败后允许下次重试。 */
function loadGis(): Promise<GoogleIdApi> {
  const ready = window.google?.accounts?.id;
  if (ready) return Promise.resolve(ready);
  gisLoading ??= new Promise<GoogleIdApi>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = GIS_SRC;
    script.async = true;
    script.onload = () => {
      const api = window.google?.accounts?.id;
      if (api) resolve(api);
      else reject(new Error("Google Identity Services unavailable"));
    };
    script.onerror = () => reject(new Error("Google Identity Services failed to load"));
    document.head.appendChild(script);
  }).catch((e: unknown) => {
    gisLoading = undefined;
    document.querySelector(`script[src="${GIS_SRC}"]`)?.remove();
    throw e;
  });
  return gisLoading;
}

export interface GoogleSignInProps {
  /** Google OAuth 客户端 ID（GetLoginOptions 返回）。 */
  clientId: string;
  /** 用户在 Google 弹窗里确认后，拿到 ID token。 */
  onCredential: (idToken: string) => void;
  /** 脚本加载失败时显示的文案。 */
  loadFailedText: string;
}

/** Google 官方的“使用 Google 账号登录”按钮，由 Google 渲染，样式和语言跟随浏览器。 */
export function GoogleSignIn({ clientId, onCredential, loadFailedText }: GoogleSignInProps) {
  const holder = useRef<HTMLDivElement>(null);
  const callback = useRef(onCredential);
  callback.current = onCredential;
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setFailed(false);
    loadGis().then(
      (api) => {
        const el = holder.current;
        if (cancelled || !el) return;
        api.initialize({
          client_id: clientId,
          callback: (res) => {
            if (res.credential) callback.current(res.credential);
          },
        });
        // 按钮宽度跟随容器，Google 只接受 200–400 的像素值。
        const width = Math.min(
          400,
          Math.max(200, Math.round(el.getBoundingClientRect().width) || 320),
        );
        el.replaceChildren();
        api.renderButton(el, {
          type: "standard",
          theme: "outline",
          size: "large",
          text: "signin_with",
          shape: "rectangular",
          logo_alignment: "left",
          width,
        });
      },
      () => {
        if (!cancelled) setFailed(true);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [clientId]);

  if (failed) {
    return (
      <Typography.Text type="secondary" role="status">
        {loadFailedText}
      </Typography.Text>
    );
  }
  return (
    <div
      ref={holder}
      data-testid="google-signin"
      style={{ display: "flex", justifyContent: "center", minHeight: 40 }}
    />
  );
}
