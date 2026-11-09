package adminapp

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
)

// Server timeouts guard against slow-client resource exhaustion. ReadTimeout
// and WriteTimeout stay unset so long uploads and streaming RPCs keep working.
const (
	serverReadHeaderTimeout = 10 * time.Second
	serverIdleTimeout       = 120 * time.Second
)

// ApplyServerTimeouts fills in the framework's connection timeouts on server.
// Timeouts the application already set are kept. Applications that build
// their own http.Server should call it too.
func ApplyServerTimeouts(server *http.Server) {
	if server.ReadHeaderTimeout == 0 {
		server.ReadHeaderTimeout = serverReadHeaderTimeout
	}
	if server.IdleTimeout == 0 {
		server.IdleTimeout = serverIdleTimeout
	}
}

// metricsHandler protects /metrics. With ADMIN_METRICS_TOKEN set, callers must
// send "Authorization: Bearer <token>". Without a token the endpoint is open in
// development and disabled in production.
func metricsHandler(cfg adminconfig.Config, next http.Handler) http.Handler {
	token := cfg.MetricsToken
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			if cfg.IsProd() {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
