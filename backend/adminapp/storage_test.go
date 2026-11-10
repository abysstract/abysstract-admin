package adminapp

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/storage"
	"github.com/abysstract/abysstract-admin/backend/storage/localstorage"
)

type nopPinger struct{}

func (nopPinger) PingContext(context.Context) error { return nil }

func testHandler(t *testing.T, cfg adminconfig.Config, objects storage.Storage) http.Handler {
	t.Helper()
	reg := prometheus.NewRegistry()
	h, err := buildHandler(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), []string{http.MethodGet, http.MethodPost},
		nil, nil, connect.NewServer(), connecthttp.WithReadMaxBytes(1<<20), httpSecurity{}, objects, nil, nopPinger{}, reg, reg)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestStorageConfigDerivesSigningSecret(t *testing.T) {
	cfg := adminconfig.Config{JWTSecret: "jwt-secret", UploadDir: "/data/uploads", StorageS3Bucket: "b", StorageS3PathStyle: true}
	a := StorageConfig(cfg)
	if a.Driver != "local" || a.LocalDir != "/data/uploads" || len(a.SigningSecret) != 32 || a.S3.Bucket != "b" || !a.S3.UsePathStyle {
		t.Fatalf("StorageConfig = %+v", a)
	}
	if bytes.Contains(a.SigningSecret, []byte("jwt-secret")) {
		t.Fatal("signing secret is the raw JWT secret")
	}
	cfg.JWTSecret = "rotated"
	if bytes.Equal(StorageConfig(cfg).SigningSecret, a.SigningSecret) {
		t.Fatal("derived secret ignores the JWT secret")
	}
	cfg.StorageSigningSecret = "explicit-secret-value"
	if string(StorageConfig(cfg).SigningSecret) != "explicit-secret-value" {
		t.Fatal("explicit signing secret not used")
	}
	if StorageConfig(adminconfig.Config{}).LocalDir != "./uploads" {
		t.Fatal("default upload dir not applied")
	}
}

func TestBuildHandlerMountsLocalStorage(t *testing.T) {
	st, err := localstorage.New(localstorage.Options{Dir: t.TempDir(), Secret: []byte("0123456789abcdef")})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := adminconfig.Config{Env: adminconfig.EnvDev, CORSOrigins: []string{"http://localhost:5173"}}
	h := testHandler(t, cfg, st)

	up, err := st.PresignPut(context.Background(), "files/a.png", "image/png", 3, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	pre := httptest.NewRequest(http.MethodOptions, up.URL, nil)
	pre.Header.Set("Origin", "http://localhost:5173")
	pre.Header.Set("Access-Control-Request-Method", http.MethodPut)
	pre.Header.Set("Access-Control-Request-Headers", "content-type")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, pre)
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), http.MethodPut) {
		t.Fatalf("preflight PUT not allowed: %d %v", rec.Code, rec.Header())
	}

	req := httptest.NewRequest(http.MethodPut, up.URL, strings.NewReader("png"))
	req.Header.Set("Content-Type", "image/png")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, st.URL("files/a.png"), nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "png" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("GET = %d %q %v", rec.Code, rec.Body, rec.Header())
	}
}

type remoteStorage struct{ storage.Storage }

func TestBuildHandlerSkipsRemoteStorage(t *testing.T) {
	h := testHandler(t, adminconfig.Config{Env: adminconfig.EnvDev}, remoteStorage{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/a.png", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /static/ with remote storage = %d", rec.Code)
	}
}

type wrongMount struct{ *localstorage.Storage }

func (wrongMount) MountPath() string { return "/files/" }

func TestBuildHandlerRejectsOtherMountPath(t *testing.T) {
	st, err := localstorage.New(localstorage.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg := prometheus.NewRegistry()
	_, err = buildHandler(adminconfig.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), []string{http.MethodGet},
		nil, nil, connect.NewServer(), connecthttp.WithReadMaxBytes(1<<20), httpSecurity{}, wrongMount{st}, nil, nopPinger{}, reg, reg)
	if err == nil || !strings.Contains(err.Error(), "mount path") {
		t.Fatalf("buildHandler = %v", err)
	}
}

func TestNewRejectsUnregisteredStorageDriver(t *testing.T) {
	cfg := testConfig(t)
	cfg.StorageDriver = "not-registered"
	if _, err := New(Options{Config: cfg, Modules: []module.Definition{systemModule()}}); err == nil || !strings.Contains(err.Error(), "not-registered") {
		t.Fatalf("New = %v", err)
	}
	if _, err := New(Options{Config: cfg, Modules: []module.Definition{systemModule()}, Storage: remoteStorage{}}); err != nil {
		t.Fatalf("injected storage with unknown driver name: %v", err)
	}
}
