import { describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({
  listConfigs: vi.fn(),
  createConfig: vi.fn(),
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  deleteConfig: vi.fn(),
  batchDeleteConfigs: vi.fn(),
  listDictTypes: vi.fn(),
  createDictType: vi.fn(),
  getDictType: vi.fn(),
  updateDictType: vi.fn(),
  deleteDictType: vi.fn(),
  batchDeleteDictTypes: vi.fn(),
  listDictData: vi.fn(),
  createDictData: vi.fn(),
  getDictData: vi.fn(),
  updateDictData: vi.fn(),
  deleteDictData: vi.fn(),
  batchDeleteDictData: vi.fn(),
}));

vi.mock("@connectrpc/connect", () => ({
  createClient: vi.fn(() => rpc),
}));

import { create } from "@bufbuild/protobuf";
import {
  ConfigSchema,
  CreateConfigRequestSchema,
} from "@abysstract/admin-proto/abysstract/admin/v1/config_pb";
import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import { createSystemOperations } from "../../system/operations";

const { configOperations, dictDataOperations, dictTypeOperations, resourceOperations } =
  createSystemOperations({} as never);
import type { ResourceListQuery, ResourceOrder } from "./operations";
import type { SortableFieldName } from "./schema";

const largeId = 9007199254740993n;

const configItem = create(ConfigSchema, {
  id: largeId,
  configName: "站点名称",
  configKey: "site.name",
  configValue: "Abysstract",
  configType: 1,
  remark: "",
});

