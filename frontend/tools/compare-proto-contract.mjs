// Compare the generated wire descriptors against a specified Git baseline.
import { execFileSync } from "node:child_process";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { fromBinary, toJson } from "@bufbuild/protobuf";
import { FileDescriptorProtoSchema } from "@bufbuild/protobuf/wkt";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const ref = process.argv[2];
if (!ref) throw new Error("usage: node compare-proto-contract.mjs <baseline-ref>");
const directory = path.join(root, "frontend/packages/admin-proto/src/abysstract/admin/v1");
function descriptor(text) {
  const match = text.match(/fileDesc\("([A-Za-z0-9+/=]+)"/);
  if (!match) throw new Error("missing generated fileDesc");
  return toJson(
    FileDescriptorProtoSchema,
    fromBinary(FileDescriptorProtoSchema, Buffer.from(match[1], "base64")),
  );
}
for (const file of readdirSync(directory)
  .filter((f) => f.endsWith("_pb.ts"))
  .sort()) {
  const old = descriptor(
    execFileSync("git", ["show", `${ref}:frontend/src/gen/abysstract/admin/v1/${file}`], {
      cwd: root,
      encoding: "utf8",
    }),
  );
  const current = descriptor(readFileSync(path.join(directory, file), "utf8"));
  const before = old.options.goPackage;
  old.options.goPackage = before.replace("/backend/internal/gen/", "/backend/gen/");
  old.dependency?.sort();
  current.dependency?.sort();
  if (JSON.stringify(old) !== JSON.stringify(current))
    throw new Error(`unexpected descriptor change: ${file}`);
  console.log(
    `PASS ${file}: go_package ${before} -> ${current.options.goPackage}; all other descriptor content matches (import ordering normalized)`,
  );
}
