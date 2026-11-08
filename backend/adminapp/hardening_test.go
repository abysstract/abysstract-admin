package adminapp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
)

func TestApplyServerTimeouts(t *testing.T) {
	server := &http.Server{}
	ApplyServerTimeouts(server)
	if server.ReadHeaderTimeout <= 0 {
		t.Fatal("ReadHeaderTimeout not set")
	}
	if server.IdleTimeout <= 0 {
		t.Fatal("IdleTimeout not set")
	}

	custom := &http.Server{ReadHeaderTimeout: time.Second, IdleTimeout: time.Minute}
	ApplyServerTimeouts(custom)
	if custom.ReadHeaderTimeout != time.Second || custom.IdleTimeout != time.Minute {
		t.Fatalf("custom timeouts overwritten: %v %v", custom.ReadHeaderTimeout, custom.IdleTimeout)
	}
}

func TestMetricsHandler(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	cases := []struct {
		name   string
		cfg    adminconfig.Config
		header string
		want   int
	}{
		{"dev without token is open", adminconfig.Config{Env: adminconfig.EnvDev}, "", http.StatusOK},
		{"prod without token is disabled", adminconfig.Config{Env: adminconfig.EnvProd}, "", http.StatusNotFound},
		{"token missing header", adminconfig.Config{Env: adminconfig.EnvProd, MetricsToken: "s3cret"}, "", http.StatusUnauthorized},
		{"token wrong", adminconfig.Config{Env: adminconfig.EnvProd, MetricsToken: "s3cret"}, "Bearer nope", http.StatusUnauthorized},
		{"token right", adminconfig.Config{Env: adminconfig.EnvProd, MetricsToken: "s3cret"}, "Bearer s3cret", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			metricsHandler(tc.cfg, ok).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
