package adminapp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestStaticFileHandler(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	handler := http.StripPrefix("/static/", staticFileHandler(http.Dir(dir)))

	for _, path := range []string{"/static/", "/static/sub", "/static/sub/", "/static/missing.txt"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/a.txt", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "hello" {
		t.Fatalf("file status = %d body = %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("nosniff = %q", got)
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing Content-Security-Policy")
	}

	if err := os.WriteFile(filepath.Join(dir, "doc.PDF"), []byte("%PDF-1.4"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/doc.PDF", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("pdf status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Fatalf("pdf CSP = %q, want none", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("pdf nosniff = %q", got)
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
