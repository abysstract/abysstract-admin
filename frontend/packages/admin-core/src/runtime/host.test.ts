import { describe, expect, it } from "vitest";
import { normalizeApiBaseUrl } from "./host";

describe("normalizeApiBaseUrl", () => {
  it("keeps absolute http(s) bases without the trailing slash", () => {
    expect(normalizeApiBaseUrl("http://localhost:8080")).toBe("http://localhost:8080");
    expect(normalizeApiBaseUrl("https://api.example.com/admin/")).toBe(
      "https://api.example.com/admin",
    );
  });

  it("resolves same-origin paths against the page URL", () => {
    const page = "https://review.example.com:8443/system/user?page=2#top";
    expect(normalizeApiBaseUrl("/", page)).toBe("https://review.example.com:8443");
    expect(normalizeApiBaseUrl("/api", page)).toBe("https://review.example.com:8443/api");
    expect(normalizeApiBaseUrl(" /api/ ", page)).toBe("https://review.example.com:8443/api");
  });

  it("uses window.location when no page URL is given", () => {
    expect(normalizeApiBaseUrl("/api")).toBe(`${window.location.origin}/api`);
  });

  it("rejects bare hosts, protocol-relative and non-http bases", () => {
    expect(() => normalizeApiBaseUrl("localhost:8080")).toThrow(/绝对地址/);
    expect(() => normalizeApiBaseUrl("api")).toThrow(/绝对地址/);
    expect(() => normalizeApiBaseUrl("//evil.example.com")).toThrow(/同源路径/);
    expect(() => normalizeApiBaseUrl("ftp://example.com")).toThrow(/绝对地址/);
  });
});
