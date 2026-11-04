import source from "./DeptList.tsx?raw";
import { describe, expect, it } from "vitest";

describe("DeptList Resource boundary", () => {
  it("uses generated schema/facade and keeps standard CRUD out of the page", () => {
    expect(source).toContain("useSystemOperations");
    expect(source).not.toContain('from "../../api/resources"');
    expect(source).toContain('from "../../gen/resource/dept.schema.gen"');
    expect(source).toContain("<ResourceTable");
    expect(source).toContain("treeData");
    expect(source).toContain("buildDeptTree");
    expect(source).not.toContain("<DeptTreeView");
    expect(source).not.toContain("deptOperations.custom");
    expect(source).not.toContain("Tabs");
    expect(source).not.toContain("deptClient");
    expect(source).not.toContain("PageRequest");
    expect(source).not.toContain("OrderBy");
    expect(source).not.toContain("changedFields");
    expect(source).not.toContain("deptOperations.raw");
    expect(source).not.toMatch(/has\(\s*["']system:dept:(list|add|edit|remove)["']\s*\)/);
  });
});
