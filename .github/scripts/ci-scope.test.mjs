import assert from "node:assert/strict";
import test from "node:test";
import { ciScope } from "./ci-scope.mjs";

for (const [name, paths, databases] of [
  ["documentation", ["docs/module-guide.md", "backend/README.md"], []],
  ["frontend", ["frontend/apps/reference/src/App.tsx"], []],
  ["backend HTTP", ["backend/internal/app/server.go"], ["postgres"]],
  ["MySQL migration", ["backend/migrations/mysql/20261103190001_note.sql"], ["postgres", "mysql"]],
  ["MariaDB migration", ["backend/coremigrations/mariadb/20261103190001_note.sql"], ["postgres", "mariadb"]],
  ["PostgreSQL migration", ["backend/migrations/postgres/20261103190001_note.sql"], ["postgres"]],
  ["PostgreSQL driver", ["backend/database/postgres.go"], ["postgres"]],
  ["SQLite migration", ["backend/migrations/sqlite/20261103190001_note.sql"], ["postgres", "sqlite"]],
  ["SQLite driver", ["backend/database/sqlite.go"], ["postgres", "sqlite"]],
  ["shared MySQL driver", ["backend/database/mysql.go"], ["postgres", "sqlite", "mysql", "mariadb"]],
  ["shared ownership migration", ["backend/moduleseed/upgrade.go"], ["postgres", "sqlite", "mysql", "mariadb"]],
  ["business resource", ["backend/internal/modules/notes/resources/note.json"], ["postgres", "sqlite", "mysql", "mariadb"]],
  ["custom business query", ["backend/internal/modules/notes/note_query.go"], ["postgres", "sqlite", "mysql", "mariadb"]],
  ["model generator", ["backend/cmd/modelgen/main.go"], ["postgres", "sqlite", "mysql", "mariadb"]],
  ["generation script", ["scripts/generate.sh"], ["postgres", "sqlite", "mysql", "mariadb"]],
  ["dependencies", ["backend/go.sum"], ["postgres", "sqlite", "mysql", "mariadb"]],
  ["workflow", [".github/workflows/ci.yml"], ["postgres", "sqlite", "mysql", "mariadb"]],
]) {
  test(name, () => {
    const { checks, ...result } = ciScope(paths, { event: "pull_request" });
    assert.equal(checks, true);
    assert.deepEqual(Object.keys(result).filter((key) => result[key]), databases);
    assert.deepEqual(ciScope(paths, { event: "push" }), { checks, ...result });
  });
}

for (const database of ["postgres", "sqlite", "mysql", "mariadb", "all"]) {
  test(`manual ${database}`, () => {
    const { checks, ...result } = ciScope([], { event: "workflow_dispatch", database });
    assert.equal(checks, database === "all" || database === "postgres");
    assert.deepEqual(
      Object.keys(result).filter((key) => result[key]),
      database === "all" ? ["postgres", "sqlite", "mysql", "mariadb"] : [database],
    );
  });
}
