import type { AdminApplication } from "./types";

export interface AdminPageLifecycleTarget {
  addEventListener(type: string, listener: (event: Event) => void): void;
  removeEventListener(type: string, listener: (event: Event) => void): void;
}

export interface BindAdminAppPageLifecycleInput {
  application: AdminApplication;
  root: { unmount(): void };
  target?: AdminPageLifecycleTarget;
}

function isPersisted(event: Event): boolean {
  return Boolean((event as PageTransitionEvent).persisted);
}

export function bindAdminAppPageLifecycle(input: BindAdminAppPageLifecycleInput): () => void {
  const target = input.target ?? window;
  let released = false;

  const release = () => {
    if (released) return;
    released = true;
    input.root.unmount();
    input.application.dispose();
  };

  const onPageHide = (event: Event) => {
    if (isPersisted(event)) return;
    release();
  };

  const onPageShow = (event: Event) => {
    if (released || !isPersisted(event)) return;
    input.application.resumeFromPageCache();
  };

  target.addEventListener("pagehide", onPageHide);
  target.addEventListener("pageshow", onPageShow);

  return () => {
    target.removeEventListener("pagehide", onPageHide);
    target.removeEventListener("pageshow", onPageShow);
  };
}
