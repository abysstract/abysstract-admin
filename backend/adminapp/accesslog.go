package adminapp

import (
	"context"
	"log/slog"
	"net/http"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/internal/observ"

	// The built-in sinks are always available; other sinks are enabled by
	// importing their packages, which call accesslog.Register.
	_ "github.com/abysstract/abysstract-admin/backend/accesslog/dbsink"
	_ "github.com/abysstract/abysstract-admin/backend/accesslog/stdoutsink"
)

// AccessLogConfig maps the application configuration to the configuration
// handed to access log sink factories.
func AccessLogConfig(cfg adminconfig.Config, db *gorm.DB, logger *slog.Logger) accesslog.Config {
	return accesslog.Config{
		DB:        db,
		Retention: cfg.AccessLogRetention(),
		Logger:    logger,
	}
}

// OpenAccessLog opens the sinks named by cfg.AccessLogSinks, appends extra
// and starts a dispatcher with the configured queue, batch size and flush
// interval. It returns nil, which disables access logging, when there is no
// sink. The caller owns the Logger and closes it after the HTTP servers that
// use it have stopped; closing it also closes extra.
//
// Applications that run their own API server next to the admin application
// open one Logger here, pass it as Options.AccessLog and use it with
// accesslog.Interceptor in their server.
func OpenAccessLog(ctx context.Context, cfg adminconfig.Config, db *gorm.DB, logger *slog.Logger, extra ...accesslog.Sink) (*accesslog.Logger, error) {
	sinks, err := accesslog.Open(ctx, cfg.AccessLogSinks, AccessLogConfig(cfg, db, logger))
	if err != nil {
		return nil, err
	}
	for _, sink := range extra {
		if sink != nil {
			sinks = append(sinks, sink)
		}
	}
	if len(sinks) == 0 {
		return nil, nil
	}
	return accesslog.New(accesslog.Options{
		Sinks:         sinks,
		QueueSize:     cfg.AccessLogQueueSize,
		BatchSize:     cfg.AccessLogBatchSize,
		FlushInterval: cfg.AccessLogFlushInterval,
		Logger:        logger,
	}), nil
}

// AccessLogCapture returns the capture options of the admin application:
// ADMIN_TRUST_PROXY for the client IP, the X-Request-Id assigned by the
// application, and no entries for health probes, metrics or CORS preflight.
func AccessLogCapture(cfg adminconfig.Config) accesslog.CaptureOptions {
	return accesslog.CaptureOptions{
		TrustProxy: cfg.TrustProxy,
		RequestID:  observ.RequestIDFromContext,
		Skip:       skipAccessLog,
	}
}

func skipAccessLog(r *http.Request) bool {
	if r.Method == http.MethodOptions {
		return true
	}
	switch r.URL.Path {
	case "/healthz", "/readyz", "/metrics", "/version":
		return true
	}
	return false
}
