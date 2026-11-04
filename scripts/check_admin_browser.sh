#!/bin/bash
# Use the job's verified disposable target after backend tests have completed.
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"
export GOWORK=off
browser_dir="$(mktemp -d)"
chmod 700 "$browser_dir"
export ADMIN_E2E_OUTPUT="$browser_dir/results"
export ADMIN_E2E_USERNAME=ADMIN
export ADMIN_E2E_BASE_URL=http://127.0.0.1:14173
export VITE_API_BASE=http://127.0.0.1:18086
export ADMIN_DB_DRIVER="$ADMIN_TEST_DRIVER"
export ADMIN_DB_DSN="$ADMIN_TEST_DSN"
export ADMIN_ADDR=127.0.0.1:18086
export ADMIN_ENV=dev
ADMIN_JWT_SECRET="$(openssl rand -hex 32)"
export ADMIN_JWT_SECRET
export ADMIN_CORS_ORIGINS=http://127.0.0.1:14173
export ADMIN_UPLOAD_DIR="$browser_dir/uploads"
(cd backend && go run ./cmd/admin-testdb -action verify)
(cd backend && go run ./cmd/migrate)
(cd frontend && npm ci --ignore-scripts && npm run build)
(cd frontend && npx playwright install --with-deps chromium)
(cd backend && go build -o "$browser_dir/server" ./cmd/server)
"$browser_dir/server" > "$browser_dir/server.log" 2>&1 &
backend_pid=$!
(cd frontend/apps/reference && exec node ../../node_modules/vite/bin/vite.js preview --host 127.0.0.1 --port 14173) > "$browser_dir/reference.log" 2>&1 &
reference_pid=$!
trap 'kill "$backend_pid" "$reference_pid"; wait; rm -rf "$browser_dir"' EXIT
python3 - <<'PY'
import time,urllib.request,urllib.error
for url in ['http://127.0.0.1:18086/healthz','http://127.0.0.1:14173']:
    for attempt in range(100):
        try:
            urllib.request.urlopen(url,timeout=1).close()
            break
        except urllib.error.URLError:
            if attempt==99: raise
            time.sleep(.1)
PY
(cd frontend && npm run test:e2e)
