import { describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({
  createDictData: vi.fn(),
  createDictType: vi.fn(),
  deleteDictType: vi.fn(),
  listDictTypes: vi.fn(),
  updateDictData: vi.fn(),
  updateDictType: vi.fn(),
}));

vi.mock("@connectrpc/connect", () => ({
  createClient: vi.fn(() => rpc),
}));

import { createSystemOperations } from "../../system/operations";

const { dictDataRelationLoaders, resourceRaw } = createSystemOperations({} as never);

describe("generated relation loaders", () => {
  it("finds the target standard List through the static registry and keeps all pages available", async () => {
    rpc.listDictTypes.mockResolvedValueOnce({
      items: [{ dictType: "color", dictName: "颜色" }],
      page: { total: 2n, pageNum: 1, pageSize: 1 },
    });

    const result = await dictDataRelationLoaders.dictType.load({
      pageNum: 1,
      pageSize: 1,
      keyword: "颜",
    });

    expect(resourceRaw.DictType.listDictTypes).toBeDefined();
    expect(rpc.listDictTypes).toHaveBeenCalledWith({
      page: { pageNum: 1, pageSize: 1 },
      keyword: "颜",
    });
    expect(result).toEqual({ options: [{ value: "color", label: "颜色" }], nextPage: 2 });
  });

  it("propagates target authorization and network errors instead of returning empty options", async () => {
    const denied = new Error("PermissionDenied");
    rpc.listDictTypes.mockRejectedValueOnce(denied);

    await expect(dictDataRelationLoaders.dictType.load({ pageNum: 2, pageSize: 20 })).rejects.toBe(
      denied,
    );
  });
});
