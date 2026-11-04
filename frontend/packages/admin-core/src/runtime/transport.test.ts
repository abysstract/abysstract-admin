import { Code, ConnectError, createClient } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import {
  AuthService,
  GetUserInfoRequestSchema,
} from "@abysstract/admin-proto/abysstract/admin/v1/auth_pb";
import { describe, expect, it, vi } from "vitest";
import { SessionController } from "./session";
import { createMemoryAuthStorage } from "./storage";
import { createAdminTransport, wrapAsyncIterable } from "./transport";

function createObservingFetch() {
  const requests: Array<{ signal: AbortSignal; aborted: Promise<void> }> = [];
  const fetchImpl: typeof fetch = (_input, init) => {
    const signal = init?.signal ?? null;
    if (signal === null) {
      return Promise.reject(new Error("request is missing signal"));
    }
    let settleAbort: () => void = () => undefined;
    const aborted = new Promise<void>((resolve) => {
      settleAbort = resolve;
    });
    const fail = () => {
      const error = new Error("This operation was aborted");
      error.name = "AbortError";
      return error;
    };
    if (signal.aborted) {
      settleAbort();
      requests.push({ signal, aborted });
      return Promise.reject(fail());
    }
    signal.addEventListener(
      "abort",
      () => {
        settleAbort();
      },
      { once: true },
    );
    requests.push({ signal, aborted });
    return new Promise((_resolve, reject) => {
      signal.addEventListener(
        "abort",
        () => {
          reject(fail());
        },
        { once: true },
      );
    });
  };
  return { fetch: fetchImpl, requests };
}

async function startGetUserInfo(token = "token-old") {
  const session = new SessionController(createMemoryAuthStorage(token));
  const observed = createObservingFetch();
  const transport = createAdminTransport({
    baseUrl: "http://127.0.0.1:9",
    session,
    fetch: observed.fetch,
  });
  const client = createClient(AuthService, transport);
  const pending = client.getUserInfo({});
  await vi.waitFor(() => {
    expect(observed.requests.length).toBeGreaterThanOrEqual(1);
  });
  return { session, client, pending, observed };
}

describe("admin transport cancels in-flight Connect requests", () => {
  it("aborts the in-flight fetch when the application session is disposed", async () => {
    const { session, pending, observed } = await startGetUserInfo();
    const inFlight = observed.requests[0];
    session.dispose();
    await inFlight.aborted;
    expect(inFlight.signal.aborted).toBe(true);
    await expect(pending).rejects.toBeInstanceOf(ConnectError);
    await expect(pending).rejects.toMatchObject({ code: Code.Canceled });
  });

  it("aborts the in-flight fetch on logout and keeps a later request alive", async () => {
    const { session, client, pending, observed } = await startGetUserInfo();
    const loggedOut = observed.requests[0];
    session.clearToken();
    await loggedOut.aborted;
    expect(loggedOut.signal.aborted).toBe(true);
    await expect(pending).rejects.toMatchObject({ code: Code.Canceled });

    const next = client.getUserInfo({});
    await vi.waitFor(() => {
      expect(observed.requests.length).toBe(2);
    });
    expect(observed.requests[1].signal.aborted).toBe(false);
    expect(observed.requests[1].signal).not.toBe(loggedOut.signal);
    session.dispose();
    await observed.requests[1].aborted;
    await expect(next).rejects.toMatchObject({ code: Code.Canceled });
  });

  it("aborts the old fetch after a new login while the new request stays live", async () => {
    const { session, client, pending, observed } = await startGetUserInfo("token-old");
    const previous = observed.requests[0];
    session.applyToken("token-new");
    await previous.aborted;
    expect(previous.signal.aborted).toBe(true);
    await expect(pending).rejects.toMatchObject({ code: Code.Canceled });

    const next = client.getUserInfo({});
    await vi.waitFor(() => {
      expect(observed.requests.length).toBe(2);
    });
    expect(observed.requests[1].signal.aborted).toBe(false);
    expect(observed.requests[1].signal).not.toBe(previous.signal);
    session.dispose();
    await observed.requests[1].aborted;
    await expect(next).rejects.toMatchObject({ code: Code.Canceled });
  });

  it("still honors the caller AbortSignal", async () => {
    const session = new SessionController(createMemoryAuthStorage("token"));
    const observed = createObservingFetch();
    const transport = createAdminTransport({
      baseUrl: "http://127.0.0.1:9",
      session,
      fetch: observed.fetch,
    });
    const client = createClient(AuthService, transport);
    const caller = new AbortController();
    const pending = client.getUserInfo({}, { signal: caller.signal });
    await vi.waitFor(() => {
      expect(observed.requests.length).toBe(1);
    });
    caller.abort();
    await observed.requests[0].aborted;
    expect(observed.requests[0].signal.aborted).toBe(true);
    await expect(pending).rejects.toMatchObject({ code: Code.Canceled });
    expect(session.isDisposed()).toBe(false);
    session.dispose();
  });
});

