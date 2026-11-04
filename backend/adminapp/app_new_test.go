package adminapp

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestNewDoesNotOpenDBOrListen(t *testing.T) {
	var created atomic.Int32
	restoreOpen := OverrideOpenDatabase(func(context.Context, string, string) (*gorm.DB, error) {
		t.Fatal("openDatabase called")
		return nil, nil
	})
	defer restoreOpen()
	restoreListen := OverrideListen(func(string, string) (net.Listener, error) {
		t.Fatal("listen called")
		return nil, nil
	})
	defer restoreListen()
	_, err := New(Options{Config: testConfig(t), Logger: silentLogger(), Modules: []module.Definition{{
		Name:   "system",
		Create: countingCreate(&created),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if created.Load() != 0 {
		t.Fatalf("Create called %d times", created.Load())
	}
}

func TestNewRejectsInvalidConfigWithoutCreate(t *testing.T) {
	var created atomic.Int32
	cfg := testConfig(t)
	cfg.ShutdownTimeout = 0
	_, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{{
		Name:   "system",
		Create: countingCreate(&created),
	}}})
	if err == nil {
		t.Fatal("expected config error")
	}
	if created.Load() != 0 {
		t.Fatalf("Create called %d times", created.Load())
	}
}

func TestNewClonesConfig(t *testing.T) {
	cfg := testConfig(t)
	app, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{systemModule()}})
	if err != nil {
		t.Fatal(err)
	}
	cfg.CORSOrigins[0] = "https://evil.example"
	if app.cfg.CORSOrigins[0] != "http://localhost:5173" {
		t.Fatalf("config mutated: %v", app.cfg.CORSOrigins)
	}
}

func TestConfigLogValueHidesSecrets(t *testing.T) {
	cfg := adminconfig.Config{DBDriver: "postgres", DBDSN: "password=super-secret", JWTSecret: "super-secret"}
	text := cfg.LogValue().String()
	if strings.Contains(text, "super-secret") {
		t.Fatalf("leaked: %s", text)
	}
}

func TestShutdownBeforeRun(t *testing.T) {
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), Modules: []module.Definition{systemModule()}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Shutdown(t.Context()); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("err = %v", err)
	}
}
