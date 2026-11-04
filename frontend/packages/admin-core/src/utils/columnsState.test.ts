import { describe, it, expect, beforeEach } from "vitest";
import fc from "fast-check";
import { saveColumns, loadColumns, clearColumns, type ColumnStateMap } from "./columnsState";

// 内存 Storage 替身，便于每个用例隔离。
class MemoryStorage implements Storage {
  private m = new Map<string, string>();
  get length() {
    return this.m.size;
  }
  clear() {
    this.m.clear();
  }
  getItem(key: string): string | null {
    return this.m.has(key) ? this.m.get(key)! : null;
  }
  key(): string | null {
    return null;
  }
  removeItem(key: string) {
    this.m.delete(key);
  }
  setItem(key: string, value: string) {
    this.m.set(key, value);
  }
}

// 属性 16：保存-恢复往返为恒等。
// 对任意 user+page+state：loadColumns(saveColumns(state)) === state。
describe("Property 16: columns state round-trip", () => {
  let storage: Storage;

  beforeEach(() => {
    storage = new MemoryStorage();
  });

  it("空状态往返", () => {
    saveColumns("u1", "page1", {}, storage);
    expect(loadColumns("u1", "page1", storage)).toEqual({});
  });

  it("单列设置往返", () => {
    const state: ColumnStateMap = {
      username: { show: true, order: 1, fixed: null },
    };
    saveColumns("u1", "page1", state, storage);
    expect(loadColumns("u1", "page1", storage)).toEqual(state);
  });

  it("多列设置往返且按 user+page 隔离", () => {
    const s1: ColumnStateMap = {
      a: { show: true, order: 2 },
      b: { show: false },
    };
    const s2: ColumnStateMap = {
      x: { show: true, fixed: "left" },
    };
    saveColumns("u1", "page1", s1, storage);
    saveColumns("u1", "page2", s2, storage);
    saveColumns("u2", "page1", { y: { show: true } }, storage);

    expect(loadColumns("u1", "page1", storage)).toEqual(s1);
    expect(loadColumns("u1", "page2", storage)).toEqual(s2);
    expect(loadColumns("u2", "page1", storage)).toEqual({ y: { show: true } });
  });

  it("fast-check: 任意随机 state 往返恒等", () => {
    const columnStateArb = fc.record({
      show: fc.boolean(),
      order: fc.integer({ min: -100, max: 100 }),
      fixed: fc.constantFrom<"left" | "right" | null>("left", "right", null),
    });

    const stateArb = fc.dictionary(
      fc.string({ minLength: 1, maxLength: 12 }).map((s) => s.replace(/[:/]/g, "_")),
      columnStateArb,
      { minKeys: 0, maxKeys: 10 },
    );

    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 8 }).map((s) => s.replace(/[:/]/g, "_")),
        fc.string({ minLength: 1, maxLength: 8 }).map((s) => s.replace(/[:/]/g, "_")),
        stateArb,
        (user, page, state) => {
          saveColumns(user, page, state, storage);
          const loaded = loadColumns(user, page, storage);
          // 保存-恢复应得到完全相同的对象（键集与值一致）
          expect(loaded).toEqual(state);
        },
      ),
      { numRuns: 200 },
    );
  });

  it("clearColumns 后 load 返回空对象", () => {
    const state: ColumnStateMap = { a: { show: true } };
    saveColumns("u1", "page1", state, storage);
    clearColumns("u1", "page1", storage);
    expect(loadColumns("u1", "page1", storage)).toEqual({});
  });

  it("未保存的 key load 返回空对象（非崩溃）", () => {
    expect(loadColumns("nobody", "never", storage)).toEqual({});
  });

  it("损坏的 JSON load 返回空对象而非抛错", () => {
    storage.setItem("abysstract:columns:u1:page1", "{not-json");
    expect(loadColumns("u1", "page1", storage)).toEqual({});
  });
});
