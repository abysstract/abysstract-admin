import { describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({
  batchDeleteConfigs: vi.fn(),
  batchDeleteDictData: vi.fn(),
  batchDeleteDictTypes: vi.fn(),
  createConfig: vi.fn(),
  createDictData: vi.fn(),
  createDictType: vi.fn(),
  deleteConfig: vi.fn(),
  deleteDictData: vi.fn(),
  deleteDictType: vi.fn(),
  getConfig: vi.fn(),
  getDictData: vi.fn(),
  getDictType: vi.fn(),
  listConfigs: vi.fn(),
  listDictData: vi.fn(),
  listDictTypes: vi.fn(),
  updateConfig: vi.fn(),
  updateDictData: vi.fn(),
  updateDictType: vi.fn(),
}));

vi.mock("@connectrpc/connect", () => ({ createClient: vi.fn(() => rpc) }));
import { createSystemOperations } from "../../system/operations";

const {
  configRaw,
  dictDataRaw,
  dictDataRelationLoaders,
  dictTypeRaw,
  resourceOperations,
  resourceRaw,
} = createSystemOperations({} as never);
import { configSchema } from "../../gen/resource/config.schema.gen";
import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import { dictTypeSchema } from "../../gen/resource/dict-type.schema.gen";
import { aRelationLoaders } from "../../test/resource-bidir-sample/a.relations";
import { bRelationLoaders } from "../../test/resource-bidir-sample/b.relations";
import { bidirRawOperations } from "../../test/resource-bidir-sample/registry";

import configOperationsSource from "../../gen/resource/config.operations.gen.ts?raw";
import configRawSource from "../../gen/resource/config.raw.gen.ts?raw";
import configSchemaSource from "../../gen/resource/config.schema.gen.ts?raw";
import dictDataRelationsSource from "../../gen/resource/dict-data.relations.gen.ts?raw";
import registrySource from "../../gen/resource/resource-operations.gen.ts?raw";

describe("Resource generated module topology", () => {
  it("initializes schema, raw operations, facades, registry, loaders, and a bidirectional sample without undefined exports", async () => {
    expect(configSchema.name).toBe("Config");
    expect(dictDataSchema.name).toBe("DictData");
    expect(dictTypeSchema.name).toBe("DictType");
    expect(configRaw.listConfigs).toBeTypeOf("function");
    expect(dictDataRaw.updateDictData).toBeTypeOf("function");
    expect(dictTypeRaw.listDictTypes).toBeTypeOf("function");
    expect(resourceRaw.Config.listConfigs).toBeTypeOf("function");
    expect(resourceRaw.DictType.listDictTypes).toBeTypeOf("function");
    expect(dictDataRelationLoaders.dictType.load).toBeTypeOf("function");

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

    expect(bidirRawOperations.A.list).toBeTypeOf("function");
    expect(bidirRawOperations.B.list).toBeTypeOf("function");
    expect((await aRelationLoaders.b.load({ pageNum: 1, pageSize: 10 })).options[0].value).toBe(2n);
    expect((await bRelationLoaders.a.load({ pageNum: 1, pageSize: 10 })).options[0].value).toBe(1n);
  });

  it("keeps the generated import graph acyclic: pb -> raw -> facade/registry -> relation -> consumers", () => {
    expect(configSchemaSource).not.toMatch(/\.raw\.gen/);
    expect(configSchemaSource).not.toMatch(/resource-operations\.gen/);
    expect(configSchemaSource).not.toMatch(/\.relations\.gen/);

    expect(configRawSource).not.toMatch(/\.schema\.gen/);
    expect(configRawSource).not.toMatch(/resource-operations\.gen/);
    expect(configRawSource).not.toMatch(/\.relations\.gen/);
    expect(configRawSource).not.toMatch(/api\/transport/);

    expect(configOperationsSource).toMatch(/from "\.\/config\.raw\.gen"/);
    expect(configOperationsSource).toMatch(/from "\.\/config\.schema\.gen"/);
    expect(configOperationsSource).not.toMatch(/resource-operations\.gen/);
    expect(configOperationsSource).not.toMatch(/\.relations\.gen/);

    expect(registrySource).toMatch(/\.raw\.gen/);
    expect(registrySource).toMatch(/\.operations\.gen/);
    expect(registrySource).not.toMatch(/\.schema\.gen/);
    expect(registrySource).not.toMatch(/\.relations\.gen/);

    expect(dictDataRelationsSource).toMatch(/from "\.\/dict-type\.raw\.gen"/);
    expect(dictDataRelationsSource).not.toMatch(/resource-operations\.gen/);
    expect(dictDataRelationsSource).not.toMatch(/\.schema\.gen/);
  });
});
