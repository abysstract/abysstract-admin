import source from "./MenuList.tsx?raw";
import { describe, expect, it } from "vitest";

describe("MenuList Resource boundary", () => {
  it("keeps CRUD and tree projection behind typed Resource boundaries", () => {
    expect(source).toContain("useSystemOperations");
    expect(source).not.toContain('from "../../api/resources"');
    expect(source).toContain('from "../../gen/resource/menu.schema.gen"');
    expect(source).toContain("<ResourceTable");
    expect(source).toContain("treeData");
    expect(source).toContain("buildMenuTree");
    expect(source).not.toContain("menuClient");
    expect(source).not.toContain("menuOperations.raw");
    expect(source).not.toContain("PageRequest");
    expect(source).not.toContain("OrderBy");
    expect(source).not.toContain("changedFields");
    expect(source).not.toMatch(/has\(\s*["']system:menu:(list|add|edit|remove)["']\s*\)/);
  });
});
