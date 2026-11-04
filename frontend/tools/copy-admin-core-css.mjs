import { cpSync, mkdirSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const srcRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../packages/admin-core/src",
);
const distRoot = path.resolve(srcRoot, "../dist");

function walk(dir) {
  for (const name of readdirSync(dir)) {
    const full = path.join(dir, name);
    if (statSync(full).isDirectory()) walk(full);
    else if (name.endsWith(".css")) {
      const dest = path.join(distRoot, path.relative(srcRoot, full));
      mkdirSync(path.dirname(dest), { recursive: true });
      cpSync(full, dest);
    }
  }
}

walk(srcRoot);
