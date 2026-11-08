package adminapp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/internal/observ"
	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/storage"
)

type createdInstance struct {
	name     string
	services []module.ConnectService
	http     []module.HTTPHandler
	close    func(context.Context) error
}

func buildHandler(
	cfg adminconfig.Config,
	logger *slog.Logger,
	allowedMethods []string,
	routes []preparedRoute,
	instances []createdInstance,
	server *connect.Server,
	connectOpts connecthttp.Option,
	sec httpSecurity,
	objects storage.Storage,
	pinger observ.Pinger,
	registerer prometheus.Registerer,
	gatherer prometheus.Gatherer,
) (http.Handler, error) {
	mux := http.NewServeMux()
	for _, inst := range instances {
		for _, svc := range inst.services {
			svc.Register(server)
		}
		handlers := make(map[string]http.Handler, len(inst.http))
		for _, item := range inst.http {
			handlers[item.Name] = item.Handler
		}
		for _, route := range routes {
			if route.Module != inst.name {
				continue
			}
			if err := validateServeMuxPattern(route.Pattern); err != nil {
				return nil, err
			}
			handler := handlers[route.Route.Name]
			if handler == nil {
				return nil, fieldError(inst.name, "HTTP", fmt.Sprintf("handler %q is nil", route.Route.Name))
			}
			mux.Handle(route.Pattern, wrapHTTPRoute(route, handler, sec))
		}
	}
	connecthttp.Mount(mux, server, connectOpts)
	corsMethods := append([]string{}, allowedMethods...)
	if served, ok := objects.(storage.HTTPHandler); ok {
		// Drivers that serve objects from the application (the local
		// driver) take the reserved /static/ subtree, including signed PUT
		// uploads from the browser.
		if served.MountPath() != storageMountPath {
			return nil, fmt.Errorf("adminapp: storage mount path %q must be %q", served.MountPath(), storageMountPath)
		}
		mux.Handle(storageMountPath, served)
		if !slices.Contains(corsMethods, http.MethodPut) {
			corsMethods = append(corsMethods, http.MethodPut)
		}
	}
	mux.Handle("/healthz", observ.HealthzHandler())
	mux.Handle("/readyz", observ.ReadyzHandler(pinger))
	mux.Handle("/metrics", metricsHandler(cfg, promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})))
	mux.Handle("/version", observ.VersionHandler())

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORSOriginsOrDefault(),
		AllowedMethods:   append(corsMethods, http.MethodOptions),
		AllowedHeaders:   []string{"Content-Type", "Authorization", "Connect-Protocol-Version", "Connect-Timeout-Ms"},
		ExposedHeaders:   []string{"Connect-Protocol-Version", observ.HeaderRequestID},
		AllowCredentials: false,
	}).Handler(mux)

	metrics := observ.NewMetricsWithRegistry(registerer)
	return observ.RequestIDMiddleware(
		observ.LoggerMiddleware(logger, metrics.MetricsMiddleware(corsHandler)),
	), nil
}
