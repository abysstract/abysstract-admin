import { Code, ConnectError, type Interceptor, type Transport } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import type { SessionController } from "./session";
import type { SessionSnapshot } from "./types";

export class ApplicationDisposedError extends Error {
  constructor() {
    super("createAdminApp 实例已释放");
    this.name = "ApplicationDisposedError";
  }
}

export function linkAbortSignals(
  caller: AbortSignal | undefined,
  session: AbortSignal,
): { controller: AbortController; signal: AbortSignal; cleanup: () => void } {
  const controller = new AbortController();
  const sources = caller === undefined ? [session] : [caller, session];
  const listeners = new Map<AbortSignal, () => void>();
  let cleaned = false;

  const cleanup = () => {
    if (cleaned) return;
    cleaned = true;
    for (const [signal, listener] of listeners) {
      signal.removeEventListener("abort", listener);
    }
    listeners.clear();
  };

  const abortWith = (signal: AbortSignal) => {
    if (controller.signal.aborted) return;
    const reason =
      signal.reason ??
      (() => {
        const error = new Error("This operation was aborted");
        error.name = "AbortError";
        return error;
      })();
    controller.abort(reason);
  };

  for (const signal of sources) {
    if (signal.aborted) {
      abortWith(signal);
      return { controller, signal: controller.signal, cleanup };
    }
  }
  for (const signal of sources) {
    const listener = () => {
      abortWith(signal);
      cleanup();
    };
    listeners.set(signal, listener);
    signal.addEventListener("abort", listener);
  }
  return { controller, signal: controller.signal, cleanup };
}

export function wrapAsyncIterable<T>(
  iterable: AsyncIterable<T>,
  options: {
    settleOnComplete: boolean;
    onSettle: () => void;
    onCancel?: () => void;
  },
): AsyncIterable<T> {
  let iterator: AsyncIterator<T> | undefined;
  return {
    [Symbol.asyncIterator]() {
      if (iterator) return iterator;
      const it = iterable[Symbol.asyncIterator]();
      let settled = false;
      const settle = () => {
        if (settled) return;
        settled = true;
        options.onSettle();
      };
      iterator = {
        async next() {
          try {
            const result = await it.next();
            if (result.done && options.settleOnComplete) settle();
            return result;
          } catch (error) {
            settle();
            throw error;
          }
        },
        async return(value?: unknown) {
          try {
            options.onCancel?.();
            if (it.return) return await it.return(value as T | PromiseLike<T>);
            return { done: true, value: value as T };
          } finally {
            settle();
          }
        },
        async throw(error?: unknown) {
          try {
            options.onCancel?.();
            if (it.throw) return await it.throw(error);
            throw error;
          } finally {
            settle();
          }
        },
      };
      return iterator;
    },
  };
}

export function createAdminTransport(input: {
  baseUrl: string;
  session: SessionController;
  fetch?: typeof globalThis.fetch;
}): Transport {
  const interceptor: Interceptor = (next) => async (req) => {
    if (input.session.isDisposed()) {
      throw new ApplicationDisposedError();
    }
    const snapshot: SessionSnapshot = input.session.snapshot();
    if (snapshot.token) {
      req.header.set("Authorization", `Bearer ${snapshot.token}`);
    }
    const linked = linkAbortSignals(req.signal, input.session.abortSignal());
    const finish = () => linked.cleanup();
    linked.signal.addEventListener("abort", finish, { once: true });
    const cancelLinked = () => {
      if (!linked.controller.signal.aborted) {
        linked.controller.abort();
      }
    };
    try {
      const outgoing = req.stream
        ? {
            ...req,
            signal: linked.signal,
            message: wrapAsyncIterable(req.message, {
              settleOnComplete: false,
              onSettle: finish,
              onCancel: cancelLinked,
            }),
          }
        : { ...req, signal: linked.signal };
      const res = await next(outgoing);
      if (!res.stream) {
        finish();
        return res;
      }
      return {
        ...res,
        message: wrapAsyncIterable(res.message, {
          settleOnComplete: true,
          onSettle: finish,
          onCancel: cancelLinked,
        }),
      };
    } catch (error) {
      finish();
      if (error instanceof ConnectError && error.code === Code.Unauthenticated) {
        input.session.handleUnauthenticated(snapshot);
      }
      throw error;
    }
  };
  const inner = createConnectTransport({
    baseUrl: input.baseUrl,
    interceptors: [interceptor],
    fetch: input.fetch,
  });
  return {
    unary: (method, signal, timeoutMs, header, inputMessage, contextValues) =>
      inner.unary(method, signal, timeoutMs, header, inputMessage, contextValues),
    async stream(method, signal, timeoutMs, header, inputMessage, contextValues) {
      const local = new AbortController();
      const abortLocal = () => {
        if (!local.signal.aborted) {
          local.abort();
        }
      };
      if (signal?.aborted) {
        abortLocal();
      } else {
        signal?.addEventListener("abort", abortLocal, { once: true });
      }
      const wrappedInput = wrapAsyncIterable(inputMessage, {
        settleOnComplete: false,
        onSettle: () => undefined,
        onCancel: abortLocal,
      });
      try {
        const res = await inner.stream(
          method,
          local.signal,
          timeoutMs,
          header,
          wrappedInput,
          contextValues,
        );
        return {
          ...res,
          message: wrapAsyncIterable(res.message, {
            settleOnComplete: true,
            onSettle: () => {
              signal?.removeEventListener("abort", abortLocal);
            },
            onCancel: abortLocal,
          }),
        };
      } catch (error) {
        signal?.removeEventListener("abort", abortLocal);
        throw error;
      }
    },
  };
}
