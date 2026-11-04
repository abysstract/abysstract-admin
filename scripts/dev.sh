#!/usr/bin/env bash
# 聚合启动脚本：按「依赖 → 迁移 → 后端 → 前端」顺序启动，仅在前一组件成功后才启动下一步。
# 后端在后台启动并做健康检查（/healthz 返回 200 才视为成功，失败则脚本退出）；
# 前端最后在前台启动；Ctrl+C 会同时终止两个进程。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "==> 0. 准备环境变量"
if [ ! -f "$ROOT/.env" ]; then
  echo "   .env 不存在，请先执行: cp .env.example .env"
  exit 1
fi

# 与后端 conf.Load（godotenv）一致的 .env 加载语义：
# 已存在的环境变量优先，仅从 .env 补充当前未设置的变量；
# 处理 godotenv 支持的完整语法（export 前缀、键名前后空白、引号、转义、行内注释）。
# 不在 shell 里手写解析器：godotenv 的语义复杂度远超简易正则匹配，
# 自行实现会与后端产生行为漂移。
# 直接调用后端的 dumpenv 子命令：用同一个 godotenv 库解析，语义天然一致。
DUMPENV="$ROOT/backend/bin/dumpenv"
if [ ! -x "$DUMPENV" ]; then
  echo "   构建后端 dumpenv 工具..."
  (cd "$ROOT/backend" && go build -o ./bin/dumpenv ./cmd/dumpenv)
fi
# eval 把 dumpenv 输出的 KEY=VALUE 行作为赋值执行（已设置的不覆盖）。
# shellcheck disable=SC2046
eval "$("$DUMPENV" "$ROOT/.env")"

# 解析 ADMIN_ADDR（与后端 conf.Load 默认值保持一致：默认 ":8080"）。
# 支持 ":port" / "host:port" / "*:port" 三种形式，统一规范化为 health 探测 URL。
ADDR="${ADMIN_ADDR:-:8080}"
# 去掉前导冒号或通配 host，得到 host / port 分量。
case "$ADDR" in
  :*)
    HEALTH_HOST="localhost"
    HEALTH_PORT="${ADDR#:}"
    ;;
  *:*)
    HEALTH_HOST="${ADDR%%:*}"
    HEALTH_PORT="${ADDR##*:}"
    # 若 host 部分为 "*"（监听所有接口），用 localhost 探测本地实例。
    if [ "$HEALTH_HOST" = "*" ] || [ "$HEALTH_HOST" = "0.0.0.0" ]; then
      HEALTH_HOST="localhost"
    fi
    ;;
  *)
    echo "::error::无法解析 ADMIN_ADDR=$ADDR，应为 [host]:port 形式"
    exit 1
    ;;
esac
HEALTH_URL="http://${HEALTH_HOST}:${HEALTH_PORT}/healthz"

echo "==> 1. 启动本地依赖（独立 admin PostgreSQL）"
docker compose up -d --wait
echo "   PostgreSQL 就绪"

echo "==> 2. 数据库迁移"
(cd "$ROOT/backend" && go run ./cmd/migrate)
echo "   迁移完成"

# 清理函数：退出时确保后台后端进程被回收。
BACKEND_PID=""
cleanup() {
  if [ -n "$BACKEND_PID" ]; then
    echo ""
    echo "==> 停止后端 (pid=$BACKEND_PID)"
    kill "$BACKEND_PID" 2>/dev/null || true
    wait "$BACKEND_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

echo "==> 3. 启动后端（后台，等待健康检查通过）"
(cd "$ROOT/backend" && go run ./cmd/server) &
BACKEND_PID=$!

# 轮询 /healthz：最多等 60s（与 conf.Validate + db.Open + 探针就绪预期匹配）。
# 前端 dev server 无稳定 TCP 健康端口，因此前端必须最后启动且用前台阻塞。
HEALTH_OK=0
for i in $(seq 1 60); do
  # kill -0 检查进程是否还在：若已退出说明后端启动失败。
  if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
    echo "::error::后端进程在启动期间退出"
    wait "$BACKEND_PID"
    exit 1
  fi
  if curl -fsS -o /dev/null "$HEALTH_URL" 2>/dev/null; then
    HEALTH_OK=1
    break
  fi
  sleep 1
done

if [ "$HEALTH_OK" -ne 1 ]; then
  echo "::error::后端 60s 内未通过健康检查 ($HEALTH_URL)"
  kill "$BACKEND_PID" 2>/dev/null || true
  exit 1
fi
echo "   后端就绪 (pid=$BACKEND_PID, $HEALTH_URL)"

echo "==> 4. 启动前端 dev server（前台，Ctrl+C 退出）"
# 前端 dev server 立即失败（如端口占用）会在前台抛错并终止脚本。
cd "$ROOT/frontend" && npm run dev
