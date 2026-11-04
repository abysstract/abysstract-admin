import source from "./RoleList.tsx?raw";
import { describe, expect, it } from "vitest";

describe("RoleList Resource boundary", () => {
  it("keeps standard CRUD behind generated schema, operations and ResourceTable", () => {
    expect(source).toContain("useSystemOperations");
    expect(source).not.toContain('from "../../api/resources"');
    expect(source).toContain('from "../../gen/resource/role.schema.gen"');
    expect(source).toContain("<ResourceTable");
    expect(source).toContain("roleOperations.custom.changeRoleStatus");
    expect(source).toContain("RoleStatusSwitch");
    expect(source).toContain("render: ({ entity, reload })");
    expect(source).not.toContain("roleClient");
    expect(source).not.toContain("roleOperations.raw");
    expect(source).not.toContain("PageRequest");
    expect(source).not.toContain("OrderBy");
    expect(source).not.toContain("changedFields");
    expect(source).not.toMatch(/has\(\s*["']system:role:(list|add|edit|remove)["']\s*\)/);
  });
});
