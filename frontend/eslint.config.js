import js from "@eslint/js";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import prettierConfig from "eslint-config-prettier";

export default tseslint.config(
  {
    ignores: [
      "tools/fixtures/gen/**",
      "packages/admin-core/src/gen/**",
      "packages/*/dist/**",
      "packages/*/node_modules/**",
      "packages/admin-proto/src/**",
      "apps/*/dist/**",
      "apps/*/src/modules/*/gen/**",
      "apps/*/node_modules/**",
      "dist/**",
      "node_modules/**",
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["tools/**/*.mjs"],
    languageOptions: {
      globals: { process: "readonly", console: "readonly", Buffer: "readonly", URL: "readonly" },
    },
  },
  {
    files: ["packages/admin-core/src/**/*.{ts,tsx}", "apps/reference/src/**/*.{ts,tsx}"],
    plugins: {
      "react-hooks": reactHooks,
    },
    rules: {
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
  prettierConfig,
);
