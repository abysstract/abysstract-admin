import type { AuthStorage, SessionSnapshot } from "./types";

export class SessionController {
  private epoch = 0;
  private token: string | null = null;
  private disposed = false;
  private abort = new AbortController();
  private readonly listeners = new Set<() => void>();
  private unsubStorage: (() => void) | undefined;
  private syncing = false;

  constructor(
    private readonly storage: AuthStorage,
    private readonly reportError?: (error: unknown, context: { operation: string }) => void,
  ) {
    this.token = this.safeGet();
    if (this.token) this.epoch = 1;
    this.unsubStorage = this.storage.subscribe(() => {
      this.syncFromStorage();
    });
  }

  snapshot(): SessionSnapshot {
    return { epoch: this.epoch, token: this.token };
  }

  isDisposed(): boolean {
    return this.disposed;
  }

  abortSignal(): AbortSignal {
    return this.abort.signal;
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  applyToken(token: string): void {
    this.safeSet(token);
  }

  clearToken(): void {
    this.safeClear();
  }

  handleUnauthenticated(request: SessionSnapshot): void {
    if (this.disposed) return;
    const current = this.snapshot();
    if (current.epoch !== request.epoch || current.token !== request.token) return;
    this.safeClear();
  }

  resyncFromStorage(): void {
    this.syncFromStorage();
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    this.abort.abort();
    this.unsubStorage?.();
    this.unsubStorage = undefined;
    this.listeners.clear();
  }

  private syncFromStorage(): void {
    if (this.disposed || this.syncing) return;
    const next = this.safeGet();
    this.replaceToken(next);
  }

  private replaceToken(next: string | null): void {
    if (this.disposed) return;
    if (next === this.token) return;
    this.token = next;
    this.epoch += 1;
    this.abort.abort();
    this.abort = new AbortController();
    this.emit();
  }

  private emit(): void {
    for (const listener of this.listeners) listener();
  }

  private safeGet(): string | null {
    try {
      return this.storage.getToken();
    } catch (error) {
      this.reportError?.(error, { operation: "authStorage.getToken" });
      this.forceUnauthenticated();
      return null;
    }
  }

  private safeSet(token: string): void {
    try {
      this.syncing = true;
      this.storage.setToken(token);
      this.replaceToken(token);
    } catch (error) {
      this.reportError?.(error, { operation: "authStorage.setToken" });
      this.forceUnauthenticated();
    } finally {
      this.syncing = false;
    }
  }

  private safeClear(): void {
    try {
      this.syncing = true;
      this.storage.clearToken();
      this.replaceToken(null);
    } catch (error) {
      this.reportError?.(error, { operation: "authStorage.clearToken" });
      this.forceUnauthenticated();
    } finally {
      this.syncing = false;
    }
  }

  private forceUnauthenticated(): void {
    if (this.token !== null || this.epoch === 0) {
      this.token = null;
      this.epoch += 1;
      this.abort.abort();
      this.abort = new AbortController();
      this.emit();
    }
  }
}
