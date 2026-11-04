import { describe, expect, it, vi } from "vitest";

import { createRelationOptionLoader } from "./relation";

describe("createRelationOptionLoader", () => {
  it("does not forward keyword when remote search is disabled and preserves bigint values", async () => {
    const loadPage = vi.fn().mockResolvedValue({
      options: [{ value: 9007199254740993n, label: "large" }],
      nextPage: 2,
    });
    const loader = createRelationOptionLoader({ remoteSearchEnabled: false, loadPage });

    const result = await loader.load({ pageNum: 1, pageSize: 1, keyword: "ignored" });

    expect(loadPage).toHaveBeenCalledWith({ pageNum: 1, pageSize: 1 });
    expect(result.options[0].value).toBe(9007199254740993n);
    expect(typeof result.options[0].value).toBe("bigint");
    expect(result.nextPage).toBe(2);
  });

  it("passes keyword only for remote-searchable targets and propagates errors", async () => {
    const denied = new Error("PermissionDenied");
    const loadPage = vi.fn().mockRejectedValue(denied);
    const loader = createRelationOptionLoader({ remoteSearchEnabled: true, loadPage });

    await expect(loader.load({ pageNum: 2, pageSize: 20, keyword: "find" })).rejects.toBe(denied);
    expect(loadPage).toHaveBeenCalledWith({ pageNum: 2, pageSize: 20, keyword: "find" });
  });
});
