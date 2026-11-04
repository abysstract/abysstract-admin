// Install the real tarball outside the repository, with exact locked tool versions.
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync, mkdirSync, cpSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { rewriteProtoImports } from "./rewrite-proto-imports.mjs";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const temp = mkdtempSync(path.join(tmpdir(), "admin-proto-install-"));
const evidence = process.env.ADMIN_PROTO_EVIDENCE;
const run = (cmd, args, cwd = temp) =>
  execFileSync(cmd, args, { cwd, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
try {
  const lock = JSON.parse(readFileSync(path.join(root, "frontend/package-lock.json"), "utf8"));
  const packed = JSON.parse(
    run(
      "npm",
      ["pack", "--json", "--pack-destination", temp],
      path.join(root, "frontend/packages/admin-proto"),
    ),
  )[0];
  const allowed = /^(dist\/.*\.(js|d\.ts)|README\.md|LICENSE|package\.json)$/;
  if (packed.files.some((f) => !allowed.test(f.path)))
    throw new Error("unexpected tarball contents");
  writeFileSync(
    path.join(temp, "package.json"),
    JSON.stringify({
      name: "external-proto-install-test",
      private: true,
      type: "module",
      dependencies: {
        "@abysstract/admin-proto": `file:./${packed.filename}`,
        "@bufbuild/protobuf": lock.packages["node_modules/@bufbuild/protobuf"].version,
      },
      devDependencies: { typescript: lock.packages["node_modules/typescript"].version },
    }),
  );
  console.log(run("npm", ["install", "--ignore-scripts"]));
  const fixture = "testpb/test_pb.ts";
  const generated = rewriteProtoImports(
    new Map([
      [fixture, readFileSync(path.join(root, "frontend/tools/fixtures/gen", fixture), "utf8")],
    ]),
    new Set(["abysstract/admin/v1/common_pb.ts"]),
  );
  mkdirSync(path.join(temp, "src/gen/testpb"), { recursive: true });
  writeFileSync(path.join(temp, "src/gen", fixture), generated.get(fixture));
  writeFileSync(
    path.join(temp, "test.ts"),
    `import { create } from '@bufbuild/protobuf';
import { PageRequestSchema, type PageRequest } from '@abysstract/admin-proto/abysstract/admin/v1/common_pb';
import { ConfigService } from '@abysstract/admin-proto/abysstract/admin/v1/config_pb';
import { ListWidgetsRequestSchema } from './src/gen/testpb/test_pb.js';
const page: PageRequest = create(PageRequestSchema, {pageNum: 1, pageSize: 20});
const request = create(ListWidgetsRequestSchema, {page});
if (request.page?.pageNum !== 1 || !ConfigService.methods.length) throw new Error('descriptor import failed');
if (ListWidgetsRequestSchema.fields[0].message !== PageRequestSchema) throw new Error('duplicate system descriptor');
console.log('PASS package + business TS compile, ESM runtime, shared descriptor identity');
`,
  );
  console.log(
    run(path.join(temp, "node_modules/.bin/tsc"), [
      "--strict",
      "--target",
      "ES2020",
      "--module",
      "NodeNext",
      "--moduleResolution",
      "NodeNext",
      "--outDir",
      "dist",
      "test.ts",
    ]),
  );
  console.log(run("node", ["dist/test.js"]));
  console.log(run("npm", ["ls", "--all"]));
  if (evidence) {
    mkdirSync(evidence, { recursive: true });
    for (const f of ["package.json", "package-lock.json", "test.ts"])
      cpSync(path.join(temp, f), path.join(evidence, f));
    writeFileSync(path.join(evidence, "pack.json"), JSON.stringify(packed, null, 2));
  }
} catch (error) {
  console.error(error.stdout?.toString(), error.stderr?.toString());
  throw error;
} finally {
  rmSync(temp, { recursive: true, force: true });
  console.log(`cleaned external install directory ${temp}`);
}
