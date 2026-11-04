//go:build integration

package resource

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestStoreValidationAndBorrowedHandle(t *testing.T) {
	if _, err := NewStore(nil); err == nil {
		t.Fatal("NewStore(nil) must return an error")
	}
	gdb, err := gorm.Open(sqlite.Open("file:resource-store-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	store, err := NewStore(gdb)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if store.DB() != gdb {
		t.Fatal("Store must retain the borrowed GORM handle")
	}
	if (*Store)(nil).DB() != nil {
		t.Fatal("nil Store DB must be nil")
	}
	if (Tx{}).DB() != nil {
		t.Fatal("zero Tx DB must be nil")
	}
	if err := store.InTransaction(nil, func(Tx) error { return nil }); err == nil {
		t.Fatal("nil context must be rejected")
	}
	if err := store.InTransaction(context.Background(), nil); err == nil {
		t.Fatal("nil callback must be rejected")
	}
	if err := (*Store)(nil).InTransaction(context.Background(), func(Tx) error { return nil }); err == nil {
		t.Fatal("nil Store must be rejected")
	}
}

func TestStoreTransactionPreservesCallbackError(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file:resource-store-rollback?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	store, err := NewStore(gdb)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	callbackErr := errors.New("callback failed")
	if err := store.InTransaction(context.Background(), func(tx Tx) error {
		if tx.DB() == nil || tx.DB() == store.DB() {
			t.Fatal("transaction must expose a distinct GORM handle")
		}
		return callbackErr
	}); !errors.Is(err, callbackErr) {
		t.Fatalf("InTransaction error = %v, want %v", err, callbackErr)
	}
}