describe("generated unified Resource operations facade", () => {
  it("maps a Config list query onto the raw List RPC and keeps the bigint total", async () => {
    rpc.listConfigs.mockResolvedValueOnce({
      items: [configItem],
      page: { total: largeId, pageNum: 2, pageSize: 10 },
    });

    const result = await configOperations.list({
      pageNum: 2,
      pageSize: 10,
      keyword: "站点",
    });

    expect(rpc.listConfigs).toHaveBeenCalledWith({
      page: { pageNum: 2, pageSize: 10 },
      keyword: "站点",
    });
    expect(result.total).toBe(largeId);
    expect(typeof result.total).toBe("bigint");
    expect(result.items).toEqual([configItem]);
  });

  it("maps DictData page/keyword/orders/filters onto the typed raw request", async () => {
    rpc.listDictData.mockResolvedValueOnce({
      items: [],
      page: { total: 7n, pageNum: 1, pageSize: 20 },
    });

    const result = await dictDataOperations.list({
      pageNum: 1,
      pageSize: 20,
      keyword: "启",
      orders: [{ field: "dictSort", desc: true }],
      filters: { dictType: "notice" },
    });

    expect(rpc.listDictData).toHaveBeenCalledWith({
      page: { pageNum: 1, pageSize: 20 },
      keyword: "启",
      orders: [{ field: "dictSort", desc: true }],
      dictType: "notice",
    });
    expect(result.total).toBe(7n);
    expect(result.items).toEqual([]);
  });

  it("maps DictType filters without inventing orders", async () => {
    rpc.listDictTypes.mockResolvedValueOnce({
      items: [],
      page: { total: 0n, pageNum: 1, pageSize: 10 },
    });

    await dictTypeOperations.list({
      pageNum: 1,
      pageSize: 10,
      filters: { dictType: "status" },
    });

    expect(rpc.listDictTypes).toHaveBeenCalledWith({
      page: { pageNum: 1, pageSize: 10 },
      keyword: "",
      orders: [],
      dictType: "status",
    });
  });

  it("keyOf keeps the bigint primary key untouched", () => {
    expect(configOperations.keyOf(configItem)).toBe(largeId);
    expect(typeof configOperations.keyOf(configItem)).toBe("bigint");
  });

  it("create calls the raw create RPC and unwraps the item", async () => {
    rpc.createConfig.mockResolvedValueOnce({ item: configItem });

    await expect(
      configOperations.create(
        create(CreateConfigRequestSchema, {
          configName: "站点名称",
          configKey: "site.name",
          configValue: "Abysstract",
          configType: 0,
          remark: "",
        }),
      ),
    ).resolves.toBe(configItem);

    expect(rpc.createConfig).toHaveBeenCalledWith(
      expect.objectContaining({
        configName: "站点名称",
        configKey: "site.name",
        configValue: "Abysstract",
        configType: 0,
        remark: "",
      }),
    );
  });

  it("get calls the raw Get RPC and unwraps the item", async () => {
    rpc.getConfig.mockResolvedValueOnce({ item: configItem });

    await expect(configOperations.get(largeId)).resolves.toBe(configItem);
    expect(rpc.getConfig).toHaveBeenCalledWith({ id: largeId });
  });

  it("update reuses the generated builder and writes only touched fields", async () => {
    rpc.updateConfig.mockResolvedValueOnce({ item: configItem });

    const result = await configOperations.update({
      key: 7n,
      values: { configName: "新名称", configValue: "", remark: "保留" },
      touched: new Set(["configValue"]),
      dirty: new Set(["configValue"]),
    });

    expect(rpc.updateConfig).toHaveBeenCalledWith({ id: 7n, configValue: "" });
    expect(result).toBe(configItem);
  });

  it("update preserves explicit 0/empty-string touched values and omits untouched keys", async () => {
    rpc.updateDictData.mockResolvedValueOnce({
      item: {
        id: 8n,
        dictSort: 0,
        dictLabel: "l",
        dictValue: "v",
        dictType: "t",
        isDefault: 0,
        remark: "",
      },
    });

    await dictDataOperations.update({
      key: 8n,
      values: {
        dictSort: 0,
        dictLabel: "l",
        dictValue: "v",
        isDefault: 0,
        remark: "",
      },
      touched: new Set(["dictSort", "isDefault", "remark"]),
      dirty: new Set(["dictSort"]),
    });

    const sent = rpc.updateDictData.mock.calls[0][0];
    expect(sent).toEqual({ id: 8n, dictSort: 0, isDefault: 0, remark: "" });
    expect(Object.prototype.hasOwnProperty.call(sent, "dictLabel")).toBe(false);
    expect(Object.prototype.hasOwnProperty.call(sent, "dictValue")).toBe(false);
  });

  it("keeps the identifier/values/touched update shape for pre-migration page callers", async () => {
    rpc.updateDictType.mockResolvedValueOnce({
      item: { id: 9n, dictName: "状态", dictType: "status", remark: "", status: 1 },
    });

    await dictTypeOperations.update(9n, { dictName: "状态", remark: "" }, new Set(["remark"]));

    expect(rpc.updateDictType).toHaveBeenCalledWith({ id: 9n, remark: "" });
  });

  it("delete and batchDelete call the raw RPCs with bigint keys", async () => {
    rpc.deleteConfig.mockResolvedValueOnce({});
    await configOperations.delete(5n);
    expect(rpc.deleteConfig).toHaveBeenCalledWith({ id: 5n });

    rpc.batchDeleteConfigs.mockResolvedValueOnce({});
    await configOperations.batchDelete([5n, 6n]);
    expect(rpc.batchDeleteConfigs).toHaveBeenCalledWith({ ids: [5n, 6n] });
  });

  it("exposes all three resources through the static facade registry", () => {
    for (const name of ["Config", "DictType", "DictData"] as const) {
      const entry = resourceOperations[name];
      expect(entry).toBeDefined();
      for (const method of [
        "keyOf",
        "list",
        "get",
        "create",
        "update",
        "delete",
        "batchDelete",
      ] as const) {
        expect(entry[method]).toBeTypeOf("function");
      }
    }
  });

  it("rejects non-sortable order fields at compile time", () => {
    type DictDataOrderField = SortableFieldName<typeof dictDataSchema>;
    const badOrder: ResourceOrder<DictDataOrderField> = {
      // @ts-expect-error dictLabel is not a sortable field of DictData
      field: "dictLabel",
      desc: false,
    };
    const query: ResourceListQuery<{ dictType?: string }, DictDataOrderField> = {
      pageNum: 1,
      pageSize: 10,
      orders: [
        {
          // @ts-expect-error dictLabel is not a sortable field of DictData
          field: "dictLabel",
          desc: false,
        },
      ],
    };
    void badOrder;
    void query;
  });
});
