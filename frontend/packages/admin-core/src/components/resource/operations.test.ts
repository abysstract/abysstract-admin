import { describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({
  createDictType: vi.fn(),
  deleteDictType: vi.fn(),
  listDictTypes: vi.fn(),
  updateDictType: vi.fn(),
}));

vi.mock("@connectrpc/connect", () => ({
  createClient: vi.fn(() => rpc),
}));

import { createSystemOperations } from "../../system/operations";
import type { DictTypeUpdateValues } from "../../gen/resource/dict-type.operations.gen";

const { dictTypeRaw, dictTypeBuildUpdateRequest, dictDataBuildUpdateRequest } =
  createSystemOperations({} as never);

describe("generated Resource operations", () => {
  it("Raw operation forwards explicit zero values without defaults or cleanup", async () => {
    const request = { id: 7n, dictName: "", remark: "" };
    await dictTypeRaw.updateDictType(request);

    expect(rpc.updateDictType).toHaveBeenCalledOnce();
    expect(rpc.updateDictType).toHaveBeenCalledWith(request);
  });

  it("Update adapter sends identifier and only touched updatable fields", () => {
    const values: DictTypeUpdateValues = {
      dictName: "original name",
      remark: "",
    };

    const request = dictTypeBuildUpdateRequest(7n, values, new Set(["remark"]));

    expect(request).toEqual({ id: 7n, remark: "" });
    expect(Object.prototype.hasOwnProperty.call(request, "dictName")).toBe(false);
    expect(Object.prototype.hasOwnProperty.call(request, "dictType")).toBe(false);
  });

  it("Update adapter preserves an explicitly touched empty string", () => {
    const values: DictTypeUpdateValues = {
      dictName: "",
      remark: "unchanged",
    };

    const request = dictTypeBuildUpdateRequest(9n, values, new Set(["dictName"]));

    expect(request).toEqual({ id: 9n, dictName: "" });
    expect(Object.prototype.hasOwnProperty.call(request, "remark")).toBe(false);
  });
});

import type { DictDataUpdateValues } from "../../gen/resource/dict-data.operations.gen";

describe("generated numeric update presence", () => {
  it("keeps an explicitly touched numeric zero as an own request property", () => {
    const values: DictDataUpdateValues = {
      dictSort: 0,
      dictLabel: "label",
      dictValue: "value",
      isDefault: 1,
      remark: "remark",
    };

    const request = dictDataBuildUpdateRequest(21n, values, new Set(["dictSort"]));

    expect(request).toEqual({ id: 21n, dictSort: 0 });
    expect(Object.prototype.hasOwnProperty.call(request, "dictSort")).toBe(true);
    expect(Object.prototype.hasOwnProperty.call(request, "remark")).toBe(false);
  });
});
