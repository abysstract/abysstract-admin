import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  realpathSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const allowed = new Set(["package.json", "README.md", "LICENSE"]);

function run(cmd, args, cwd) {
  return execFileSync(cmd, args, { cwd, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
}

function sha256(file) {
  return createHash("sha256").update(readFileSync(file)).digest("hex");
}

function walkFiles(dir, acc = []) {
  for (const name of readdirSync(dir)) {
    const full = path.join(dir, name);
    if (statSync(full).isDirectory()) walkFiles(full, acc);
    else acc.push(full);
  }
  return acc;
}

function assertNoViteEnv() {
  const src = path.join(frontendRoot, "packages/admin-core/src");
  for (const file of walkFiles(src)) {
    if (file.includes(".test.")) continue;
    const text = readFileSync(file, "utf8");
    if (text.includes("import.meta.env") || /\bVITE_[A-Z0-9_]+\b/.test(text)) {
      throw new Error(`admin-core source still reads Vite env: ${path.relative(src, file)}`);
    }
  }
}

function packList(dir) {
  const parsed = JSON.parse(run("npm", ["pack", "--json", "--dry-run"], dir));
  const entry = Array.isArray(parsed) ? parsed[0] : parsed;
  return (entry.files ?? []).map((file) => file.path ?? file);
}

function assertPackContents(name, files) {
  for (const file of files) {
    const ok = allowed.has(file) || file.startsWith("dist/");
    if (!ok) throw new Error(`${name} pack contains non-whitelist file: ${file}`);
    if (file.endsWith(".map")) throw new Error(`${name} pack contains source map: ${file}`);
  }
  if (!files.includes("package.json") || !files.some((file) => file.startsWith("dist/"))) {
    throw new Error(`${name} pack missing package.json or dist`);
  }
}

assertNoViteEnv();
run("npm", ["run", "build", "-w", "@abysstract/admin-proto"], frontendRoot);
run("npm", ["run", "build", "-w", "@abysstract/admin-core"], frontendRoot);

const protoDir = path.join(frontendRoot, "packages/admin-proto");
const coreDir = path.join(frontendRoot, "packages/admin-core");
const protoFiles = packList(protoDir);
const coreFiles = packList(coreDir);
assertPackContents("@abysstract/admin-proto", protoFiles);
assertPackContents("@abysstract/admin-core", coreFiles);

const protoPkg = JSON.parse(readFileSync(path.join(protoDir, "package.json"), "utf8"));
const corePkg = JSON.parse(readFileSync(path.join(coreDir, "package.json"), "utf8"));
for (const pkg of [protoPkg, corePkg]) {
  if (pkg.private === true) throw new Error(`${pkg.name} must be publishable (remove private)`);
  if (pkg.publishConfig?.access !== "public") {
    throw new Error(`${pkg.name} must set publishConfig.access to "public"`);
  }
  if (pkg.license !== "Apache-2.0") throw new Error(`${pkg.name} must declare license Apache-2.0`);
}
if (corePkg.dependencies?.["@abysstract/admin-proto"] !== protoPkg.version) {
  throw new Error("admin-core must depend on the same admin-proto version");
}
if (corePkg.scripts?.prepare || corePkg.scripts?.postinstall) {
  throw new Error("admin-core install scripts must not run repository maintenance");
}

const protoPackName = run("npm", ["pack", "--silent"], protoDir).trim();
const corePackName = run("npm", ["pack", "--silent"], coreDir).trim();
const protoTgz = path.join(protoDir, protoPackName);
const coreTgz = path.join(coreDir, corePackName);

const isolated = mkdtempSync(path.join(tmpdir(), "admin-core-k10-"));
copyFileSync(protoTgz, path.join(isolated, protoPackName));
copyFileSync(coreTgz, path.join(isolated, corePackName));
writeFileSync(
  path.join(isolated, "package.json"),
  JSON.stringify(
    {
      name: "admin-core-isolated-consumer",
      private: true,
      type: "module",
      dependencies: {
        "@abysstract/admin-core": `file:${corePackName}`,
        "@abysstract/admin-proto": `file:${protoPackName}`,
        "@ant-design/icons": "5.4.0",
        "@ant-design/pro-components": "2.7.19",
        "@bufbuild/protobuf": "2.2.0",
        "@connectrpc/connect": "2.0.0",
        "@connectrpc/connect-web": "2.0.0",
        antd: "5.21.6",
        react: "18.3.1",
        "react-dom": "18.3.1",
        "react-router-dom": "6.27.0",
      },
      devDependencies: {
        "@types/react": "18.3.12",
        "@types/react-dom": "18.3.1",
        typescript: "5.6.3",
      },
    },
    null,
    2,
  ),
);
mkdirSync(path.join(isolated, "src"));
writeFileSync(
  path.join(isolated, "src/consumer.ts"),
  `import { bindAdminAppPageLifecycle, createAdminApp, createMemoryAuthStorage, formatErrorMessage } from "@abysstract/admin-core";
import { ResourceTable } from "@abysstract/admin-core/resource";
import type { ResourceSchema } from "@abysstract/admin-core/resource/schema";
import type { ResourceOperations } from "@abysstract/admin-core/resource/operations";
import { createRelationOptionLoader } from "@abysstract/admin-core/resource/relation";
import { useSystemClients, useSystemOperations } from "@abysstract/admin-core/system";
import { DirectImageUpload, useDirectUpload } from "@abysstract/admin-core/upload";
import { ConnectError } from "@connectrpc/connect";
export const schema = { name: "demo" } as ResourceSchema;
export type Ops = ResourceOperations<unknown, string>;
export const loader = createRelationOptionLoader;
export const table = ResourceTable;
export const systemHooks = { useSystemClients, useSystemOperations };
export const upload = { DirectImageUpload, useDirectUpload };
export const app = createAdminApp({
  host: {
    apiBaseUrl: "http://localhost:8080",
    mode: "development",
    authStorage: createMemoryAuthStorage(),
    title: "isolated",
  },
  modules: [],
});
export const stopPageLifecycle = bindAdminAppPageLifecycle({
  application: app,
  root: { unmount() {} },
  target: { addEventListener() {}, removeEventListener() {} },
});
export const formatted = formatErrorMessage(new ConnectError("x"));
stopPageLifecycle();
app.dispose();
`,
);
writeFileSync(
  path.join(isolated, "tsconfig.json"),
  JSON.stringify(
    {
      compilerOptions: {
        target: "ES2020",
        module: "ESNext",
        moduleResolution: "bundler",
        jsx: "react-jsx",
        strict: true,
        skipLibCheck: true,
        noEmit: true,
      },
      include: ["src"],
    },
    null,
    2,
  ),
);

run("npm", ["install", "--ignore-scripts"], isolated);
const coreReal = realpathSync(path.join(isolated, "node_modules/@abysstract/admin-core"));
const workspaceCore = realpathSync(coreDir);
if (coreReal === workspaceCore) {
  throw new Error(`isolated install used workspace directory: ${coreReal}`);
}
const ls = run("npm", ["ls", "--all", "--json"], isolated);
writeFileSync(path.join(isolated, "npm-ls.json"), ls);
run("npx", ["tsc", "--noEmit"], isolated);

const peerDir = mkdtempSync(path.join(tmpdir(), "admin-core-k10-peer-"));
writeFileSync(
  path.join(peerDir, "package.json"),
  JSON.stringify({
    name: "admin-core-peer-fail",
    private: true,
    dependencies: {
      "@abysstract/admin-core": `file:${coreTgz}`,
      "@abysstract/admin-proto": `file:${protoTgz}`,
      react: "16.14.0",
      "react-dom": "16.14.0",
    },
  }),
);
let peerFailed = false;
try {
  run(
    "npm",
    ["install", "--ignore-scripts", "--strict-peer-deps", "--legacy-peer-deps=false"],
    peerDir,
  );
} catch {
  peerFailed = true;
}
rmSync(peerDir, { recursive: true, force: true });
if (!peerFailed) throw new Error("incompatible react peer did not fail");

const report = {
  protoTgz,
  coreTgz,
  protoSha256: sha256(protoTgz),
  coreSha256: sha256(coreTgz),
  protoFiles,
  coreFiles,
  isolated,
  coreReal,
  workspaceCore,
};
process.stdout.write(`${JSON.stringify(report, null, 2)}\n`);
