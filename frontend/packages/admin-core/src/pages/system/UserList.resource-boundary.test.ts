import source from "./UserList.tsx?raw";
import { describe, expect, it } from "vitest";

describe("UserList Resource boundary", () => {
  it("keeps ordinary fields behind generated schema/form and typed CUSTOM workflows", () => {
    expect(source).toContain("useSystemOperations");
    expect(source).not.toContain('from "../../api/resources"');
    expect(source).toContain('from "../../gen/resource/user.schema.gen"');
    expect(source).toContain("<ResourceTable");
    expect(source).toContain("<ResourceForm");
    expect(source).toContain("readOnly");
    expect(source).toContain("userOperations.custom.createUser");
    expect(source).toContain("userOperations.custom.updateUser");
    expect(source).toContain("userOperations.custom.changeUserStatus");
    expect(source).toContain("userOperations.custom.resetPassword");
    expect(source).toContain("userOperations.custom.batchDeleteUsers");
    expect(source).not.toContain('batchDelete("system:user"');
    expect(source).not.toContain("userClient");
    expect(source).not.toContain("roleClient");
    expect(source).not.toContain("deptClient");
    expect(source).not.toContain("userOperations.raw");
    expect(source).not.toContain("PageRequest");
    expect(source).not.toContain("OrderBy");
    expect(source).not.toContain("changedFields");
    expect(source).not.toMatch(/has\(\s*["']system:user:(list|add|edit|remove)["']\s*\)/);
  });
});
