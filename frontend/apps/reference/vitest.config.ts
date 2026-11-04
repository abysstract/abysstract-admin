import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    passWithNoTests: true,
    include: ["src/**/*.test.{ts,tsx}"],
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.tsx"],
    maxWorkers: 1,
    css: true,
    testTimeout: 15000,
    server: {
      deps: {
        inline: ["@abysstract/admin-core"],
      },
    },
  },
});
