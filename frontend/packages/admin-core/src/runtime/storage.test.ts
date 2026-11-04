import { describe, expect, it, vi } from "vitest";
import { createLocalAuthStorage, createMemoryAuthStorage } from "./storage";
import { SessionController } from "./session";

describe("auth storage adapters", () => {
  it("isolates two memory keys", () => {
    const a = createMemoryAuthStorage();
    const b = createMemoryAuthStorage();
    a.setToken("token-a");
    b.setToken("token-b");
    expect(a.getToken()).toBe("token-a");
    expect(b.getToken()).toBe("token-b");
    a.clearToken();
    expect(a.getToken()).toBeNull();
    expect(b.getToken()).toBe("token-b");
  });

  it("notifies local subscribers on set and clear", () => {
    const storage = createMemoryAuthStorage();
    const listener = vi.fn();
    const stop = storage.subscribe(listener);
    storage.setToken("one");
    storage.clearToken();
    expect(listener).toHaveBeenCalledTimes(2);
    stop();
    storage.setToken("two");
    expect(listener).toHaveBeenCalledTimes(2);
  });

  it("syncs localStorage adapters across tabs via storage events", () => {
    const key = "admin-core-test-token";
    window.localStorage.removeItem(key);
    const storage = createLocalAuthStorage(key);
    const listener = vi.fn();
    storage.subscribe(listener);
    window.dispatchEvent(
      new StorageEvent("storage", {
        key,
        newValue: "from-other-tab",
        storageArea: window.localStorage,
      }),
    );
    expect(listener).toHaveBeenCalled();
    storage.clearToken();
  });
});

describe("session epoch", () => {
  it("keeps same-value storage events idempotent", () => {
    const storage = createMemoryAuthStorage("same");
    const session = new SessionController(storage);
    const first = session.snapshot();
    storage.setToken("same");
    expect(session.snapshot()).toEqual(first);
    session.dispose();
  });

  it("ignores unauthenticated results from an old epoch", () => {
    const storage = createMemoryAuthStorage("old");
    const session = new SessionController(storage);
    const old = session.snapshot();
    session.applyToken("new");
    session.handleUnauthenticated(old);
    expect(session.snapshot().token).toBe("new");
    expect(session.snapshot().epoch).not.toBe(old.epoch);
    session.dispose();
  });

  it("clears the current epoch on unauthenticated", () => {
    const storage = createMemoryAuthStorage("now");
    const session = new SessionController(storage);
    const current = session.snapshot();
    session.handleUnauthenticated(current);
    expect(session.snapshot().token).toBeNull();
    expect(session.snapshot().epoch).toBeGreaterThan(current.epoch);
    session.dispose();
  });

  it("resyncs from storage when same-tab writes did not notify subscribers", () => {
    const key = "admin-core-resync-token";
    window.localStorage.setItem(key, "old-token");
    const storage = createLocalAuthStorage(key);
    const session = new SessionController(storage);
    expect(session.snapshot().token).toBe("old-token");
    window.localStorage.setItem(key, "new-token");
    expect(session.snapshot().token).toBe("old-token");
    session.resyncFromStorage();
    expect(session.snapshot().token).toBe("new-token");
    expect(session.snapshot().epoch).toBeGreaterThan(1);
    session.dispose();
    window.localStorage.removeItem(key);
  });
  it("reports storage read failure and stays unauthenticated", () => {
    const reportError = vi.fn();
    const storage = {
      getToken() {
        throw new Error("quota");
      },
      setToken() {},
      clearToken() {},
      subscribe() {
        return () => undefined;
      },
    };
    const session = new SessionController(storage, reportError);
    expect(session.snapshot().token).toBeNull();
    expect(reportError).toHaveBeenCalled();
    session.dispose();
  });
});
