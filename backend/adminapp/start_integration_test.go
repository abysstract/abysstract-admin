//go:build integration

package adminapp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestStartReturnsHandlerAndKeepsBorrowedResources(t *testing.T) {
	skipSystemTables(t)
	restore := OverrideListen(func(string, string) (net.Listener, error) {
		t.Fatal("Start opened a listener")
		return nil, nil
	})
	defer restore()
	db := sqliteDB(t)
	var runtimeContext context.Context
	var closes atomic.Int32
	app, err := New(Options{Config: testConfig(t), DB: db, Logger: silentLogger(), Modules: []module.Definition{{
		Name: "system",
		HTTP: []module.HTTPRoute{{Name: "business", Method: "GET", Path: "/business", Access: module.AccessPublic}},
		Create: func(_ context.Context, runtime module.Runtime) (module.Instance, error) {
			runtimeContext = runtime.Context
			return module.Instance{
				HTTP:  []module.HTTPHandler{{Name: "business", Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })}},
				Close: func(context.Context) error { closes.Add(1); return nil },
			}, nil
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	handler, err := app.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if runtimeContext.Err() != nil {
		t.Fatal("startup context ended the runtime")
	}
	if _, err := app.Start(t.Context()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("second Start: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler)
	mux.HandleFunc("GET /custom", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Application", "business")
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	for _, path := range []string{"/healthz", "/business", "/custom"} {
		response, err := server.Client().Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode >= 300 || response.Header.Get("X-Application") != "business" {
			t.Fatalf("%s: %d %v", path, response.StatusCode, response.Header)
		}
	}
	server.Close()
	if err := app.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := app.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	if closes.Load() != 1 || runtimeContext.Err() == nil {
		t.Fatalf("closes=%d runtime=%v", closes.Load(), runtimeContext.Err())
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(); err != nil {
		t.Fatalf("borrowed database closed: %v", err)
	}
}

func TestStartFailureClosesCreatedModulesAndOwnedDatabase(t *testing.T) {
	skipSystemTables(t)
	db := sqliteDB(t)
	restore := OverrideOpenDatabase(func(context.Context, string, string) (*gorm.DB, error) { return db, nil })
	defer restore()
	var closes atomic.Int32
	want := errors.New("business startup failed")
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), Modules: []module.Definition{
		{Name: "system", Create: func(context.Context, module.Runtime) (module.Instance, error) {
			return module.Instance{Close: func(context.Context) error { closes.Add(1); return nil }}, nil
		}},
		{Name: "business", Requires: []string{"system"}, Create: func(context.Context, module.Runtime) (module.Instance, error) { return module.Instance{}, want }},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Start(t.Context()); !errors.Is(err, want) {
		t.Fatalf("Start: %v", err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if closes.Load() != 1 || pool.Ping() == nil {
		t.Fatalf("closes=%d, owned database still open", closes.Load())
	}
}
