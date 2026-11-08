import { describe, expect, it, vi } from "vitest";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
  createSystemPresign,
  directUpload,
  resolveApiUrl,
  sendWithXhr,
  DirectUploadError,
  type PresignedUpload,
  type SendUpload,
} from "./directUpload";

const presigned: PresignedUpload = {
  key: "files/2026/11/abc.png",
  url: "/static/files/2026/11/abc.png",
  uploadUrl: "/static/files/2026/11/abc.png?X-Signature=s",
  method: "PUT",
  headers: { "Content-Type": "image/png" },
};

describe("resolveApiUrl", () => {
  it("prefixes root-relative URLs with the API base, keeping its path", () => {
    expect(resolveApiUrl("/static/a.png", "http://localhost:8080")).toBe(
      "http://localhost:8080/static/a.png",
    );
    expect(resolveApiUrl("/static/a.png", "https://example.com/api/")).toBe(
      "https://example.com/api/static/a.png",
    );
  });

  it("keeps absolute and protocol-relative URLs", () => {
    expect(resolveApiUrl("https://cdn.example.com/a.png", "http://localhost:8080")).toBe(
      "https://cdn.example.com/a.png",
    );
    expect(resolveApiUrl("//cdn.example.com/a.png", "http://localhost:8080")).toBe(
      "//cdn.example.com/a.png",
    );
  });
});

describe("directUpload", () => {
  it("presigns, sends the file with the signed headers and returns key and URL", async () => {
    const file = new File(["png"], "photo.png", { type: "image/png" });
    const presign = vi.fn(async () => presigned);
    const send = vi.fn<SendUpload>(async () => {});
    const onProgress = vi.fn();
    const result = await directUpload(
      file,
      { presign, apiBaseUrl: "http://localhost:8080", send },
      { onProgress },
    );

    expect(presign).toHaveBeenCalledWith(file, { signal: undefined });
    expect(send).toHaveBeenCalledWith(
      {
        url: "http://localhost:8080/static/files/2026/11/abc.png?X-Signature=s",
        method: "PUT",
        headers: { "Content-Type": "image/png" },
      },
      file,
      { onProgress },
    );
    expect(result).toEqual({
      key: "files/2026/11/abc.png",
      url: "http://localhost:8080/static/files/2026/11/abc.png",
      size: 3,
    });
  });

  it("does not send when presigning fails", async () => {
    const send = vi.fn<SendUpload>(async () => {});
    const presign = vi.fn(async () => {
      throw new Error("文件类型不在允许的白名单内");
    });
    await expect(
      directUpload(new File(["x"], "a.exe"), { presign, apiBaseUrl: "http://h", send }),
    ).rejects.toThrow("白名单");
    expect(send).not.toHaveBeenCalled();
  });
});

describe("createSystemPresign", () => {
  it("maps CreateUpload to a presigned upload", async () => {
    const expires = new Date("2026-11-08T12:15:00Z");
    const createUpload = vi.fn(async () => ({
      key: "files/a.png",
      url: "https://cdn.example.com/files/a.png",
      uploadUrl: "https://bucket.example.com/files/a.png?X-Amz-Signature=s",
      method: "PUT",
      headers: { "Content-Type": "image/png" },
      expiresAt: timestampFromDate(expires),
    }));
    const presign = createSystemPresign({ createUpload } as never);
    const controller = new AbortController();
    const file = new File(["hello"], "a.png", { type: "image/png" });
    const res = await presign(file, { signal: controller.signal });

    expect(createUpload).toHaveBeenCalledWith(
      { filename: "a.png", contentType: "image/png", size: 5n },
      { signal: controller.signal },
    );
    expect(res.key).toBe("files/a.png");
    expect(res.uploadUrl).toContain("X-Amz-Signature");
    expect(res.expiresAt?.toISOString()).toBe(expires.toISOString());
  });
});

class FakeXhr {
  static last: FakeXhr | null = null;
  method = "";
  url = "";
  headers: Record<string, string> = {};
  body: unknown = null;
  status = 0;
  upload: {
    onprogress: ((e: { loaded: number; total: number; lengthComputable: boolean }) => void) | null;
  } = {
    onprogress: null,
  };
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onabort: (() => void) | null = null;
  aborted = false;
  constructor() {
    FakeXhr.last = this;
  }
  open(method: string, url: string) {
    this.method = method;
    this.url = url;
  }
  setRequestHeader(name: string, value: string) {
    this.headers[name] = value;
  }
  send(body: unknown) {
    this.body = body;
  }
  abort() {
    this.aborted = true;
    this.onabort?.();
  }
}

describe("sendWithXhr", () => {
  const target = {
    url: "https://bucket/a.png",
    method: "PUT",
    headers: { "Content-Type": "image/png" },
  };

  function withFakeXhr(run: () => Promise<void>) {
    const original = globalThis.XMLHttpRequest;
    globalThis.XMLHttpRequest = FakeXhr as never;
    return run().finally(() => {
      globalThis.XMLHttpRequest = original;
    });
  }

  it("sends the body with signed headers and reports progress", () =>
    withFakeXhr(async () => {
      const file = new Blob(["data"]);
      const onProgress = vi.fn();
      const pending = sendWithXhr(target, file, { onProgress });
      const xhr = FakeXhr.last!;
      expect(xhr.method).toBe("PUT");
      expect(xhr.url).toBe("https://bucket/a.png");
      expect(xhr.headers).toEqual({ "Content-Type": "image/png" });
      expect(xhr.headers).not.toHaveProperty("Authorization");
      expect(xhr.body).toBe(file);
      xhr.upload.onprogress?.({ loaded: 2, total: 4, lengthComputable: true });
      expect(onProgress).toHaveBeenCalledWith(2, 4);
      xhr.status = 200;
      xhr.onload?.();
      await expect(pending).resolves.toBeUndefined();
    }));

  it("rejects with the HTTP status on failure", () =>
    withFakeXhr(async () => {
      const pending = sendWithXhr(target, new Blob(["x"]), {});
      const xhr = FakeXhr.last!;
      xhr.status = 403;
      xhr.onload?.();
      const error = await pending.catch((e: unknown) => e);
      expect(error).toBeInstanceOf(DirectUploadError);
      expect((error as DirectUploadError).status).toBe(403);
    }));

  it("aborts with the caller's signal", () =>
    withFakeXhr(async () => {
      const controller = new AbortController();
      const pending = sendWithXhr(target, new Blob(["x"]), { signal: controller.signal });
      const xhr = FakeXhr.last!;
      controller.abort(new Error("stop"));
      expect(xhr.aborted).toBe(true);
      await expect(pending).rejects.toThrow("stop");
    }));

  it("does not start when the signal is already aborted", () =>
    withFakeXhr(async () => {
      FakeXhr.last = null;
      const controller = new AbortController();
      controller.abort();
      await expect(
        sendWithXhr(target, new Blob(["x"]), { signal: controller.signal }),
      ).rejects.toBeDefined();
      expect(FakeXhr.last).toBeNull();
    }));
});
