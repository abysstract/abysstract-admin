import path from "node:path";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: {
      "@abysstract/admin-core/resource/schema": path.resolve(__dirname, "src/resource/schema.ts"),
      "@abysstract/admin-core/resource/operations": path.resolve(
        __dirname,
        "src/resource/operations.ts",
      ),
      "@abysstract/admin-core/resource/relation": path.resolve(
        __dirname,
        "src/resource/relation.ts",
      ),
    },
  },
  test: {
    include: ["src/**/*.test.{ts,tsx}"],
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.tsx"],
    maxWorkers: 2,
    css: true,
    testTimeout: 10000,
  },
});
