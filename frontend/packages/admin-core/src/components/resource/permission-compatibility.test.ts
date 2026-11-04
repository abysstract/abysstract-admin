import { describe, expect, it } from "vitest";
import { deptSchema } from "../../gen/resource/dept.schema.gen";
import { menuSchema } from "../../gen/resource/menu.schema.gen";
import { roleSchema } from "../../gen/resource/role.schema.gen";
import type { ResourcePermissionMap } from "./metadata";

// Legacy Get permissions are generated from the backend Schema plan instead of
// being inferred from the generic :list default in Resource UI metadata.
const rolePermissionMap: ResourcePermissionMap<
  "system:role",
  "system:role:list" | "system:role:query"
> = {
  list: "system:role:list",
  get: "system:role:query",
  create: "system:role:add",
  update: "system:role:edit",
  delete: "system:role:remove",
  batchDelete: "system:role:remove",
};

void rolePermissionMap;

describe("generated Resource permission compatibility", () => {
  it("projects the legacy query permission for Role/Menu/Dept Get", () => {
    expect(roleSchema.permissions?.list).toBe("system:role:list");
    expect(roleSchema.permissions?.get).toBe("system:role:query");
    expect(menuSchema.permissions?.list).toBe("system:menu:list");
    expect(menuSchema.permissions?.get).toBe("system:menu:query");
    expect(deptSchema.permissions?.list).toBe("system:dept:list");
    expect(deptSchema.permissions?.get).toBe("system:dept:query");
  });
});
