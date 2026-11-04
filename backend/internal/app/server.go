package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	"github.com/abysstract/abysstract-admin/backend/adminconfig"
)

// NewServer is the application's HTTP composition point. Add routes to mux and
// wrap its handler with application middleware before constructing the server.
func NewServer(cfg adminconfig.Config, core http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", core)
	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	server := &http.Server{Addr: cfg.Addr, Handler: mux, Protocols: protocols}
	adminapp.ApplyServerTimeouts(server)
	return server
}

func serve(ctx context.Context, server *http.Server, listener net.Listener, timeout time.Duration) error {
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	var serveErr error
	select {
	case serveErr = <-result:
	case <-ctx.Done():
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		closeErr := server.Close()
		if errors.Is(shutdownErr, context.DeadlineExceeded) {
			shutdownErr = errors.Join(adminapp.ErrShutdownTimeout, shutdownErr)
		}
		return errors.Join(serveErr, shutdownErr, closeErr)
	}
	return serveErr
}
