package adminapp

import (
	"crypto/subtle"
	"net/http"
	"path"
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

// staticFileHandler serves uploaded files without directory listings. It
// stops browsers from sniffing content types and from running uploaded HTML or
// SVG as pages of the admin origin. It does not restrict who can download a
// file: anyone with the URL can fetch it.
func staticFileHandler(root http.FileSystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		f, err := root.Open(r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		// The CSP sandbox stops Chrome's built-in viewer from rendering PDFs,
		// so PDFs rely on nosniff alone.
		if !strings.EqualFold(path.Ext(info.Name()), ".pdf") {
			h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	})
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
