//go:build integration

package corelibraryprobe

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/testdb"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func openBorrowedDB(t *testing.T) *gorm.DB {
	t.Helper()
	config, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db, err := testdb.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test-owned borrowed pool: %v", err)
		}
	})
	return db
}

func TestBorrowedDBStaysOpen(t *testing.T) {
	borrowed := openBorrowedDB(t)
	runtime := module.Runtime{DB: borrowed, Logger: testRuntime().Logger}
	closer, err := Start(context.Background(), runtime, []module.Definition{
		testDefinition("module", nil, func() (module.Instance, error) {
			return module.Instance{Close: func(context.Context) error { return nil }}, nil
		}),
	}, time.Second)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := closer(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	assertDBQueryWorks(t, borrowed)
	if runtime.DB != borrowed {
		t.Fatal("module lifecycle must retain the application DB")
	}

	createErr := errors.New("borrowed create failure")
	_, err = Start(context.Background(), runtime, []module.Definition{
		testDefinition("first", nil, func() (module.Instance, error) {
			return module.Instance{Close: func(context.Context) error { return nil }}, nil
		}),
		testDefinition("failed", []string{"first"}, func() (module.Instance, error) { return module.Instance{}, createErr }),
	}, time.Second)
	if !errors.Is(err, createErr) || runtime.DB != borrowed {
		t.Fatalf("failed start changed borrowed DB: err=%v", err)
	}
	assertDBQueryWorks(t, borrowed)
}
