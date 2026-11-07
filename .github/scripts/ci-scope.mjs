// Select database jobs from the changed paths or an explicit manual request.
export function ciScope(paths, { event, database } = {}) {
  if (event === "workflow_dispatch") {
    return {
      checks: database === "all" || database === "postgres",
      postgres: database === "all" || database === "postgres",
      sqlite: database === "all" || database === "sqlite",
      mysql: database === "all" || database === "mysql",
      mariadb: database === "all" || database === "mariadb",
    };
  }

  const files = paths.filter((file) => !file.endsWith(".md"));
  const workflow = files.some((file) =>
    file === ".github/workflows/ci.yml" || file.startsWith(".github/scripts/"),
  );
  const backend = workflow || files.some((file) =>
    /^(backend\/|proto\/|buf[^/]*\.yaml$|admin\.modules\.json$|go\.(?:mod|sum|work(?:\.sum)?)$)/.test(file) ||
    /^(docker-compose[^/]*\.ya?ml|compose[^/]*\.ya?ml|scripts\/(generate\.sh|init_pg\.sql|reset_db\.sh|check_admin_(browser|generated)\.(sh|py)))$/.test(file),
  );
  const shared = workflow || files.some((file) => {
    if (/^backend\/(coremigrations|migrations)\/(postgres|mysql|mariadb|sqlite)\//.test(file)) return false;
    if (/^backend\/database\/(postgres|sqlite)(?:[._/])/.test(file)) return false;
    return /^(proto\/|buf[^/]*\.yaml$|go\.(?:mod|sum|work(?:\.sum)?)$|admin\.modules\.json$|scripts\/generate\.sh$)/.test(file) ||
      /^backend\/(go\.(mod|sum)$|database\/|coremigrations\/|migrations\/|moduleseed\/|resource(?:gen)?\/|testdb\/|system\/|cmd\/(modelgen|adminresource-gen|migrate)\/|internal\/(repo|service|dal|codegen|resource|testdb|testutil|modules)\/|internal\/cli\/module\/)/.test(file);
  });
  return {
    checks: true,
    postgres: backend,
    sqlite: shared || files.some((file) =>
      /^backend\/(coremigrations|migrations)\/sqlite\//.test(file) ||
      /^backend\/database\/sqlite(?:[._/])/.test(file),
    ),
    mysql: shared || files.some((file) => /^backend\/(coremigrations|migrations)\/mysql\//.test(file)),
    mariadb: shared || files.some((file) => /^backend\/(coremigrations|migrations)\/mariadb\//.test(file)),
  };
}
