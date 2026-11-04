import { describe, expect, it } from "vitest";

import type { ResourceSchema } from "./schema";
import { configSchema } from "../../gen/resource/config.schema.gen";
import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import { dictTypeSchema } from "../../gen/resource/dict-type.schema.gen";

function field(schema: ResourceSchema, name: string) {
  const result = schema.fields.find((candidate) => candidate.name === name);
  if (result === undefined) {
    throw new Error(`${schema.name} is missing generated field ${name}`);
  }
  return result;
}

describe("generated resource schema projections", () => {
  it("exposes the Registry-owned header and lifecycle metadata", () => {
    expect(dictTypeSchema).toMatchObject({
      name: "DictType",
      permPrefix: "system:dict",
      table: "sys_dict_type",
    });
    expect(field(dictTypeSchema, "dictName")).toMatchObject({
      protoName: "dict_name",
      tsType: "string",
      required: true,
      searchable: true,
      creatable: true,
      updatable: true,
      serverManaged: false,
    });
    expect(field(dictTypeSchema, "id")).toMatchObject({
      tsType: "bigint",
      creatable: false,
      updatable: false,
      serverManaged: true,
    });
  });

  it("projects only Config List capabilities that the API can reach", () => {
    expect(configSchema).toMatchObject({
      name: "Config",
      permPrefix: "system:config",
      table: "sys_config",
    });
    expect(field(configSchema, "configName")).toMatchObject({
      searchable: true,
      filterable: false,
      sortable: false,
      creatable: true,
      updatable: true,
    });
    expect(field(configSchema, "configKey")).toMatchObject({
      searchable: true,
      filterable: false,
      sortable: false,
      creatable: true,
      updatable: false,
    });
    expect(field(configSchema, "configType")).toMatchObject({
      creatable: true,
      updatable: false,
    });
  });

  it("uses resolved camelCase relation names and preserves frontend scalar types", () => {
    expect(dictDataSchema).toMatchObject({
      name: "DictData",
      permPrefix: "system:dict",
      table: "sys_dict_data",
    });
    expect(field(dictDataSchema, "dictType")).toMatchObject({
      protoName: "dict_type",
      tsType: "string",
      relation: {
        kind: "resource",
        target: "DictType",
        valueField: "dictType",
        labelField: "dictName",
      },
      creatable: true,
      updatable: false,
      serverManaged: false,
    });
    expect(field(dictDataSchema, "id").tsType).toBe("bigint");
    expect(field(dictDataSchema, "dictSort").tsType).toBe("number");
    expect(field(dictDataSchema, "createTime").tsType).toBe("string");
  });
});
