import { describe, expect, it } from "vitest";

import dictListSource from "../../pages/system/DictList.tsx?raw";
import dictDataDrawerSource from "../../pages/system/DictDataDrawer.tsx?raw";
import configListSource from "../../pages/system/ConfigList.tsx?raw";
import userListSource from "../../pages/system/UserList.tsx?raw";

/**
 * Verifies Resource component usage directly in the four application pages.
 * Fixture and component-test usage is checked separately.
 */
const pages = [
  { name: "DictList", source: dictListSource, expectedFail: false },
  { name: "DictDataDrawer", source: dictDataDrawerSource, expectedFail: false },
  { name: "ConfigList", source: configListSource, expectedFail: false },
  { name: "UserList", source: userListSource, expectedFail: false },
] as const;

function realResourceUsage(source: string): {
  importsResource: boolean;
  usesResourceTableJSX: boolean;
  usesResourceFormJSX: boolean;
  usesOperationsFacade: boolean;
  hasRawRpc: boolean;
  hasChangedFields: boolean;
  hasManualPageRequestOrOrderBy: boolean;
  hasLegacyProCrud: boolean;
} {
  const importsResource =
    /import\s*\{[^}]*Resource(?:Table|Form)[^}]*\}\s*from ["'][^"']*components\/resource["']/.test(
      source,
    );
  const usesResourceTableJSX = /<ResourceTable\b/.test(source);
  // Standard pages may let ResourceTable own the modal/form renderer. In that
  // path the page consumes the production ResourceForm contract through typed
  // createConfig/editConfig instead of spelling a ResourceForm JSX node.
  const usesResourceFormJSX =
    /<ResourceForm\b/.test(source) || /\b(?:createConfig|editConfig)\s*:/.test(source);
  const importsOperations =
    /useSystemOperations|from ["'][^"']*(?:gen\/resource\/[^"']+operations\.gen|api\/resources)["']/.test(
      source,
    );
  const callsFacade = /\b[A-Za-z]+Operations\.(?:list|get|create|update|delete|batchDelete)\b/.test(
    source,
  );
  const hasRawRpc =
    /\b[A-Za-z]+Operations\.raw\b|from ["'][^"']+\.raw\.gen["']|\b[A-Za-z][A-Za-z0-9]*Client\.(?:create|update|delete|batchDelete|get|list|change|reset)[A-Z]?\w*\s*\(/.test(
      source,
    );
  const hasChangedFields = /\bchangedFields\b/.test(source);
  // ResourceListQuery.orders is the typed ResourceTable sorting contract, not
  // a handwritten protobuf OrderBy. Raw page construction remains rejected by
  // the PageRequest/order type and page object checks below.
  const hasManualPageRequestOrOrderBy = /\b(?:PageRequest|OrderBy)\b|\bpage\s*:\s*\{/.test(source);
  const hasLegacyProCrud =
    /\b(?:ProTable|ModalForm|ProFormText|ProFormTextArea|ProFormSelect)\b/.test(source);

  return {
    importsResource,
    usesResourceTableJSX,
    usesResourceFormJSX,
    usesOperationsFacade:
      importsOperations &&
      (callsFacade || /\boperations\s*=\s*\{[A-Za-z]+Operations\b/.test(source)) &&
      !hasRawRpc,
    hasRawRpc,
    hasChangedFields,
    hasManualPageRequestOrOrderBy,
    hasLegacyProCrud,
  };
}

describe("application page Resource consumption", () => {
  for (const page of pages) {
    const test = page.expectedFail ? it.fails : it;
    test(`${page.name} imports and consumes production Resource UI`, () => {
      expect(realResourceUsage(page.source)).toEqual({
        importsResource: true,
        usesResourceTableJSX: true,
        usesResourceFormJSX: true,
        usesOperationsFacade: true,
        hasRawRpc: false,
        hasChangedFields: false,
        hasManualPageRequestOrOrderBy: false,
        hasLegacyProCrud: false,
      });
    });
  }
});
