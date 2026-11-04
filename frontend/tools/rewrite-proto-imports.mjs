import ts from "typescript";
import path from "node:path";

const systemPrefix = "abysstract/admin/v1/";
// File keys are module-relative output paths, derived from full Proto filenames.
export function rewriteProtoImports(
  files,
  systemFiles = new Set(),
  moduleFiles = new Map(),
  outputDir = "",
) {
  const output = new Map();
  for (const [file, source] of files) {
    const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
    const edits = [];
    function visit(node) {
      if (
        (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
        node.moduleSpecifier &&
        ts.isStringLiteral(node.moduleSpecifier)
      ) {
        const literal = node.moduleSpecifier;
        const spec = literal.text;
        if (spec.startsWith(".")) {
          const target =
            path.posix
              .normalize(path.posix.join(path.posix.dirname(file), spec))
              .replace(/\.(js|ts)$/, "") + ".ts";
          let replacement;
          if (
            target.startsWith(systemPrefix) &&
            !file.startsWith(systemPrefix) &&
            systemFiles.has(target)
          ) {
            replacement = "@abysstract/admin-proto/" + target.slice(0, -3);
          } else if (files.has(target)) {
            replacement = spec.replace(/\.(js|ts)$/, "") + ".js";
          } else if (moduleFiles.has(target)) {
            replacement = path
              .relative(path.dirname(path.join(outputDir, file)), moduleFiles.get(target))
              .split(path.sep)
              .join("/")
              .replace(/\.ts$/, ".js");
            if (!replacement.startsWith(".")) replacement = "./" + replacement;
          } else {
            throw new Error(`${file}: unknown relative Proto dependency ${spec} (${target})`);
          }
          edits.push([literal.getStart(ast) + 1, literal.getEnd() - 1, replacement]);
        }
      }
      ts.forEachChild(node, visit);
    }
    visit(ast);
    let text = source;
    for (const [start, end, replacement] of edits.reverse())
      text = text.slice(0, start) + replacement + text.slice(end);
    output.set(file, text);
  }
  return output;
}
