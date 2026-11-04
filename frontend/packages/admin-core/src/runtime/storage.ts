import type { AuthStorage } from "./types";

export function createMemoryAuthStorage(initialToken: string | null = null): AuthStorage {
  let token = initialToken;
  const listeners = new Set<() => void>();
  const notify = () => {
    for (const listener of listeners) listener();
  };
  return {
    getToken() {
      return token;
    },
    setToken(next: string) {
      token = next;
      notify();
    },
    clearToken() {
      token = null;
      notify();
    },
    subscribe(listener: () => void) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
  };
}

export function createLocalAuthStorage(key: string): AuthStorage {
  if (!key) {
    throw new Error("createLocalAuthStorage 需要非空 key");
  }
  const listeners = new Set<() => void>();
  let storageHandler: ((event: StorageEvent) => void) | undefined;

  const notify = () => {
    for (const listener of listeners) listener();
  };

  const ensureWindowListener = () => {
    if (storageHandler || typeof window === "undefined") return;
    storageHandler = (event: StorageEvent) => {
      if (event.storageArea && event.storageArea !== window.localStorage) return;
      if (event.key !== key && event.key !== null) return;
      notify();
    };
    window.addEventListener("storage", storageHandler);
  };

  const removeWindowListener = () => {
    if (!storageHandler || typeof window === "undefined") return;
    window.removeEventListener("storage", storageHandler);
    storageHandler = undefined;
  };

  return {
    getToken() {
      return window.localStorage.getItem(key);
    },
    setToken(token: string) {
      window.localStorage.setItem(key, token);
      notify();
    },
    clearToken() {
      window.localStorage.removeItem(key);
      notify();
    },
    subscribe(listener: () => void) {
      listeners.add(listener);
      ensureWindowListener();
      return () => {
        listeners.delete(listener);
        if (listeners.size === 0) removeWindowListener();
      };
    },
  };
}
