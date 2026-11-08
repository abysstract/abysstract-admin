# Repository Guidelines

## Project Structure & Module Organization

This repository contains a Go ConnectRPC backend, public frontend packages, an application with preinstalled system and product modules.

- `proto/` owns public API sources; business Proto lives in `backend/internal/modules/<module>/proto`. Root `buf.yaml` declares their workspace.
- `backend/internal/app/` owns application module composition, HTTP routes, middleware, and server lifecycle. Business development continues in the root application.
- The Go module `github.com/abysstract/abysstract-admin` has its `go.mod` at the repository root; Go code lives in `backend/`. Public packages are `backend/{resource,module,permission,resourcegen,coremigrations,database,system,adminconfig,identity,adminapp,moduleseed,storage}`; `storage/localstorage` and `storage/s3storage` are its drivers (importing `s3storage` pulls in the AWS SDK). Commands are under `backend/cmd/`. Go commands work from the root (`./backend/...`) or inside `backend/`. One `vX.Y.Z` tag versions both the Go module and the npm packages.
- `frontend/packages/admin-proto` and `frontend/packages/admin-core` are the public frontend packages. `frontend/apps/reference` is the bundled admin app.
- Product code lives in `backend/internal/modules/product` and `frontend/apps/reference/src/modules/product`.
- Resource-style CRUD: Proto defines the wire contract, Go `resource/schema.Resource` defines behavior, and generated repositories call application Gen queries. System declarations live in `backend/internal/resource/coredefs`.
- `backend/coremigrations/` provides Core migration sources. Applications maintain a local `migrations/` directory and execute migrations before starting services.

## Build, Test, and Development Commands

- `docker compose up -d --wait`: start local PostgreSQL on port 5433.
- `cd backend && go run ./cmd/migrate`: apply system and product migrations before starting the server.
- `cd backend && go run ./cmd/server`: run the bundled backend on `:8080`.
- `cd frontend && npm ci && npm run dev`: build public packages and run the reference app.
- `cd frontend && npm run build -w @abysstract/admin-proto -w @abysstract/admin-core && npm pack --workspace @abysstract/admin-core && npm pack --workspace @abysstract/admin-proto`: local tarballs with the same contents as the npm release (`docs/development.md#发布前端包`).
- `cd proto && buf lint && buf generate` is not the unified entry; use `node frontend/tools/generate-proto.mjs`.
- Module lifecycle: `cd backend && go run ./cmd/admin module new|resource|generate|update|uninstall|status|sync|check`. Public generation uses only modules selected in `admin.modules.json`; `module generate NAME` reads the explicit target declaration and supports generation before installation. After table migration, `module resource MODULE TABLE` creates the initial JSON; `generate` reads those files and runs Model/query, protocol and Resource generators. Lifecycle commands execute application migrations; status reports database history. See `docs/module-guide.md`. Tables use `<module>_<table>`.
- `sh scripts/generate.sh`: after database migration, regenerate system and business Model/query, Go/TS protocol and Resource outputs. Run build, typecheck and tests separately. For partial generation commands, see `docs/development.md`.
- In `backend/`, use `go build ./...`, `go vet ./...`, and `go test ./...` for database-free verification.
- `cd frontend && npm run typecheck && npm test && npm run build`: frontend verification.
- `python3 scripts/check_admin_public_packages.py --repo-root . --stage k2`
- `python3 scripts/check_admin_generated.py --scope all`
- `python3 scripts/admin_artifacts.py pack --dest /tmp/admin-artifacts`: pack and verify the tarballs before npm publish.

## Coding Style & Naming Conventions

Use Go 1.26+ and Connect-Go v2 (`connectrpc.com/connect/v2`). Handlers accept and return Proto messages. Register services on `connect.Server` and mount them with `connecthttp.Mount`; the application owns HTTP routing and lifecycle.

Use `gofmt` for Go and keep package names short and lowercase. Keep bundled system services in `backend/internal/service` and persistence in `backend/internal/repo`. React pages/components use PascalCase file names. Use generated Connect clients and types instead of handwritten DTOs.

## Testing Guidelines

Place Go tests beside implementation files using `*_test.go`. Frontend package tests run with `npm test` in `frontend/`. Application tests run with Vitest and Playwright under `frontend/apps/reference/`. For API changes run buf lint and regenerate both sides.

## Architecture Constraints

Use DB-first changes. The backend owns migrations for shared business tables; front and admin services may share generated business models and keep separate APIs.

JSON modules follow `docs/new-module-tutorial.md`: migrate → `module resource` → edit JSON → `module generate` → add Hook/Overrides → test. Handwritten Resource modules follow `docs/ai-cookbook.md`: migrate → generate Model/query and protocol types → generate Resource → add Hook/Overrides → test. Set `resourcegen.Config.GenQueryImport` to the application query package. Preserve presence, AccessScope, the caller transaction and Hook/AfterCommit semantics. Regenerate with the documented commands; never hand-edit `*.gen.go` / `*.gen.ts` / `*.pb.go`.

## Commit & Pull Request Guidelines

Follow Conventional Commits: `<type>(<scope>): <subject>`. PRs follow `.github/PULL_REQUEST_TEMPLATE.md`. Security issues follow `SECURITY.md`.

## Security & Configuration Tips

Start from `.env.example` and avoid committing secrets. Production mode requires non-default `ADMIN_JWT_SECRET`, `ADMIN_DB_DSN`, and explicit `ADMIN_CORS_ORIGINS`.

## Documentation

Keep current usage and development guidance in README and `docs/`. Keep agent instructions here concise and link to the relevant guide. Write tutorials in task order, with runnable steps and observable results.

- Business onboarding: `docs/new-module-tutorial.md`
- Handwritten product extension: `docs/admin-core-getting-started.md`
- Public API usage: `docs/admin-core-usage.md`
- Model/query and Resource generation: `docs/development.md`
- Standard CRUD and dedicated system workflows: `docs/resource-migration.md`
- File storage, presigned uploads and drivers: `docs/storage.md`
