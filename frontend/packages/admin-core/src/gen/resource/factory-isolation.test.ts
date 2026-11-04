import { describe, expect, it, vi } from "vitest";
import type { Transport } from "@connectrpc/connect";

const calls = vi.hoisted(() => [] as Array<{ transport: unknown; method: string }>);

vi.mock("@connectrpc/connect", () => ({
  createClient: (_service: unknown, transport: Transport) => ({
    listConfigs: () => {
      calls.push({ transport, method: "listConfigs" });
      return Promise.resolve({ items: [], page: { total: 0n, pageNum: 1, pageSize: 10 } });
    },
    getConfig: () => {
      calls.push({ transport, method: "getConfig" });
      return Promise.resolve({ item: { id: 1n } });
    },
    createConfig: () => Promise.resolve({ item: { id: 1n } }),
    updateConfig: () => Promise.resolve({ item: { id: 1n } }),
    deleteConfig: () => Promise.resolve({}),
    batchDeleteConfigs: () => Promise.resolve({}),
  }),
}));

import { createConfigOperations } from "./config.operations.gen";
import { createConfigRaw } from "./config.raw.gen";
import { createResourceOperations } from "./resource-operations.gen";

describe("generated request factories isolate transports", () => {
  it("routes each instance through its own transport", async () => {
    calls.length = 0;
    const transportA = { name: "a" } as unknown as Transport;
    const transportB = { name: "b" } as unknown as Transport;
    const rawA = createConfigRaw(transportA);
    const rawB = createConfigRaw(transportB);
    const opsA = createConfigOperations(rawA);
    const opsB = createConfigOperations(rawB);
    await opsA.get(1n);
    await opsB.get(2n);
    expect(calls).toEqual([
      { transport: transportA, method: "getConfig" },
      { transport: transportB, method: "getConfig" },
    ]);
    const setA = createResourceOperations(transportA);
    const setB = createResourceOperations(transportB);
    await setA.raw.Config.listConfigs({ page: { pageNum: 1, pageSize: 10 } });
    await setB.raw.Config.listConfigs({ page: { pageNum: 1, pageSize: 10 } });
    expect(calls.at(-2)?.transport).toBe(transportA);
    expect(calls.at(-1)?.transport).toBe(transportB);
  });
});
