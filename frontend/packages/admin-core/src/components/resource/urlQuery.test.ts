import { afterEach, describe, expect, it } from "vitest";
import { readUrlFilters, urlFilterParam, writeUrlQuery, type UrlFilterType } from "./urlQuery";

const types: Record<string, UrlFilterType> = {
  deptId: "bigint",
  status: "number",
  enabled: "boolean",
  dictType: "string",
  page: "number",
};

function at(search: string) {
  window.history.replaceState(null, "", `/list${search}`);
}

describe("structured filters in the address bar", () => {
  afterEach(() => at(""));

  it("writes only filters that differ from the defaults, with readable names", () => {
    at("?keep=1");
    writeUrlQuery(
      {
        pageNum: 1,
        pageSize: 20,
        filters: { deptId: 12n, status: 0, enabled: false, dictType: "", page: 2 },
      },
      { pageNum: 1, pageSize: 20, filters: { status: 0 } },
      types,
    );
    const params = new URLSearchParams(window.location.search);
    expect(params.get("deptId")).toBe("12");
    expect(params.has("status")).toBe(false); // equals the default
    expect(params.get("enabled")).toBe("false");
    expect(params.get("dictType")).toBe(""); // empty string is a real filter value
    expect(params.get("f.page")).toBe("2"); // does not collide with the page number
    expect(params.has("page")).toBe(false);
    expect(params.get("keep")).toBe("1"); // unrelated parameters survive
  });

  it("removes filters that were cleared", () => {
    at("?deptId=12&q=x");
    writeUrlQuery({ pageNum: 1, pageSize: 20, keyword: "x" }, { pageNum: 1, pageSize: 20 }, types);
    expect(window.location.search).toBe("?q=x");
  });

  it("reads typed values and ignores invalid ones", () => {
    at("?deptId=9007199254740993&status=2&enabled=true&dictType=sys_x&f.page=3");
    expect(readUrlFilters(types)).toEqual({
      deptId: 9007199254740993n,
      status: 2,
      enabled: true,
      dictType: "sys_x",
      page: 3,
    });
    at("?deptId=abc&status=&enabled=yes&other=1&page=4");
    expect(readUrlFilters(types)).toEqual({});
  });

  it("round-trips through write and read", () => {
    at("");
    const filters = { deptId: -5n, status: 1.5, enabled: true, dictType: "a b&c" };
    writeUrlQuery({ pageNum: 1, pageSize: 20, filters }, { pageNum: 1, pageSize: 20 }, types);
    expect(readUrlFilters(types)).toEqual(filters);
    expect(urlFilterParam("sort")).toBe("f.sort");
    expect(urlFilterParam("deptId")).toBe("deptId");
  });
});
