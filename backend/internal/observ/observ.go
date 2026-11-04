// Package observ 提供运维可观测中间件与探针 handler。
//
// 包含三类能力（对应需求 8.4–8.7）：
//   - RequestID：沿用入站 X-Request-Id 或生成 ≥16 字符的随机 ID，注入 context 与响应头。
//   - Logger：log/slog 结构化 JSON 日志，每条请求日志含 request_id/ts/method/status/duration。
//   - Metrics：Prometheus 请求计数与延迟直方图。
//
// 中间件以 net/http 风格实现，包裹整个 mux（位于 Connect 拦截器之外），
// 因此同时覆盖 RPC 调用与 /healthz /readyz /metrics /version 等非 RPC 端点。
package observ

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HeaderRequestID 是请求 ID 的 HTTP 头名称（沿用社区惯例）。
const HeaderRequestID = "X-Request-Id"

// minRequestIDLen 是请求 ID 的最小长度（字符数）。
// 需求 8.5：入站无请求 ID 时生成 ≥16 字符的唯一 ID。
const minRequestIDLen = 16

type ctxKey int

const requestIDKey ctxKey = iota

// RequestIDFromContext 从 context 取出请求 ID（由 RequestIDMiddleware 注入）。
// 不存在时返回空串。
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// generateRequestID 生成 32 字符（16 字节 hex）的随机请求 ID。
// 用 crypto/rand 保证全局唯一性；失败时回退到时间戳（极端降级）。
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 极端路径：crypto/rand 失败时退化为时间戳 hex，保证长度仍 ≥16。
		return time.Now().UTC().Format("20060102150405000000000000000000")[:minRequestIDLen*2]
	}
	return hex.EncodeToString(b)
}

// resolveRequestID 决定单个请求使用的 request ID：
// 入站已携带 X-Request-Id 则沿用（需求 8.6），否则生成新的（需求 8.5）。
func resolveRequestID(header string) string {
	if len(header) >= minRequestIDLen {
		return header
	}
	return generateRequestID()
}

// RequestIDMiddleware 注入请求 ID 到 context 并回写响应头。
// 入站已带 X-Request-Id（≥16 字符）则沿用，否则生成新的随机 ID。
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := resolveRequestID(r.Header.Get(HeaderRequestID))
		w.Header().Set(HeaderRequestID, rid)
		ctx := context.WithValue(r.Context(), requestIDKey, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// statusRecorder 捕获响应状态码（ResponseWriter 默认不暴露）。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Metrics 是 RPC / HTTP 请求的可观测指标集（Prometheus）。
type Metrics struct {
	requestsTotal *prometheus.CounterVec
	duration      *prometheus.HistogramVec
}

// NewMetrics 注册请求计数与延迟直方图到默认 registry。
// 标签：method（HTTP 方法或 RPC 过程名）、code（HTTP 状态码）。
func NewMetrics() *Metrics {
	return NewMetricsWithRegistry(prometheus.DefaultRegisterer)
}

// NewMetricsWithRegistry 用给定 registry 注册指标。
// 测试中传入 prometheus.NewRegistry() 避免全局 registry 冲突。
func NewMetricsWithRegistry(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	factory := promauto.With(reg)
	return &Metrics{
		requestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"method", "code"}),
		duration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "code"}),
	}
}

// MetricsMiddleware 记录请求计数与延迟。
// 必须在 RequestIDMiddleware 之后注册（这样日志和指标共享同一 request_id 上下文）。
func (m *Metrics) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start).Seconds()

		code := http.StatusText(rec.status)
		if code == "" {
			code = strconv.Itoa(rec.status)
		}
		m.requestsTotal.WithLabelValues(r.Method, code).Inc()
		m.duration.WithLabelValues(r.Method, code).Observe(elapsed)
	})
}

// LoggerMiddleware 用 slog 输出结构化 JSON 请求日志（需求 8.4）。
// 每条日志包含 request_id/ts/method/path/status/duration。
func LoggerMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start)

		logger.Info("http request",
			slog.String("request_id", RequestIDFromContext(r.Context())),
			slog.String("ts", start.UTC().Format(time.RFC3339Nano)),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("duration", elapsed),
		)
	})
}

// Chain 把多个中间件按给定顺序应用到 handler。
// 第一个中间件位于最外层，最后执行；与 mux.Use 语义一致。
func Chain(handler http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler
}
