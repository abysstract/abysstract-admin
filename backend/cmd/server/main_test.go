package main

import (
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/internal/app"
)

func TestNewApplicationDoesNotListen(t *testing.T) {
	cfg := adminconfig.Config{
		Env:      adminconfig.EnvDev,
		Addr:     "127.0.0.1:0",
		DBDriver: "postgres", DBDSN: "host=localhost user=test password=test dbname=test sslmode=disable",
		JWTSecret:           "test-secret-not-default",
		JWTExpiresIn:        time.Hour,
		CORSOrigins:         []string{"http://localhost:5173"},
		ShutdownTimeout:     5 * time.Second,
		DBMaxOpenConns:      5,
		DBMaxIdleConns:      2,
		LoginMaxFailures:    5,
		LoginLockoutSeconds: 900,
		UploadMaxBytes:      1 << 20,
		AvatarMaxBytes:      1 << 20,
		UploadDir:           t.TempDir(),
	}
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if application.ListenerAddr() != "" {
		t.Fatalf("New listened on %s", application.ListenerAddr())
	}
}
