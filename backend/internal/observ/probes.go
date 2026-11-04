// Package observ — 探针与版本端点（需求 8.1–8.3、8.7、8.13）。
//
// 提供以下 HTTP handler，挂到主 mux 的非 RPC 路径：
//   - /healthz  : 进程存活探针，立即返回 200（不依赖 DB）
//   - /readyz   : 就绪探针，Ping DB 成功返回 200，失败返回 503
//   - /metrics  : Prometheus 指标端点（promhttp.Handler）
//   - /version  : 构建版本信息（ldflags 注入）
package observ

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/abysstract/abysstract-admin/backend/internal/buildinfo"
)

// readyTimeout 是 /readyz 探针 Ping DB 的最大等待时间（需求 8.2：<2s）。
const readyTimeout = 2 * time.Second

// Pinger 是 /readyz 依赖的最小接口，*sql.DB 与 *gorm.DB.DB() 都满足。
type Pinger interface {
	PingContext(ctx context.Context) error
}

// HealthzHandler 立即返回 200（进程存活，需求 8.1：<1s）。
// 不依赖任何下游，避免 DB 抖动导致进程被 K8s 杀死。
func HealthzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
}

// ReadyzHandler Ping DB，成功返回 200 ready，失败返回 503 not_ready（需求 8.2/8.3）。
// pinger 为 nil 时返回 503（启动早期 DB 未初始化）。
func ReadyzHandler(pinger Pinger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if pinger == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
		defer cancel()
		if err := pinger.PingContext(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}

// MetricsHandler 返回 Prometheus 默认 registry 的指标（需求 8.7）。
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// VersionHandler 返回构建版本信息（需求 8.13）。
func VersionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, buildinfo.Current())
	})
}

// AttachProbes 把探针端点挂到给定 mux。
// pinger 可为 nil（启动早期）； readyz 会自动返回 not_ready。
func AttachProbes(mux *http.ServeMux, pinger Pinger) {
	mux.Handle("/healthz", HealthzHandler())
	mux.Handle("/readyz", ReadyzHandler(pinger))
	mux.Handle("/metrics", MetricsHandler())
	mux.Handle("/version", VersionHandler())
}

// Compile-time check: 确保 *sql.DB 满足 Pinger。
var _ Pinger = (*sql.DB)(nil)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
