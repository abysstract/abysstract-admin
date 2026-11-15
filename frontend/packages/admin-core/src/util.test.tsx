import { describe, expect, it } from "vitest";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";

import { fmtTime, fmtTimeText, formatDateTime } from "./util";

describe("time formatting", () => {
  it("uses YYYY-MM-DD HH:mm:ss regardless of the browser locale", () => {
    const date = new Date(2026, 0, 5, 7, 8, 9);
    expect(formatDateTime(date)).toBe("2026-01-05 07:08:09");
    expect(fmtTime(timestampFromDate(date))).toBe("2026-01-05 07:08:09");
  });

  it("shows - for empty values", () => {
    expect(fmtTime(undefined)).toBe("-");
    expect(fmtTimeText("")).toBe("-");
  });

  it("normalises server time strings and keeps unparseable text", () => {
    expect(fmtTimeText("2026-01-05 07:08:09")).toBe("2026-01-05 07:08:09");
    expect(fmtTimeText(new Date(2026, 0, 5, 7, 8, 9).toISOString())).toBe("2026-01-05 07:08:09");
    expect(fmtTimeText("昨天")).toBe("昨天");
  });
});