const endStreamFlag = 0b00000010;

function encodeEnvelope(flags: number, data: Uint8Array): Uint8Array {
  const bytes = new Uint8Array(data.length + 5);
  bytes.set(data, 5);
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  view.setUint8(0, flags);
  view.setUint32(1, data.length);
  return bytes;
}

function createObservingStreamFetch(options?: { completeImmediately?: boolean }) {
  const requests: Array<{
    signal: AbortSignal;
    aborted: Promise<void>;
    closeWithEndStream: () => void;
  }> = [];
  const fetchImpl: typeof fetch = (_input, init) => {
    const signal = init?.signal ?? null;
    if (signal === null) {
      return Promise.reject(new Error("request is missing signal"));
    }
    let settleAbort: () => void = () => undefined;
    const aborted = new Promise<void>((resolve) => {
      settleAbort = resolve;
    });
    let bodyController: ReadableStreamDefaultController<Uint8Array> | undefined;
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        bodyController = controller;
        if (options?.completeImmediately) {
          controller.enqueue(encodeEnvelope(endStreamFlag, new TextEncoder().encode("{}")));
          controller.close();
        }
      },
    });
    const onAbort = () => {
      settleAbort();
      try {
        bodyController?.error(new DOMException("The operation was aborted", "AbortError"));
      } catch {
        // already closed
      }
    };
    if (signal.aborted) {
      onAbort();
    } else {
      signal.addEventListener("abort", onAbort, { once: true });
    }
    const closeWithEndStream = () => {
      bodyController?.enqueue(encodeEnvelope(endStreamFlag, new TextEncoder().encode("{}")));
      bodyController?.close();
    };
    requests.push({ signal, aborted, closeWithEndStream });
    return Promise.resolve(
      new Response(body, {
        status: 200,
        headers: { "Content-Type": "application/connect+json" },
      }),
    );
  };
  return { fetch: fetchImpl, requests };
}

function watchUserInfoMethod() {
  return {
    ...AuthService.method.getUserInfo,
    methodKind: "server_streaming" as const,
    name: "WatchUserInfo",
  };
}

async function startWatchUserInfo(token = "token-old") {
  const session = new SessionController(createMemoryAuthStorage(token));
  const observed = createObservingStreamFetch();
  const transport = createAdminTransport({
    baseUrl: "http://127.0.0.1:9",
    session,
    fetch: observed.fetch,
  });
  const response = await transport.stream(
    watchUserInfoMethod(),
    undefined,
    undefined,
    undefined,
    (async function* () {
      yield create(GetUserInfoRequestSchema);
    })(),
  );
  const iterator = response.message[Symbol.asyncIterator]();
  const pendingRead = iterator.next();
  pendingRead.catch(() => undefined);
  await vi.waitFor(() => {
    expect(observed.requests.length).toBeGreaterThanOrEqual(1);
  });
  return { session, transport, response, iterator, pendingRead, observed };
}

