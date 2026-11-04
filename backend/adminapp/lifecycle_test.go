package adminapp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestRunOpenDatabaseFailureZeroCreate(t *testing.T) {
	skipSystemTables(t)
	restore := OverrideOpenDatabase(func(context.Context, string, string) (*gorm.DB, error) {
		return nil, errors.New("open boom")
	})
	defer restore()
	var created atomic.Int32
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), Modules: []module.Definition{{
		Name:   "system",
		Create: countingCreate(&created),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(t.Context())
	if err == nil || !strings.Contains(err.Error(), "open boom") {
		t.Fatalf("err = %v", err)
	}
	if created.Load() != 0 {
		t.Fatalf("Create called %d", created.Load())
	}
}

func waitListening(t *testing.T, app *App, runErr ...<-chan error) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if app.ListenerAddr() != "" {
			return
		}
		if len(runErr) > 0 {
			select {
			case err := <-runErr[0]:
				t.Fatalf("run ended before listen: %v", err)
			default:
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("app did not listen")
}

func httpGet(url string) (*http.Response, error) {
	return http.Get(url)
}
