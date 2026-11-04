//go:build integration

package database_test

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func TestProductIdentity(t *testing.T) {
	cfg, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := testdb.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := admitted.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	db, err := database.Open(context.Background(), cfg.Driver, cfg.DSN)
	if err != nil {
		t.Fatal(err)
	}
	owned, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer owned.Close()
	for _, session := range []*gorm.DB{db, db.WithContext(context.Background()), db.Session(&gorm.Session{NewDB: true})} {
		if database.DriverOf(session) != cfg.Driver {
			t.Fatal("lost identity")
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if database.DriverOf(tx) != cfg.Driver {
			t.Fatal("transaction lost identity")
		}
		if err := tx.Transaction(func(nested *gorm.DB) error {
			if database.DriverOf(nested.WithContext(context.Background())) != cfg.Driver {
				t.Fatal("nested transaction lost identity")
			}
			return gorm.ErrInvalidTransaction
		}); err != gorm.ErrInvalidTransaction {
			t.Fatalf("nested rollback: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if db.NowFunc().Nanosecond()%1000 != 0 {
		t.Fatal("microseconds")
	}
}