describe("admin transport cancels in-flight streaming Connect requests", () => {
  it("aborts a stream still being consumed when the session is disposed", async () => {
    const { session, pendingRead, observed } = await startWatchUserInfo();
    const inFlight = observed.requests[0];
    session.dispose();
    await inFlight.aborted;
    expect(inFlight.signal.aborted).toBe(true);
    await expect(pendingRead).rejects.toBeTruthy();
  });

  it("aborts a stream still being consumed on logout and keeps a later stream alive", async () => {
    const { session, transport, pendingRead, observed } = await startWatchUserInfo();
    const loggedOut = observed.requests[0];
    session.clearToken();
    await loggedOut.aborted;
    expect(loggedOut.signal.aborted).toBe(true);
    await expect(pendingRead).rejects.toBeTruthy();

    const next = await transport.stream(
      watchUserInfoMethod(),
      undefined,
      undefined,
      undefined,
      (async function* () {
        yield create(GetUserInfoRequestSchema);
      })(),
    );
    expect(observed.requests.length).toBe(2);
    expect(observed.requests[1].signal.aborted).toBe(false);
    expect(observed.requests[1].signal).not.toBe(loggedOut.signal);
    const nextRead = next.message[Symbol.asyncIterator]().next();
    session.dispose();
    await observed.requests[1].aborted;
    await expect(nextRead).rejects.toBeTruthy();
  });

  it("aborts the old stream after a new login while the new stream stays live", async () => {
    const { session, transport, pendingRead, observed } = await startWatchUserInfo("token-old");
    const previous = observed.requests[0];
    session.applyToken("token-new");
    await previous.aborted;
    expect(previous.signal.aborted).toBe(true);
    await expect(pendingRead).rejects.toBeTruthy();

    const next = await transport.stream(
      watchUserInfoMethod(),
      undefined,
      undefined,
      undefined,
      (async function* () {
        yield create(GetUserInfoRequestSchema);
      })(),
    );
    expect(observed.requests.length).toBe(2);
    expect(observed.requests[1].signal.aborted).toBe(false);
    expect(observed.requests[1].signal).not.toBe(previous.signal);
    const nextRead = next.message[Symbol.asyncIterator]().next();
    session.dispose();
    await observed.requests[1].aborted;
    await expect(nextRead).rejects.toBeTruthy();
  });

  it("cleans up after a stream completes and leaves the next epoch stream live", async () => {
    const session = new SessionController(createMemoryAuthStorage("token-old"));
    const observed = createObservingStreamFetch();
    const transport = createAdminTransport({
      baseUrl: "http://127.0.0.1:9",
      session,
      fetch: observed.fetch,
    });
    const response = await transport.stream(
      watchUserInfoMethod(),
      undefined,
      undefined,
      undefined,
      (async function* () {
        yield create(GetUserInfoRequestSchema);
      })(),
    );
    observed.requests[0].closeWithEndStream();
    const iterator = response.message[Symbol.asyncIterator]();
    await expect(iterator.next()).resolves.toMatchObject({ done: true });
    session.applyToken("token-new");
    expect(observed.requests[0].signal.aborted).toBe(false);

    const next = await transport.stream(
      watchUserInfoMethod(),
      undefined,
      undefined,
      undefined,
      (async function* () {
        yield create(GetUserInfoRequestSchema);
      })(),
    );
    expect(observed.requests.length).toBe(2);
    expect(observed.requests[1].signal.aborted).toBe(false);
    const nextRead = next.message[Symbol.asyncIterator]().next();
    session.dispose();
    await observed.requests[1].aborted;
    await expect(nextRead).rejects.toBeTruthy();
  });

  it("cleans up when the consumer returns the iterator", async () => {
    const settle = vi.fn();
    const cancel = vi.fn();
    async function* values(): AsyncGenerator<number> {
      yield 1;
      yield 2;
    }
    const wrapped = wrapAsyncIterable(values(), {
      settleOnComplete: true,
      onSettle: settle,
      onCancel: cancel,
    });
    const iterator = wrapped[Symbol.asyncIterator]();
    await expect(iterator.next()).resolves.toMatchObject({ done: false, value: 1 });
    await iterator.return?.();
    expect(cancel).toHaveBeenCalledOnce();
    expect(settle).toHaveBeenCalledOnce();
  });

  it("cleans up when the iterator completes or throws", async () => {
    const completed = vi.fn();
    const completeCancel = vi.fn();
    const wrappedComplete = wrapAsyncIterable(
      (async function* () {
        yield 1;
      })(),
      { settleOnComplete: true, onSettle: completed, onCancel: completeCancel },
    );
    const completeIt = wrappedComplete[Symbol.asyncIterator]();
    await expect(completeIt.next()).resolves.toMatchObject({ done: false, value: 1 });
    await expect(completeIt.next()).resolves.toMatchObject({ done: true });
    expect(completed).toHaveBeenCalledOnce();
    expect(completeCancel).not.toHaveBeenCalled();

    const settled = vi.fn();
    const cancel = vi.fn();
    const wrappedNextReject = wrapAsyncIterable(
      {
        [Symbol.asyncIterator]() {
          return {
            next() {
              return Promise.reject(new Error("stream failed"));
            },
          };
        },
      },
      { settleOnComplete: true, onSettle: settled, onCancel: cancel },
    );
    await expect(wrappedNextReject[Symbol.asyncIterator]().next()).rejects.toThrow("stream failed");
    expect(settled).toHaveBeenCalledOnce();
    expect(cancel).not.toHaveBeenCalled();

    const thrownSettle = vi.fn();
    const thrownCancel = vi.fn();
    const wrappedConsumerThrow = wrapAsyncIterable(
      (async function* () {
        yield 1;
        yield 2;
      })(),
      { settleOnComplete: true, onSettle: thrownSettle, onCancel: thrownCancel },
    );
    const thrownIt = wrappedConsumerThrow[Symbol.asyncIterator]();
    await expect(thrownIt.next()).resolves.toMatchObject({ done: false, value: 1 });
    await expect(thrownIt.throw!(new Error("stop"))).rejects.toThrow("stop");
    expect(thrownCancel).toHaveBeenCalledOnce();
    expect(thrownSettle).toHaveBeenCalledOnce();
  });

  it("aborts the in-flight fetch when the response iterator is thrown", async () => {
    const { iterator, observed } = await startWatchUserInfo();
    const inFlight = observed.requests[0];
    await expect(iterator.throw?.(new Error("stop"))).rejects.toBeTruthy();
    await inFlight.aborted;
    expect(inFlight.signal.aborted).toBe(true);
  });

  it("aborts the in-flight fetch when the request iterator is thrown", async () => {
    const session = new SessionController(createMemoryAuthStorage("token"));
    const observed = createObservingStreamFetch();
    const local = new AbortController();
    const wrappedRequest = wrapAsyncIterable(
      (async function* () {
        yield create(GetUserInfoRequestSchema);
        await new Promise(() => undefined);
      })(),
      {
        settleOnComplete: false,
        onSettle: () => undefined,
        onCancel: () => {
          if (!local.signal.aborted) local.abort();
        },
      },
    );
    const transport = createAdminTransport({
      baseUrl: "http://127.0.0.1:9",
      session,
      fetch: observed.fetch,
    });
    const response = await transport.stream(
      watchUserInfoMethod(),
      local.signal,
      undefined,
      undefined,
      wrappedRequest,
    );
    const pendingRead = response.message[Symbol.asyncIterator]().next();
    pendingRead.catch(() => undefined);
    await vi.waitFor(() => {
      expect(observed.requests.length).toBeGreaterThanOrEqual(1);
    });
    const inFlight = observed.requests[0];
    await expect(wrappedRequest[Symbol.asyncIterator]().throw!(new Error("stop"))).rejects.toThrow(
      "stop",
    );
    await inFlight.aborted;
    expect(inFlight.signal.aborted).toBe(true);
    await expect(pendingRead).rejects.toBeTruthy();
    session.dispose();
  });
});
