import { test } from "node:test";
import assert from "node:assert/strict";
import { rewriteProtoImports } from "./rewrite-proto-imports.mjs";
const common = "abysstract/admin/v1/common_pb.ts";
const business = "company/product/v1/product_pb.ts";
test("system imports and exports become Node ESM paths; data is preserved", () => {
  const text = `// header\nimport {x} from './common_pb';\nexport {x} from './common_pb.js';\nconst data = './common_pb';\nimport {y} from '@bufbuild/protobuf/wkt';`;
  const files = new Map([
    [common, ""],
    ["abysstract/admin/v1/user_pb.ts", text],
  ]);
  const output = rewriteProtoImports(files);
  assert.equal(
    output.get("abysstract/admin/v1/user_pb.ts"),
    text.replace("from './common_pb'", "from './common_pb.js'"),
  );
  assert.deepEqual(rewriteProtoImports(output), output);
});
test("business owns internal messages and imports shared descriptors from package", () => {
  const files = new Map([
    [
      business,
      `import {p} from '../../../abysstract/admin/v1/common_pb';\nexport {x} from './other_pb';`,
    ],
    ["company/product/v1/other_pb.ts", ""],
  ]);
  assert.equal(
    rewriteProtoImports(files, new Set([common])).get(business),
    `import {p} from '@abysstract/admin-proto/abysstract/admin/v1/common_pb';\nexport {x} from './other_pb.js';`,
  );
});
test("unknown dependency fails without mutating inputs", () => {
  const files = new Map([[business, `import {p} from './missing_pb';`]]);
  const before = new Map(files);
  assert.throws(() => rewriteProtoImports(files), /unknown relative Proto dependency/);
  assert.deepEqual(files, before);
});

test("unknown system Proto path fails", () => {
  assert.throws(
    () =>
      rewriteProtoImports(
        new Map([[business, "export {x} from '../../../abysstract/admin/v1/missing_pb';"]]),
        new Set([common]),
      ),
    /unknown relative Proto dependency/,
  );
});

test("business Proto dependencies resolve to the owning module output", () => {
  const file = "company/sales/v1/order_pb.ts";
  const source = "import { Product } from '../../product/v1/product_pb.js';";
  const output = rewriteProtoImports(
    new Map([[file, source]]),
    new Set(),
    new Map([
      [
        "company/product/v1/product_pb.ts",
        "/app/modules/product/gen/proto/company/product/v1/product_pb.ts",
      ],
    ]),
    "/app/modules/sales/gen/proto",
  );
  assert.equal(
    output.get(file),
    "import { Product } from '../../../../../../product/gen/proto/company/product/v1/product_pb.js';",
  );
});
