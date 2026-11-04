import { describe, expect, it } from "vitest";
import { formatDecimalBigint, parseDecimalBigint } from "./bigint";

describe("decimal bigint conversion", () => {
  it("parses decimal strings without Number", () => {
    expect(parseDecimalBigint("9007199254740993")).toBe(9007199254740993n);
    expect(parseDecimalBigint(" 0 ")).toBe(0n);
    expect(parseDecimalBigint("")).toBeUndefined();
    expect(parseDecimalBigint("1e2")).toBeUndefined();
    expect(parseDecimalBigint("1.5")).toBeUndefined();
  });

  it("formats bigint as decimal text", () => {
    expect(formatDecimalBigint(9007199254740993n)).toBe("9007199254740993");
    expect(formatDecimalBigint(undefined)).toBe("");
  });
});
