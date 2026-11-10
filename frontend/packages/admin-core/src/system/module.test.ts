import { describe, expect, it } from "vitest";
import { CORE_SYSTEM_PAGE_KEYS, coreSystemModule } from "./module";

describe("coreSystemModule", () => {
  it("exposes the 12 system component keys as static lazy pages", () => {
    expect(coreSystemModule.name).toBe("system");
    expect(Object.keys(coreSystemModule.pages ?? {})).toEqual([...CORE_SYSTEM_PAGE_KEYS]);
    expect(CORE_SYSTEM_PAGE_KEYS).toHaveLength(12);
    for (const key of CORE_SYSTEM_PAGE_KEYS) {
      const page = coreSystemModule.pages?.[key];
      expect(page).toBeTypeOf("object");
      expect(page).toHaveProperty("$$typeof");
    }
  });

  it("registers the current system menu icons", () => {
    expect(Object.keys(coreSystemModule.icons ?? {}).sort()).toEqual(
      [
        "api",
        "apartment",
        "book",
        "login",
        "menu",
        "notification",
        "profile",
        "setting",
        "team",
        "tool",
        "user",
      ].sort(),
    );
  });
});
