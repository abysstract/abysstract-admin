import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const moduleJs = path.join(frontendRoot, "packages/admin-core/dist/system/module.js");
if (!existsSync(moduleJs)) {
  throw new Error("admin-core dist/system/module.js 不存在，先构建公共包");
}

const source = readFileSync(moduleJs, "utf8");
const imports = [...source.matchAll(/import\((['"])(.+?)\1\)/g)].map((match) => match[2]);
const expected = [
  "../pages/system/UserList",
  "../pages/system/RoleList",
  "../pages/system/MenuList",
  "../pages/system/DeptList",
  "../pages/system/DictList",
  "../pages/system/ConfigList",
  "../pages/system/NoticeList",
  "../pages/system/OperLogList",
  "../pages/system/LoginLogList",
  "../pages/dashboard",
  "../pages/profile",
];

function resolveLazyTarget(spec) {
  const base = path.resolve(path.dirname(moduleJs), spec);
  const candidates = [base, `${base}.js`, path.join(base, "index.js")];
  return candidates.find((file) => existsSync(file));
}

for (const spec of expected) {
  if (!imports.includes(spec)) {
    throw new Error(`coreSystemModule 缺少静态 lazy import: ${spec}; 实际=${imports.join(",")}`);
  }
  const target = resolveLazyTarget(spec);
  if (!target) {
    throw new Error(`lazy 目标不存在: ${spec}`);
  }
}

if (imports.length !== expected.length) {
  throw new Error(`lazy import 数量为 ${imports.length}，期望 ${expected.length}`);
}

console.log(`ok ${expected.length} static lazy pages`);
